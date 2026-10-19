// SPDX-License-Identifier: MIT

package money

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ErrInvalidAmount is returned when a string cannot be parsed as a money amount.
var ErrInvalidAmount = errors.New("money: invalid amount")

// MaxDecimals is the largest supported decimal precision. It bounds parsing
// and formatting work and includes all currency exponents used by CashFlux.
const MaxDecimals = 18

// MaxAmountLength bounds input bytes, including any surrounding whitespace.
const MaxAmountLength = 128

// ErrInvalidPrecision is returned for a precision outside [0, MaxDecimals].
var ErrInvalidPrecision = errors.New("money: invalid decimal precision")

// FormatMinor renders exact minor units as a plain decimal string, without
// rounding, symbols or grouping. It panics for unsupported precision.
//
// Deprecated: use FormatMinorChecked to handle invalid precision as an error.
func FormatMinor(amount int64, decimals int) string {
	result, err := FormatMinorChecked(amount, decimals)
	if err != nil {
		panic(err)
	}
	return result
}

// FormatMinorChecked renders exact minor units without rounding, symbols or
// grouping. Precision must be in [0, MaxDecimals]. MinInt64 is supported.
func FormatMinorChecked(amount int64, decimals int) (string, error) {
	if decimals < 0 || decimals > MaxDecimals {
		return "", ErrInvalidPrecision
	}
	digits := strconv.FormatUint(magnitude(amount), 10)
	if decimals > 0 {
		if len(digits) <= decimals {
			digits = strings.Repeat("0", decimals+1-len(digits)) + digits
		}
		point := len(digits) - decimals
		digits = digits[:point] + "." + digits[point:]
	}
	if amount < 0 {
		digits = "-" + digits
	}
	return digits, nil
}

// Format renders the Money's amount with the given number of decimal places.
// It panics for unsupported precision.
//
// Deprecated: use FormatChecked to handle invalid precision as an error.
func (m Money) Format(decimals int) string { return FormatMinor(m.Amount, decimals) }

// FormatChecked renders the Money's exact amount, checking decimal precision.
func (m Money) FormatChecked(decimals int) (string, error) {
	return FormatMinorChecked(m.Amount, decimals)
}

// magnitude returns an unsigned magnitude without negating MinInt64.
func magnitude(amount int64) uint64 {
	if amount < 0 {
		return uint64(-(amount + 1)) + 1
	}
	return uint64(amount)
}

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

// FormatAccounting renders an exact amount with the supplied symbol, comma
// grouping and parentheses for negatives. It panics for unsupported precision.
//
// Deprecated: use FormatAccountingChecked to handle invalid precision as an error.
func FormatAccounting(amount int64, decimals int, symbol string) string {
	result, err := FormatAccountingChecked(amount, decimals, symbol)
	if err != nil {
		panic(err)
	}
	return result
}

// FormatAccountingChecked renders an exact amount with the supplied symbol,
// comma grouping and parentheses for negatives, checking decimal precision.
func FormatAccountingChecked(amount int64, decimals int, symbol string) (string, error) {
	plain, err := FormatMinorChecked(amount, decimals)
	if err != nil {
		return "", err
	}
	body := symbol + Group(strings.TrimPrefix(plain, "-"))
	if amount < 0 {
		return "(" + body + ")", nil
	}
	return body, nil
}

// ParseMinor parses a plain decimal into exact minor units. It accepts optional
// surrounding whitespace, one leading sign, and omitted digits on either side
// of the point (but not both). Precision must be in [0, MaxDecimals]; input is
// limited to MaxAmountLength bytes. Excess precision, malformed input and amounts
// outside int64 are errors. No rounding or thousands separators are supported.
// Range errors match both ErrInvalidAmount and ErrOverflow.
func ParseMinor(s string, decimals int) (int64, error) {
	if decimals < 0 || decimals > MaxDecimals {
		return 0, fmt.Errorf("%w: %w", ErrInvalidAmount, ErrInvalidPrecision)
	}
	if len(s) > MaxAmountLength {
		return 0, fmt.Errorf("%w: input is too long", ErrInvalidAmount)
	}
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
	if (intPart == "" && fracPart == "") || !allDigits(intPart) || !allDigits(fracPart) {
		return 0, fmt.Errorf("%w: malformed decimal", ErrInvalidAmount)
	}
	if len(fracPart) > decimals {
		return 0, fmt.Errorf("%w: too many decimal places", ErrInvalidAmount)
	}
	limit := uint64(math.MaxInt64)
	if neg {
		limit++ // The negative range has one additional representable magnitude.
	}
	var value uint64
	appendDigit := func(digit uint64) bool {
		if value > (limit-digit)/10 {
			return false
		}
		value = value*10 + digit
		return true
	}
	for _, part := range []string{intPart, fracPart} {
		for i := 0; i < len(part); i++ {
			if !appendDigit(uint64(part[i] - '0')) {
				return 0, fmt.Errorf("%w: %w", ErrInvalidAmount, ErrOverflow)
			}
		}
	}
	for i := len(fracPart); i < decimals; i++ {
		if !appendDigit(0) {
			return 0, fmt.Errorf("%w: %w", ErrInvalidAmount, ErrOverflow)
		}
	}
	if neg {
		if value == uint64(1)<<63 {
			return math.MinInt64, nil
		}
		return -int64(value), nil
	}
	return int64(value), nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
