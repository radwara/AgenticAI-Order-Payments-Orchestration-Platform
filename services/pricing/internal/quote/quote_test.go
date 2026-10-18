package quote_test

import (
	"errors"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/radwara/agentic-order-platform/services/pricing/internal/catalog"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/quote"
)

const (
	usd   = "pl-usd-v1"
	jpy   = "pl-jpy-v1"
	promo = "promo-v1"
)

func ptr(v int64) *int64 { return &v }

func newService(t *testing.T) *quote.Service {
	t.Helper()
	c, err := catalog.Load(os.DirFS("../../testdata"))
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	return quote.NewService(c)
}

func TestQuote_AppliesPromotions(t *testing.T) {
	a := func(qty int64) []quote.Item { return []quote.Item{{SKU: "SKU-A", Quantity: qty}} }
	tests := []struct {
		name      string
		list      string
		promo     string
		items     []quote.Item
		discounts []int64
		total     int64
	}{
		// 7996 * 12.5% = 999.5 -> 1000 (even); half-down would give 999.
		{"USD percentage tie rounds half-even up", usd, promo, a(4), []int64{1000}, 6996},
		// 4020 * 12.5% = 502.5 -> 502 (even); half-up would give 503.
		{"JPY percentage tie rounds half-even down", jpy, promo, a(4), []int64{502}, 3518},
		// 1999 * 12.5% = 249.875 -> 250 (not a tie).
		{"USD non-tie rounds to nearest", usd, promo, a(1), []int64{250}, 1749},
		// SKU-B 2002-300; SKU-C 250 capped by the 100000 fixed rule so the line is 0.
		{"fixed discount is per line and capped", usd, promo,
			[]quote.Item{{SKU: "SKU-B", Quantity: 2}, {SKU: "SKU-C", Quantity: 1}}, []int64{300, 250}, 1702},
		{"no promotion version applies no discounts", usd, "", a(1), nil, 1999},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := newService(t).Quote(quote.Request{Items: tc.items, PriceListVersion: tc.list, PromoVersion: tc.promo})
			if err != nil {
				t.Fatalf("Quote: %v", err)
			}
			var got []int64
			for _, d := range res.Discounts {
				got = append(got, d.GetAmountMinor())
			}
			if !reflect.DeepEqual(got, tc.discounts) || res.GetTotalMinor() != tc.total {
				t.Fatalf("got discounts=%v total=%d, want %v / %d", got, res.GetTotalMinor(), tc.discounts, tc.total)
			}
		})
	}
}

func TestQuote_ReorderingItemsKeepsTotalAndHash(t *testing.T) {
	items := []quote.Item{
		{SKU: "SKU-C", Quantity: 3}, {SKU: "SKU-A", Quantity: 4}, {SKU: "SKU-B", Quantity: 2},
		{SKU: "SKU-A", Quantity: 1}, {SKU: "SKU-B", Quantity: 7},
	}
	svc := newService(t)
	want, err := svc.Quote(quote.Request{Items: items, PriceListVersion: usd, PromoVersion: promo})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // deterministic shuffle in a test
	for i := 0; i < 50; i++ {
		shuffled := append([]quote.Item(nil), items...)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		got, err := svc.Quote(quote.Request{Items: shuffled, PriceListVersion: usd, PromoVersion: promo})
		if err != nil {
			t.Fatalf("Quote: %v", err)
		}
		if !proto.Equal(got, want) {
			t.Fatalf("shuffle %d changed result:\n got %+v\nwant %+v", i, got, want)
		}
	}
}

func TestQuote_HashDependsOnVersionsAndItems(t *testing.T) {
	svc := newService(t)
	one := []quote.Item{{SKU: "SKU-A", Quantity: 1}}
	reqs := []quote.Request{
		{Items: one, PriceListVersion: usd},
		{Items: one, PriceListVersion: usd, PromoVersion: promo},
		{Items: one, PriceListVersion: jpy},
		{Items: []quote.Item{{SKU: "SKU-A", Quantity: 2}}, PriceListVersion: usd},
	}
	seen := map[string]bool{}
	for _, r := range reqs {
		res, err := svc.Quote(r)
		if err != nil || len(res.GetQuoteHash()) != 64 || seen[res.GetQuoteHash()] {
			t.Fatalf("hash %q (err %v) malformed or collides", res.GetQuoteHash(), err)
		}
		seen[res.GetQuoteHash()] = true
	}
}

func TestQuote_Errors(t *testing.T) {
	item := func(sku string, qty int64) []quote.Item { return []quote.Item{{SKU: sku, Quantity: qty}} }
	tests := []struct {
		name string
		req  quote.Request
		want error
	}{
		{"unknown price list", quote.Request{Items: item("SKU-A", 1), PriceListVersion: "nope"}, quote.ErrNotFound},
		{"unknown promo version", quote.Request{Items: item("SKU-A", 1), PriceListVersion: usd, PromoVersion: "nope"}, quote.ErrNotFound},
		{"unknown sku", quote.Request{Items: item("SKU-Z", 1), PriceListVersion: usd}, quote.ErrNotFound},
		{"negative quantity", quote.Request{Items: item("SKU-A", -1), PriceListVersion: usd}, quote.ErrInvalid},
		{"zero quantity", quote.Request{Items: item("SKU-A", 0), PriceListVersion: usd}, quote.ErrInvalid},
		{"no items", quote.Request{PriceListVersion: usd}, quote.ErrInvalid},
		{"price mismatch", quote.Request{Items: []quote.Item{{SKU: "SKU-A", Quantity: 1, UnitPriceMinor: ptr(5)}}, PriceListVersion: usd}, quote.ErrInvalid},
		{"asserted zero price is a mismatch, not unset", quote.Request{Items: []quote.Item{{SKU: "SKU-A", Quantity: 1, UnitPriceMinor: ptr(0)}}, PriceListVersion: usd}, quote.ErrInvalid},
		{"sku with leading space", quote.Request{Items: item(" SKU-A", 1), PriceListVersion: usd}, quote.ErrInvalid},
		{"sku with trailing space", quote.Request{Items: item("SKU-A\t", 1), PriceListVersion: usd}, quote.ErrInvalid},
		{"empty sku", quote.Request{Items: item("", 1), PriceListVersion: usd}, quote.ErrInvalid},
		{"currency mismatch", quote.Request{Items: []quote.Item{{SKU: "SKU-A", Quantity: 1, Currency: "EUR"}}, PriceListVersion: usd}, quote.ErrInvalid},
		{"overflow", quote.Request{Items: item("SKU-A", 1<<62), PriceListVersion: usd}, quote.ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := newService(t).Quote(tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
