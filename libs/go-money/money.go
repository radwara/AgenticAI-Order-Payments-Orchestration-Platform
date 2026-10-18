// Package money provides an integer minor-unit money type with ISO-4217
// currency awareness. It never uses floating point.
package money

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
)

// Errors returned by this package.
var (
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
	ErrUnknownCurrency  = errors.New("money: unknown currency")
	ErrOverflow         = errors.New("money: arithmetic overflow")
	ErrDivideByZero     = errors.New("money: zero denominator")
	ErrInvalidRatios    = errors.New("money: invalid allocation ratios")
)

// Money is an amount in minor units (cents, yen, fils) of an ISO-4217 currency.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// New returns a Money, validating that the currency is supported.
func New(amount int64, currency string) (Money, error) {
	if _, ok := Exponent(currency); !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrUnknownCurrency, currency)
	}
	return Money{Amount: amount, Currency: currency}, nil
}

func (m Money) sameCurrency(o Money) error {
	if m.Currency != o.Currency {
		return fmt.Errorf("%w: %q vs %q", ErrCurrencyMismatch, m.Currency, o.Currency)
	}
	return nil
}

// Add returns m+o. It errors on currency mismatch or int64 overflow.
func (m Money) Add(o Money) (Money, error) {
	if err := m.sameCurrency(o); err != nil {
		return Money{}, err
	}
	if (o.Amount > 0 && m.Amount > math.MaxInt64-o.Amount) ||
		(o.Amount < 0 && m.Amount < math.MinInt64-o.Amount) {
		return Money{}, ErrOverflow
	}
	return Money{Amount: m.Amount + o.Amount, Currency: m.Currency}, nil
}

// Sub returns m-o. It errors on currency mismatch or int64 overflow.
func (m Money) Sub(o Money) (Money, error) {
	if err := m.sameCurrency(o); err != nil {
		return Money{}, err
	}
	if (o.Amount < 0 && m.Amount > math.MaxInt64+o.Amount) ||
		(o.Amount > 0 && m.Amount < math.MinInt64+o.Amount) {
		return Money{}, ErrOverflow
	}
	return Money{Amount: m.Amount - o.Amount, Currency: m.Currency}, nil
}

// MulRatio returns m * num / den rounded half-even (banker's rounding) to a
// whole minor unit. It errors if den is zero or the result overflows int64.
func (m Money) MulRatio(num, den int64) (Money, error) {
	if den == 0 {
		return Money{}, ErrDivideByZero
	}
	p := new(big.Int).Mul(big.NewInt(m.Amount), big.NewInt(num))
	d := big.NewInt(den)
	neg := p.Sign()*d.Sign() < 0
	p.Abs(p)
	d.Abs(d)
	q, r := new(big.Int).QuoRem(p, d, new(big.Int))
	r.Lsh(r, 1) // compare 2*remainder with the denominator
	if c := r.Cmp(d); c > 0 || (c == 0 && q.Bit(0) == 1) {
		q.Add(q, big.NewInt(1))
	}
	if neg {
		q.Neg(q)
	}
	if !q.IsInt64() {
		return Money{}, ErrOverflow
	}
	return Money{Amount: q.Int64(), Currency: m.Currency}, nil
}

// Allocate splits m across len(ratios) parts proportionally to ratios without
// losing any minor unit: the parts always sum to m. Leftover units go one each
// to the first parts with a non-zero ratio. Ratios must be non-negative with
// a positive sum.
func (m Money) Allocate(ratios []int64) ([]Money, error) {
	if len(ratios) == 0 {
		return nil, fmt.Errorf("%w: no ratios", ErrInvalidRatios)
	}
	total := new(big.Int)
	for _, r := range ratios {
		if r < 0 {
			return nil, fmt.Errorf("%w: negative ratio", ErrInvalidRatios)
		}
		total.Add(total, big.NewInt(r))
	}
	if total.Sign() == 0 {
		return nil, fmt.Errorf("%w: ratios sum to zero", ErrInvalidRatios)
	}

	abs := new(big.Int).Abs(big.NewInt(m.Amount))
	shares := make([]*big.Int, len(ratios))
	allocated := new(big.Int)
	for i, r := range ratios {
		s := new(big.Int).Mul(abs, big.NewInt(r))
		s.Quo(s, total)
		shares[i] = s
		allocated.Add(allocated, s)
	}
	rem := new(big.Int).Sub(abs, allocated).Int64() // fewer than len(ratios)
	one := big.NewInt(1)
	for i := 0; rem > 0; i++ {
		if ratios[i] > 0 {
			shares[i].Add(shares[i], one)
			rem--
		}
	}

	out := make([]Money, len(ratios))
	for i, s := range shares {
		if m.Amount < 0 {
			s.Neg(s)
		}
		out[i] = Money{Amount: s.Int64(), Currency: m.Currency}
	}
	return out, nil
}

// Cmp compares m and o, returning -1, 0 or +1. It errors on currency mismatch.
func (m Money) Cmp(o Money) (int, error) {
	if err := m.sameCurrency(o); err != nil {
		return 0, err
	}
	switch {
	case m.Amount < o.Amount:
		return -1, nil
	case m.Amount > o.Amount:
		return 1, nil
	}
	return 0, nil
}

// Equal reports whether both amount and currency are equal.
func (m Money) Equal(o Money) bool { return m == o }

// IsZero reports whether the amount is zero.
func (m Money) IsZero() bool { return m.Amount == 0 }

type wire Money // method-free copy, avoids recursion in the JSON methods

// MarshalJSON encodes as {"amount":<minor units>,"currency":"<ISO code>"}.
func (m Money) MarshalJSON() ([]byte, error) {
	if _, ok := Exponent(m.Currency); !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownCurrency, m.Currency)
	}
	return json.Marshal(wire(m))
}

// UnmarshalJSON decodes and validates the currency.
func (m *Money) UnmarshalJSON(b []byte) error {
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	v, err := New(w.Amount, w.Currency)
	if err != nil {
		return err
	}
	*m = v
	return nil
}
