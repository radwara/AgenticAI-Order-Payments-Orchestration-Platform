// Package quote computes deterministic price quotes. The calculation path uses
// no clock, randomness or map iteration: items are sorted canonically, rules
// apply in file order, and all arithmetic is integer minor units (go-money).
package quote

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	money "github.com/radwara/agentic-order-platform/libs/go-money"
	pb "github.com/radwara/agentic-order-platform/services/pricing/gen/pricingv1"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/catalog"
)

// Errors returned by Quote; callers match with errors.Is.
var (
	// ErrInvalid marks a request that can never succeed as sent.
	ErrInvalid = errors.New("quote: invalid request")
	// ErrNotFound marks an unknown price list version, promotion version or SKU.
	ErrNotFound = errors.New("quote: not found")
)

const (
	hashDomain = "pricing-quote-v1" // bump if the canonical form changes
	basisBase  = 10000
)

// Item is one requested line. UnitPriceMinor (nil = unset; 0 is a real
// assertion) and Currency ("" = unset) are assertions that must match the
// price list. SKU is matched exactly; surrounding whitespace is rejected.
type Item struct {
	SKU            string
	Quantity       int64
	UnitPriceMinor *int64
	Currency       string
}

// Request is a quote request. PromoVersion "" applies no promotions.
type Request struct {
	Items            []Item
	PriceListVersion string
	PromoVersion     string
}

// Service prices requests against an immutable catalog. Safe for concurrent use.
type Service struct{ cat *catalog.Catalog }

// NewService returns a Service backed by cat.
func NewService(cat *catalog.Catalog) *Service { return &Service{cat: cat} }

// Quote prices req. It is pure: identical request and catalog give an
// identical response. Errors wrap ErrInvalid or ErrNotFound.
func (s *Service) Quote(req Request) (*pb.QuoteResponse, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("%w: no items", ErrInvalid)
	}
	pl, err := s.cat.PriceList(req.PriceListVersion)
	if err != nil {
		return nil, lookupErr(err)
	}
	var promos []catalog.Rule
	if req.PromoVersion != "" {
		ps, err := s.cat.Promos(req.PromoVersion)
		if err != nil {
			return nil, lookupErr(err)
		}
		promos = ps.Rules
	}

	items := canonical(req.Items)
	total, err := money.New(0, pl.Currency)
	if err != nil {
		return nil, fmt.Errorf("quote: price list currency: %w", err)
	}
	res := &pb.QuoteResponse{
		Currency: pl.Currency, PriceListVersion: req.PriceListVersion, PromotionRuleVersion: req.PromoVersion,
	}
	for _, it := range items {
		line, err := priceLine(pl, it)
		if err != nil {
			return nil, err
		}
		discounts, err := applyRules(promos, line, pl.Currency)
		if err != nil {
			return nil, err
		}
		res.Lines = append(res.Lines, line)
		res.Discounts = append(res.Discounts, discounts...)
		net := line.LineTotalMinor
		for _, d := range discounts {
			net -= d.AmountMinor // each discount is capped at what remains, so net >= 0
		}
		if total, err = total.Add(money.Money{Amount: net, Currency: pl.Currency}); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
		}
	}
	res.TotalMinor = total.Amount
	res.QuoteHash = hash(req, items)
	return res, nil
}

// lookupErr maps a catalog miss to ErrNotFound; anything else stays internal.
func lookupErr(err error) error {
	if errors.Is(err, catalog.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return fmt.Errorf("quote: catalog lookup: %w", err)
}

func priceLine(pl *catalog.PriceList, it Item) (*pb.LineTotal, error) {
	if it.SKU == "" || strings.TrimSpace(it.SKU) != it.SKU {
		return nil, fmt.Errorf("%w: sku %q is empty or has surrounding whitespace", ErrInvalid, it.SKU)
	}
	if it.Quantity <= 0 {
		return nil, fmt.Errorf("%w: quantity %d for %q must be positive", ErrInvalid, it.Quantity, it.SKU)
	}
	price, ok := pl.Prices[it.SKU]
	if !ok {
		return nil, fmt.Errorf("%w: sku %q in %q", ErrNotFound, it.SKU, pl.Version)
	}
	if it.UnitPriceMinor != nil && *it.UnitPriceMinor != price {
		return nil, fmt.Errorf("%w: unit price for %q is %d, list says %d", ErrInvalid, it.SKU, *it.UnitPriceMinor, price)
	}
	if it.Currency != "" && it.Currency != pl.Currency {
		return nil, fmt.Errorf("%w: currency %q for %q, list is %s", ErrInvalid, it.Currency, it.SKU, pl.Currency)
	}
	lineTotal, err := money.Money{Amount: price, Currency: pl.Currency}.MulRatio(it.Quantity, 1)
	if err != nil {
		return nil, fmt.Errorf("%w: line total for %q: %w", ErrInvalid, it.SKU, err)
	}
	return &pb.LineTotal{Sku: it.SKU, Quantity: it.Quantity, UnitPriceMinor: price, LineTotalMinor: lineTotal.Amount}, nil
}

// applyRules returns the discounts for one line, in rule order, with the
// running discount capped so a line never goes below zero.
func applyRules(rules []catalog.Rule, line *pb.LineTotal, currency string) ([]*pb.DiscountLine, error) {
	var out []*pb.DiscountLine
	remaining := line.LineTotalMinor
	lineTotal := money.Money{Amount: line.LineTotalMinor, Currency: currency}
	for _, r := range rules {
		if r.SKU != line.Sku {
			continue
		}
		amount := r.AmountMinor
		if r.Type == catalog.PercentageOff {
			d, err := lineTotal.MulRatio(r.BasisPoints, basisBase) // half-even
			if err != nil {
				return nil, fmt.Errorf("%w: rule %q: %w", ErrInvalid, r.ID, err)
			}
			amount = d.Amount
		}
		amount = min(amount, remaining)
		if amount <= 0 {
			continue
		}
		remaining -= amount
		out = append(out, &pb.DiscountLine{RuleId: r.ID, Sku: line.Sku, AmountMinor: amount})
	}
	return out, nil
}

// canonical returns a copy of items sorted by (sku, quantity, price, currency);
// an unset price sorts before any asserted price.
func canonical(items []Item) []Item {
	out := slices.Clone(items)
	slices.SortFunc(out, func(a, b Item) int {
		return cmp.Or(
			cmp.Compare(a.SKU, b.SKU),
			cmp.Compare(a.Quantity, b.Quantity),
			cmp.Compare(priceKey(a.UnitPriceMinor), priceKey(b.UnitPriceMinor)),
			cmp.Compare(a.Currency, b.Currency),
		)
	})
	return out
}

// priceKey renders an optional price as a sortable/hashable token: unset is "-"
// (cannot collide with a number), numbers are zero-padded to compare correctly.
func priceKey(p *int64) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%020d", *p)
}

// hash is SHA-256 (hex) over the canonical request and versions. Strings are
// Go-quoted so field boundaries are unambiguous.
func hash(req Request, sorted []Item) string {
	var b strings.Builder
	b.WriteString(hashDomain + "\n")
	b.WriteString(strconv.Quote(req.PriceListVersion) + "\n")
	b.WriteString(strconv.Quote(req.PromoVersion) + "\n")
	for _, it := range sorted {
		fmt.Fprintf(&b, "%s %d %s %s\n", strconv.Quote(it.SKU), it.Quantity, priceKey(it.UnitPriceMinor), strconv.Quote(it.Currency))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
