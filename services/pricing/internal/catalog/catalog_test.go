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
		"malformed json":   {`{`, "unexpected end"},
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
		"over 100 percent": `{"version":"v","rules":[{"id":"r","sku":"A","type":"percentage_off","basis_points":10001}]}`,
		"unknown type":     `{"version":"v","rules":[{"id":"r","sku":"A","type":"bogus"}]}`,
		"missing id":       `{"version":"v","rules":[{"sku":"A","type":"fixed_off"}]}`,
		"negative fixed":   `{"version":"v","rules":[{"id":"r","sku":"A","type":"fixed_off","amount_minor":-1}]}`,
		"empty version":    `{"rules":[]}`,
		"malformed json":   `{`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := catalog.Load(fstest.MapFS{"promos/a.json": {Data: []byte(body)}})
			if !errors.Is(err, catalog.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
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
