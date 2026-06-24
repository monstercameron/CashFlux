// SPDX-License-Identifier: MIT

package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidAmount is returned when a string cannot be parsed as a money amount.
var ErrInvalidAmount = errors.New("money: invalid amount")

// FormatMinor renders an integer minor-unit amount as a plain decimal string
// with the given number of decimal places — no currency symbol and no thousands
// grouping (suited to CSV and round-tripping). Display formatting with symbols
// and grouping lives in the UI layer.
func FormatMinor(amount int64, decimals int) string {
	if decimals <= 0 {
		return strconv.FormatInt(amount, 10)
	}
	neg := amount < 0
	if neg {
		amount = -amount
	}
	div := pow10i(decimals)
	whole := amount / div
	frac := amount % div
	s := fmt.Sprintf("%d.%0*d", whole, decimals, frac)
	if neg {
		return "-" + s
	}
	return s
}

// Format renders the Money's amount with the given number of decimal places.
func (m Money) Format(decimals int) string { return FormatMinor(m.Amount, decimals) }

// Group inserts thousands separators (commas) into the integer portion of a
// plain decimal string as produced by FormatMinor. A leading minus sign and the
// fractional portion (including the decimal point) are preserved untouched —
// e.g. "1234.56" -> "1,234.56" and "-1234567" -> "-1,234,567".
func Group(s string) string {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	intPart, fracPart := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, fracPart = s[:i], s[i:]
	}
	grouped := groupDigits(intPart) + fracPart
	if neg {
		return "-" + grouped
	}
	return grouped
}

// groupDigits comma-groups a run of digits from the right.
func groupDigits(d string) string {
	n := len(d)
	if n <= 3 {
		return d
	}
	var b strings.Builder
	pre := n % 3
	if pre > 0 {
		b.WriteString(d[:pre])
		b.WriteByte(',')
	}
	for i := pre; i < n; i += 3 {
		b.WriteString(d[i : i+3])
		if i+3 < n {
			b.WriteByte(',')
		}
	}
	return b.String()
}

// FormatAccounting renders an integer minor-unit amount in accounting style: the
// currency symbol immediately precedes the comma-grouped magnitude, `decimals`
// fractional places are always shown, and negative values are wrapped in
// parentheses with no minus sign — e.g. 123456 -> "$1,234.56" and
// -24055 -> "($240.55)". The caller supplies the symbol (from internal/currency)
// so this package stays free of the currency registry.
func FormatAccounting(amount int64, decimals int, symbol string) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	body := symbol + Group(FormatMinor(amount, decimals))
	if neg {
		return "(" + body + ")"
	}
	return body
}

// ParseMinor parses a plain decimal string into an integer minor-unit amount
// using the given number of decimal places. More fractional digits than
// `decimals` is an error (to avoid silent rounding). No thousands separators.
func ParseMinor(s string, decimals int) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidAmount
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}

	intPart, fracPart := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, fracPart = s[:i], s[i+1:]
	}
	if intPart == "" && fracPart == "" {
		return 0, ErrInvalidAmount
	}
	if len(fracPart) > decimals {
		return 0, fmt.Errorf("%w: too many decimal places in %q", ErrInvalidAmount, s)
	}
	if intPart == "" {
		intPart = "0"
	}
	for len(fracPart) < decimals {
		fracPart += "0"
	}
	if !allDigits(intPart) || !allDigits(fracPart) {
		return 0, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}

	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	var frac int64
	if fracPart != "" {
		if frac, err = strconv.ParseInt(fracPart, 10, 64); err != nil {
			return 0, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
		}
	}
	val := whole*pow10i(decimals) + frac
	if neg {
		val = -val
	}
	return val, nil
}

func pow10i(n int) int64 {
	p := int64(1)
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
