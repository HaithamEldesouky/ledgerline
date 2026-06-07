package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Money is represented throughout the system as an integer number of minor
// units (e.g. cents), never as a floating-point value. IEEE-754 doubles cannot
// represent most decimal fractions exactly, so using them for money causes
// rounding drift and reconciliation failures. The value 100 with currency
// "USD" therefore means $1.00.

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// NormalizeCurrency trims, upper-cases and validates an ISO 4217 currency code.
func NormalizeCurrency(currency string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(currency))
	if !currencyPattern.MatchString(normalized) {
		return "", ErrValidation(
			fmt.Sprintf("invalid currency code %q: expected a 3-letter ISO 4217 code", currency),
		)
	}
	return normalized, nil
}

// FormatMinor renders an amount in minor units as a human-readable decimal
// string. It is a pure presentation helper; the canonical value always stays
// an integer.
//
//	FormatMinor(12345, "USD") => "123.45 USD"
func FormatMinor(amountMinor int64, currency string) string {
	sign := ""
	if amountMinor < 0 {
		sign = "-"
	}
	abs := amountMinor
	if abs < 0 {
		abs = -abs
	}
	major := abs / 100
	minor := abs % 100
	return fmt.Sprintf("%s%d.%02d %s", sign, major, minor, currency)
}
