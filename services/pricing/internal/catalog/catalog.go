// Package catalog loads versioned price lists and promotion rules from JSON
// files. Everything is read once at start-up; lookups are immutable reads.
package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"

	money "github.com/radwara/agentic-order-platform/libs/go-money"
)

// Promotion rule types.
const (
	PercentageOff = "percentage_off"
	FixedOff      = "fixed_off"

	maxBasisPoints = 10000
)

// Errors returned by this package; match with errors.Is.
var (
	// ErrInvalid reports a malformed price list or promotion rule file.
	ErrInvalid = errors.New("catalog: invalid data")
	// ErrNotFound reports an unknown price list or promotion rule version.
	ErrNotFound = errors.New("catalog: version not found")
)

// PriceList maps SKUs to unit prices (integer minor units) in a single
// currency. Prices is only read after loading.
type PriceList struct {
	Version  string
	Currency string
	Prices   map[string]int64
}

// priceListFile is the on-disk form. Prices is a list (not an object) so that
// duplicate SKUs are detectable; amounts are integers, never floats. Pointer
// fields detect missing (as opposed to zero) values.
type priceListFile struct {
	Version  string        `json:"version"`
	Currency string        `json:"currency"`
	Prices   *[]priceEntry `json:"prices"`
}

type priceEntry struct {
	SKU        string `json:"sku"`
	PriceMinor *int64 `json:"price_minor"`
}

// Rule is one per-SKU promotion. PercentageOff uses BasisPoints (1250 = 12.5%)
// of the line total; FixedOff uses AmountMinor off the line total. Only the
// field matching Type is set.
type Rule struct {
	ID          string
	SKU         string
	Type        string
	BasisPoints int64
	AmountMinor int64
}

type ruleFile struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	Type        string `json:"type"`
	BasisPoints *int64 `json:"basis_points"`
	AmountMinor *int64 `json:"amount_minor"`
}

type promoFile struct {
	Version string      `json:"version"`
	Rules   *[]ruleFile `json:"rules"`
}

// PromoSet is a versioned, ordered list of rules (file order is apply order).
type PromoSet struct {
	Version string
	Rules   []Rule
}

// Catalog holds all loaded versions.
type Catalog struct {
	priceLists map[string]*PriceList
	promos     map[string]*PromoSet
}

// Load reads every *.json file in fsys under pricelists/ and promos/.
func Load(fsys fs.FS) (*Catalog, error) {
	c := &Catalog{priceLists: map[string]*PriceList{}, promos: map[string]*PromoSet{}}
	if err := loadDir(fsys, "pricelists", func(b []byte) error {
		pl, err := parsePriceList(b)
		if err != nil {
			return err
		}
		return put(c.priceLists, pl.Version, pl)
	}); err != nil {
		return nil, err
	}
	if err := loadDir(fsys, "promos", func(b []byte) error {
		ps, err := parsePromoSet(b)
		if err != nil {
			return err
		}
		return put(c.promos, ps.Version, ps)
	}); err != nil {
		return nil, err
	}
	if len(c.priceLists) == 0 {
		return nil, fmt.Errorf("%w: no price lists found (wrong data directory?)", ErrInvalid)
	}
	return c, nil
}

// PriceList returns the price list with the given version, or an error
// wrapping ErrNotFound.
func (c *Catalog) PriceList(version string) (*PriceList, error) {
	if pl, ok := c.priceLists[version]; ok {
		return pl, nil
	}
	return nil, fmt.Errorf("%w: price list %q", ErrNotFound, version)
}

// Promos returns the promotion rule set with the given version, or an error
// wrapping ErrNotFound.
func (c *Catalog) Promos(version string) (*PromoSet, error) {
	if ps, ok := c.promos[version]; ok {
		return ps, nil
	}
	return nil, fmt.Errorf("%w: promotion rules %q", ErrNotFound, version)
}

func put[T any](m map[string]*T, version string, v *T) error {
	if _, dup := m[version]; dup {
		return fmt.Errorf("%w: duplicate version %q", ErrInvalid, version)
	}
	m[version] = v
	return nil
}

func loadDir(fsys fs.FS, dir string, each func([]byte) error) error {
	names, err := fs.Glob(fsys, path.Join(dir, "*.json"))
	if err != nil {
		return fmt.Errorf("catalog: list %s: %w", dir, err)
	}
	for _, name := range names { // Glob returns sorted names
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("catalog: read %s: %w", name, err)
		}
		if err := each(b); err != nil {
			return fmt.Errorf("catalog: %s: %w", name, err)
		}
	}
	return nil
}

func parsePriceList(b []byte) (*PriceList, error) {
	var f priceListFile
	if err := decodeStrict(b, &f); err != nil {
		return nil, err
	}
	if f.Version == "" {
		return nil, fmt.Errorf("%w: empty version", ErrInvalid)
	}
	if _, err := money.New(0, f.Currency); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if f.Prices == nil {
		return nil, fmt.Errorf("%w: missing prices", ErrInvalid)
	}
	pl := &PriceList{Version: f.Version, Currency: f.Currency, Prices: make(map[string]int64, len(*f.Prices))}
	for _, p := range *f.Prices {
		switch _, dup := pl.Prices[p.SKU]; {
		case p.SKU == "":
			return nil, fmt.Errorf("%w: empty sku", ErrInvalid)
		case dup:
			return nil, fmt.Errorf("%w: duplicate sku %q", ErrInvalid, p.SKU)
		case p.PriceMinor == nil:
			return nil, fmt.Errorf("%w: missing price_minor for %q", ErrInvalid, p.SKU)
		case *p.PriceMinor < 0:
			return nil, fmt.Errorf("%w: negative price for %q", ErrInvalid, p.SKU)
		}
		pl.Prices[p.SKU] = *p.PriceMinor
	}
	return pl, nil
}

func parsePromoSet(b []byte) (*PromoSet, error) {
	var f promoFile
	if err := decodeStrict(b, &f); err != nil {
		return nil, err
	}
	if f.Version == "" {
		return nil, fmt.Errorf("%w: empty version", ErrInvalid)
	}
	if f.Rules == nil {
		return nil, fmt.Errorf("%w: missing rules", ErrInvalid)
	}
	ps := &PromoSet{Version: f.Version}
	seen := map[string]bool{}
	for _, r := range *f.Rules {
		rule, err := r.toRule()
		if err != nil {
			return nil, err
		}
		if seen[rule.ID] {
			return nil, fmt.Errorf("%w: duplicate rule id %q", ErrInvalid, rule.ID)
		}
		seen[rule.ID] = true
		ps.Rules = append(ps.Rules, rule)
	}
	return ps, nil
}

// toRule requires exactly the amount field that matches the rule type.
func (r ruleFile) toRule() (Rule, error) {
	rule := Rule{ID: r.ID, SKU: r.SKU, Type: r.Type}
	if r.ID == "" || r.SKU == "" {
		return Rule{}, fmt.Errorf("%w: rule needs id and sku", ErrInvalid)
	}
	switch {
	case r.Type == PercentageOff && r.BasisPoints != nil && r.AmountMinor == nil &&
		*r.BasisPoints >= 0 && *r.BasisPoints <= maxBasisPoints:
		rule.BasisPoints = *r.BasisPoints
	case r.Type == FixedOff && r.AmountMinor != nil && r.BasisPoints == nil && *r.AmountMinor >= 0:
		rule.AmountMinor = *r.AmountMinor
	default:
		return Rule{}, fmt.Errorf("%w: bad rule %q (type, amount field or range)", ErrInvalid, r.ID)
	}
	return rule, nil
}

// decodeStrict rejects unknown fields and trailing data.
func decodeStrict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing data after JSON value", ErrInvalid)
	}
	return nil
}
