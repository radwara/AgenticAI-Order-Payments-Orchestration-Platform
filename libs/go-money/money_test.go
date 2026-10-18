package money

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"
	"testing/quick"
)

func usd(a int64) Money { return Money{Amount: a, Currency: "USD"} }

func TestExponentTable(t *testing.T) {
	want := map[string]int{"USD": 2, "EUR": 2, "GBP": 2, "JPY": 0, "KWD": 3}
	for c, w := range want {
		got, ok := Exponent(c)
		if !ok || got != w {
			t.Errorf("Exponent(%s) = %d,%v want %d,true", c, got, ok, w)
		}
	}
	if _, ok := Exponent("XXX"); ok {
		t.Error("XXX must be unknown")
	}
}

func TestNew(t *testing.T) {
	if m, err := New(5, "JPY"); err != nil || m != (Money{5, "JPY"}) {
		t.Errorf("New valid: %v %v", m, err)
	}
	if _, err := New(5, "ZZZ"); !errors.Is(err, ErrUnknownCurrency) {
		t.Errorf("want ErrUnknownCurrency, got %v", err)
	}
}

// AC1: Add/Sub across different currencies returns an error.
func TestCurrencyMismatch(t *testing.T) {
	eur := Money{1, "EUR"}
	if _, err := usd(1).Add(eur); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Add: %v", err)
	}
	if _, err := usd(1).Sub(eur); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Sub: %v", err)
	}
	if _, err := usd(1).Cmp(eur); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Cmp: %v", err)
	}
}

func TestAddSub(t *testing.T) {
	if got, err := usd(100).Add(usd(-250)); err != nil || got != usd(-150) {
		t.Errorf("Add: %v %v", got, err)
	}
	if got, err := usd(100).Sub(usd(-250)); err != nil || got != usd(350) {
		t.Errorf("Sub: %v %v", got, err)
	}
}

// AC4: overflow returns an error, never wraps.
func TestOverflow(t *testing.T) {
	tests := []struct {
		name string
		op   func() (Money, error)
	}{
		{"add max+1", func() (Money, error) { return usd(math.MaxInt64).Add(usd(1)) }},
		{"add min-1", func() (Money, error) { return usd(math.MinInt64).Add(usd(-1)) }},
		{"sub min-1", func() (Money, error) { return usd(math.MinInt64).Sub(usd(1)) }},
		{"sub max-(-1)", func() (Money, error) { return usd(math.MaxInt64).Sub(usd(-1)) }},
		{"mulratio", func() (Money, error) { return usd(math.MaxInt64).MulRatio(2, 1) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.op(); !errors.Is(err, ErrOverflow) {
				t.Errorf("want ErrOverflow, got %v", err)
			}
		})
	}
	// Boundaries that do fit must succeed.
	if got, err := usd(math.MaxInt64).Add(usd(0)); err != nil || got.Amount != math.MaxInt64 {
		t.Errorf("max+0: %v %v", got, err)
	}
	if got, err := usd(math.MaxInt64 - 1).Add(usd(1)); err != nil || got.Amount != math.MaxInt64 {
		t.Errorf("max-1+1: %v %v", got, err)
	}
	if got, err := usd(math.MinInt64).Sub(usd(0)); err != nil || got.Amount != math.MinInt64 {
		t.Errorf("min-0: %v %v", got, err)
	}
	if got, err := usd(math.MaxInt64).MulRatio(2, 2); err != nil || got.Amount != math.MaxInt64 {
		t.Errorf("max*2/2: %v %v", got, err)
	}
}

// AC2: MulRatio rounds half-even, including .5 boundaries for JPY and KWD.
func TestMulRatioHalfEven(t *testing.T) {
	tests := []struct {
		name string

		cur string

		amt, n, d, want int64
	}{
		{"JPY 2.5 -> 2", "JPY", 5, 1, 2, 2},
		{"JPY 3.5 -> 4", "JPY", 7, 1, 2, 4},
		{"JPY 0.5 -> 0", "JPY", 1, 1, 2, 0},
		{"JPY 1.5 -> 2", "JPY", 3, 1, 2, 2},
		{"JPY -2.5 -> -2", "JPY", -5, 1, 2, -2},
		{"JPY -3.5 -> -4", "JPY", -7, 1, 2, -4},
		{"JPY 100 * 15/100 exact", "JPY", 100, 15, 100, 15},
		{"KWD 2.5 -> 2", "KWD", 25, 1, 10, 2},
		{"KWD 3.5 -> 4", "KWD", 35, 1, 10, 4},
		{"KWD 0.5 -> 0", "KWD", 1, 1, 2, 0},
		{"KWD 1.5 -> 2", "KWD", 3, 1, 2, 2},
		{"KWD -0.5 -> 0", "KWD", -1, 1, 2, 0},
		{"KWD 1.234 -> 1", "KWD", 1234, 1, 1000, 1},
		{"USD below half rounds down", "USD", 10, 1, 3, 3},
		{"USD above half rounds up", "USD", 10, 2, 3, 7},
		{"USD negative denominator", "USD", 5, 1, -2, -2},
		{"USD negative den odd", "USD", 7, 1, -2, -4},
		{"USD both negative", "USD", -7, -1, 2, 4},
		{"USD zero amount", "USD", 0, 7, 3, 0},
		{"USD zero num", "USD", 100, 0, 3, 0},
		{"USD large intermediate", "USD", math.MaxInt64, 3, 3, math.MaxInt64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Money{tc.amt, tc.cur}.MulRatio(tc.n, tc.d)
			if err != nil {
				t.Fatal(err)
			}
			if got.Amount != tc.want || got.Currency != tc.cur {
				t.Errorf("got %v want %d %s", got, tc.want, tc.cur)
			}
		})
	}
}

func TestMulRatioZeroDenominator(t *testing.T) {
	if _, err := usd(1).MulRatio(1, 0); !errors.Is(err, ErrDivideByZero) {
		t.Errorf("got %v", err)
	}
}

// AC3: Allocate(100,[1,1,1]) = [34,33,33] and the sum is always preserved.
func TestAllocateExample(t *testing.T) {
	got, err := usd(100).Allocate([]int64{1, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []Money{usd(34), usd(33), usd(33)}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestAllocateCases(t *testing.T) {
	tests := []struct {
		name string

		in Money

		ratios []int64

		want []int64
	}{
		{"negative amount", usd(-100), []int64{1, 1, 1}, []int64{-34, -33, -33}},
		{"zero ratio gets nothing", usd(5), []int64{0, 1, 1}, []int64{0, 3, 2}},
		{"single unit", usd(1), []int64{1, 1}, []int64{1, 0}},
		{"weighted", usd(100), []int64{70, 20, 10}, []int64{70, 20, 10}},
		{"zero amount", usd(0), []int64{1, 2}, []int64{0, 0}},
		{"min int64 single part", usd(math.MinInt64), []int64{3}, []int64{math.MinInt64}},
		{"huge ratios no overflow", usd(math.MaxInt64), []int64{math.MaxInt64, math.MaxInt64}, []int64{math.MaxInt64/2 + 1, math.MaxInt64 / 2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.in.Allocate(tc.ratios)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len %d want %d", len(got), len(tc.want))
			}
			for i, w := range tc.want {
				if got[i].Amount != w || got[i].Currency != tc.in.Currency {
					t.Errorf("part %d = %v want %d", i, got[i], w)
				}
			}
		})
	}
}

func TestAllocateErrors(t *testing.T) {
	for name, r := range map[string][]int64{
		"empty":    nil,
		"negative": {1, -1},
		"zero sum": {0, 0},
	} {
		if _, err := usd(10).Allocate(r); !errors.Is(err, ErrInvalidRatios) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestAllocatePreservesSumProperty(t *testing.T) {
	f := func(amount int64, rs [5]uint16) bool {
		ratios := make([]int64, len(rs))
		for i, r := range rs {
			ratios[i] = int64(r)
		}
		ratios[int(uint64(amount)%uint64(len(rs)))]++ // guarantee a positive sum
		parts, err := usd(amount).Allocate(ratios)
		if err != nil || len(parts) != len(ratios) {
			return false
		}
		sum := new(big.Int)
		for i, p := range parts {
			if ratios[i] == 0 && p.Amount != 0 {
				return false
			}
			if amount >= 0 && p.Amount < 0 {
				return false
			}
			if amount < 0 && p.Amount > 0 {
				return false
			}
			sum.Add(sum, big.NewInt(p.Amount))
		}
		return sum.Cmp(big.NewInt(amount)) == 0
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Error(err)
	}
}

func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		a, b Money

		want int
	}{{usd(1), usd(2), -1}, {usd(2), usd(1), 1}, {usd(2), usd(2), 0}} {
		if got, err := tc.a.Cmp(tc.b); err != nil || got != tc.want {
			t.Errorf("Cmp(%v,%v) = %d,%v", tc.a, tc.b, got, err)
		}
	}
	if !usd(1).Equal(usd(1)) || usd(1).Equal(usd(2)) || usd(1).Equal(Money{1, "EUR"}) {
		t.Error("Equal wrong")
	}
	if !usd(0).IsZero() || usd(1).IsZero() {
		t.Error("IsZero wrong")
	}
}

func TestJSON(t *testing.T) {
	b, err := json.Marshal(Money{12345, "KWD"})
	if err != nil || string(b) != `{"amount":12345,"currency":"KWD"}` {
		t.Fatalf("marshal: %s %v", b, err)
	}
	var back Money
	if err := json.Unmarshal(b, &back); err != nil || back != (Money{12345, "KWD"}) {
		t.Fatalf("unmarshal: %v %v", back, err)
	}
	// Nested use through a struct field.
	type order struct {
		Total Money `json:"total"`
	}
	var o order
	if err := json.Unmarshal([]byte(`{"total":{"amount":-7,"currency":"JPY"}}`), &o); err != nil || o.Total != (Money{-7, "JPY"}) {
		t.Fatalf("nested: %v %v", o, err)
	}
	if _, err := json.Marshal(Money{1, "ZZZ"}); !errors.Is(err, ErrUnknownCurrency) {
		t.Errorf("marshal unknown: %v", err)
	}
	for _, bad := range []string{
		`{"amount":1,"currency":"ZZZ"}`,
		`{"amount":1.5,"currency":"USD"}`,
		`{"amount":"x","currency":"USD"}`,
		`not json`,
	} {
		var m Money
		if err := json.Unmarshal([]byte(bad), &m); err == nil {
			t.Errorf("expected error for %s", bad)
		}
	}
}

// Guards the "no floating point anywhere in the package" constraint: parses
// every non-test source file and rejects float types and float literals.
func TestNoFloatingPoint(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if x.Name == "float32" || x.Name == "float64" {
					t.Errorf("%s: float type %s", fset.Position(x.Pos()), x.Name)
				}
			case *ast.BasicLit:
				if x.Kind == token.FLOAT {
					t.Errorf("%s: float literal %s", fset.Position(x.Pos()), x.Value)
				}
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no source files checked")
	}
}
