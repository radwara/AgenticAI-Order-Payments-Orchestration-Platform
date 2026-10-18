package catalog_test

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/radwara/agentic-order-platform/services/pricing/internal/catalog"
)

const validList = `{"version":"pl","currency":"USD","prices":[{"sku":"A","price_minor":1}]}`

// withList returns a filesystem with a valid price list plus extra files, so
// a rejection can only come from the extra files.
func withList(extra fstest.MapFS) fstest.MapFS {
	fsys := fstest.MapFS{"pricelists/ok.json": {Data: []byte(validList)}}
	for k, v := range extra {
		fsys[k] = v
	}
	return fsys
}

func TestLoad_TestdataVersionsAreAvailable(t *testing.T) {
	c, err := catalog.Load(os.DirFS("../../testdata"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	pl, err := c.PriceList("pl-usd-v1")
	if err != nil || pl.Currency != "USD" || pl.Prices["SKU-A"] != 1999 || len(pl.Prices) != 3 {
		t.Errorf("pl-usd-v1 = %+v, %v", pl, err)
	}
	ps, err := c.Promos("promo-v1")
	if err != nil || len(ps.Rules) != 3 || ps.Rules[0].Type != catalog.PercentageOff ||
		ps.Rules[0].BasisPoints != 1250 || ps.Rules[1].Type != catalog.FixedOff || ps.Rules[1].AmountMinor != 300 {
		t.Errorf("promo-v1 = %+v, %v", ps, err)
	}
}

func TestLookup_UnknownVersionReturnsTypedNotFound(t *testing.T) {
	c, err := catalog.Load(os.DirFS("../../testdata"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := c.PriceList("missing"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("PriceList err = %v, want ErrNotFound", err)
	}
	if _, err := c.Promos("missing"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("Promos err = %v, want ErrNotFound", err)
	}
}

func TestLoad_RejectsBadPriceLists(t *testing.T) {
	const head = `{"version":"v","currency":"USD",`
	tests := map[string]struct{ body, want string }{
		"malformed json":   {`{`, "unexpected EOF"},
		"missing price":    {head + `"prices":[{"sku":"A"}]}`, "missing price_minor"},
		"misspelled price": {head + `"prices":[{"sku":"A","pricee_minor":500}]}`, "unknown field"},
		"unknown top key":  {head + `"prices":[],"extra":1}`, "unknown field"},
		"missing prices":   {`{"version":"v","currency":"USD"}`, "missing prices"},
		"null prices":      {head + `"prices":null}`, "missing prices"},
		"trailing data":    {head + `"prices":[]} {}`, "trailing data"},
		"duplicate sku":    {head + `"prices":[{"sku":"A","price_minor":1},{"sku":"A","price_minor":2}]}`, "duplicate sku"},
		"negative price":   {head + `"prices":[{"sku":"A","price_minor":-1}]}`, "negative price"},
		"unknown currency": {`{"version":"v","currency":"XXX","prices":[]}`, "unknown currency"},
		"float amount":     {head + `"prices":[{"sku":"A","price_minor":1.5}]}`, "cannot unmarshal"},
		"empty sku":        {head + `"prices":[{"sku":"","price_minor":1}]}`, "empty sku"},
		"empty version":    {`{"currency":"USD","prices":[]}`, "empty version"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := catalog.Load(fstest.MapFS{"pricelists/a.json": {Data: []byte(tc.body)}})
			if !errors.Is(err, catalog.ErrInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want ErrInvalid containing %q", err, tc.want)
			}
		})
	}
}

func TestLoad_RejectsBadPromos(t *testing.T) {
	tests := map[string]string{
		"over 100 percent":     `{"version":"v","rules":[{"id":"r","sku":"A","type":"percentage_off","basis_points":10001}]}`,
		"unknown type":         `{"version":"v","rules":[{"id":"r","sku":"A","type":"bogus"}]}`,
		"missing id":           `{"version":"v","rules":[{"sku":"A","type":"fixed_off","amount_minor":1}]}`,
		"negative fixed":       `{"version":"v","rules":[{"id":"r","sku":"A","type":"fixed_off","amount_minor":-1}]}`,
		"empty version":        `{"rules":[]}`,
		"malformed json":       `{`,
		"missing basis_points": `{"version":"v","rules":[{"id":"r","sku":"A","type":"percentage_off"}]}`,
		"misspelled basis":     `{"version":"v","rules":[{"id":"r","sku":"A","type":"percentage_off","basis_pointz":5}]}`,
		"missing amount_minor": `{"version":"v","rules":[{"id":"r","sku":"A","type":"fixed_off"}]}`,
		"misspelled amount":    `{"version":"v","rules":[{"id":"r","sku":"A","type":"fixed_off","amount_minr":5}]}`,
		"wrong amount field":   `{"version":"v","rules":[{"id":"r","sku":"A","type":"fixed_off","basis_points":5}]}`,
		"both amount fields":   `{"version":"v","rules":[{"id":"r","sku":"A","type":"fixed_off","amount_minor":1,"basis_points":5}]}`,
		"missing rules":        `{"version":"v"}`,
		"null rules":           `{"version":"v","rules":null}`,
		"trailing data":        `{"version":"v","rules":[]} x`,
		"duplicate rule ids":   `{"version":"v","rules":[{"id":"r","sku":"A","type":"fixed_off","amount_minor":1},{"id":"r","sku":"B","type":"fixed_off","amount_minor":1}]}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := catalog.Load(withList(fstest.MapFS{"promos/a.json": {Data: []byte(body)}}))
			if !errors.Is(err, catalog.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestLoad_ZeroAmountsAreAcceptedWhenExplicit(t *testing.T) {
	promo := `{"version":"v","rules":[{"id":"r","sku":"A","type":"fixed_off","amount_minor":0}]}`
	zero := `{"version":"z","currency":"USD","prices":[{"sku":"A","price_minor":0}]}`
	c, err := catalog.Load(fstest.MapFS{"pricelists/z.json": {Data: []byte(zero)}, "promos/p.json": {Data: []byte(promo)}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if pl, _ := c.PriceList("z"); pl.Prices["A"] != 0 {
		t.Errorf("prices = %v", pl.Prices)
	}
}

func TestLoad_EmptyDataDirectoryIsAnError(t *testing.T) {
	if _, err := catalog.Load(fstest.MapFS{}); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestLoad_RejectsDuplicateVersions(t *testing.T) {
	body := []byte(`{"version":"v","currency":"USD","prices":[]}`)
	fsys := fstest.MapFS{"pricelists/a.json": {Data: body}, "pricelists/b.json": {Data: body}}
	if _, err := catalog.Load(fsys); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestLoad_SameFilesTwiceYieldIdenticalContent(t *testing.T) {
	a, err1 := catalog.Load(os.DirFS("../../testdata"))
	b, err2 := catalog.Load(os.DirFS("../../testdata"))
	if err1 != nil || err2 != nil {
		t.Fatalf("Load: %v, %v", err1, err2)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two loads of the same files differ")
	}
}

func TestLoad_ContentIsIndependentOfPriceOrderAndFileOrder(t *testing.T) {
	asc := `{"version":"v","currency":"USD","prices":[{"sku":"A","price_minor":1},{"sku":"B","price_minor":2}]}`
	desc := `{"version":"v","currency":"USD","prices":[{"sku":"B","price_minor":2},{"sku":"A","price_minor":1}]}`
	other := `{"version":"w","currency":"JPY","prices":[]}`
	c1, err1 := catalog.Load(fstest.MapFS{"pricelists/1.json": {Data: []byte(asc)}, "pricelists/2.json": {Data: []byte(other)}})
	c2, err2 := catalog.Load(fstest.MapFS{"pricelists/1.json": {Data: []byte(other)}, "pricelists/2.json": {Data: []byte(desc)}})
	if err1 != nil || err2 != nil {
		t.Fatalf("Load: %v, %v", err1, err2)
	}
	if !reflect.DeepEqual(c1, c2) {
		t.Fatal("content depends on price or file order")
	}
}
