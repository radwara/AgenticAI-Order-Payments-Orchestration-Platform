// Package catalog loads versioned price lists and promotion rules from JSON
// files. Everything is read once at start-up; lookups are immutable reads.
package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
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
// duplicate SKUs are detectable; amounts are integers, never floats.
type priceListFile struct {
	Version  string `json:"version"`
	Currency string `json:"currency"`
	Prices   []struct {
		SKU        string `json:"sku"`
		PriceMinor int64  `json:"price_minor"`
	} `json:"prices"`
}

// Rule is one per-SKU promotion. PercentageOff uses BasisPoints (1250 = 12.5%)
// of the line total; FixedOff uses AmountMinor off the line total.
type Rule struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	Type        string `json:"type"`
	BasisPoints int64  `json:"basis_points"`
	AmountMinor int64  `json:"amount_minor"`
}

// PromoSet is a versioned, ordered list of rules (file order is apply order).
type PromoSet struct {
	Version string `json:"version"`
	Rules   []Rule `json:"rules"`
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
		var ps PromoSet
		if err := json.Unmarshal(b, &ps); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalid, err)
		}
		if err := ps.validate(); err != nil {
			return err
		}
		return put(c.promos, ps.Version, &ps)
	}); err != nil {
		return nil, err
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
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if f.Version == "" {
		return nil, fmt.Errorf("%w: empty version", ErrInvalid)
	}
	if _, err := money.New(0, f.Currency); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	pl := &PriceList{Version: f.Version, Currency: f.Currency, Prices: make(map[string]int64, len(f.Prices))}
	for _, p := range f.Prices {
		switch _, dup := pl.Prices[p.SKU]; {
		case p.SKU == "":
			return nil, fmt.Errorf("%w: empty sku", ErrInvalid)
		case dup:
			return nil, fmt.Errorf("%w: duplicate sku %q", ErrInvalid, p.SKU)
		case p.PriceMinor < 0:
			return nil, fmt.Errorf("%w: negative price for %q", ErrInvalid, p.SKU)
		}
		pl.Prices[p.SKU] = p.PriceMinor
	}
	return pl, nil
}

func (ps *PromoSet) validate() error {
	if ps.Version == "" {
		return fmt.Errorf("%w: empty version", ErrInvalid)
	}
	for _, r := range ps.Rules {
		switch {
		case r.ID == "" || r.SKU == "":
			return fmt.Errorf("%w: rule needs id and sku", ErrInvalid)
		case r.Type == PercentageOff && r.BasisPoints >= 0 && r.BasisPoints <= maxBasisPoints:
		case r.Type == FixedOff && r.AmountMinor >= 0:
		default:
			return fmt.Errorf("%w: bad rule %q", ErrInvalid, r.ID)
		}
	}
	return nil
}
