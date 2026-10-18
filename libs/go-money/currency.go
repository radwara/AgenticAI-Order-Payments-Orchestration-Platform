package money

// exponents maps an ISO-4217 currency code to its number of minor-unit
// decimal places (for example USD 2, JPY 0, KWD 3).
var exponents = map[string]int{
	"USD": 2,
	"EUR": 2,
	"GBP": 2,
	"JPY": 0,
	"KWD": 3,
}

// Exponent returns the number of minor-unit decimals for currency and whether
// the currency is supported.
func Exponent(currency string) (int, bool) {
	e, ok := exponents[currency]
	return e, ok
}
