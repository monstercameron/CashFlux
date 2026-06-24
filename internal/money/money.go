// SPDX-License-Identifier: MIT

// Package money provides a precise monetary value type for CashFlux.
//
// Money is represented as an integer count of a currency's minor units (for
// example, cents for USD) together with an ISO-4217 currency code. Integer
// arithmetic avoids the rounding errors that floating-point math introduces, so
// all domain calculations operate on Money rather than float64.
//
// This package is pure Go with no platform dependencies; it is unit-tested on
// native Go and reused by the WebAssembly UI layer.
package money

import (
	"errors"
	"fmt"
	"strings"
)

// ErrCurrencyMismatch is returned when an operation combines two Money values
// of different currencies.
var ErrCurrencyMismatch = errors.New("money: currency mismatch")

// ErrOverflow is returned when an arithmetic operation would exceed the range of
// the underlying int64 minor-unit representation.
var ErrOverflow = errors.New("money: amount overflow")

// Money is an amount in a single currency, stored as integer minor units.
type Money struct {
	// Amount is the value in the currency's smallest unit (e.g. cents).
	Amount int64
	// Currency is the uppercase ISO-4217 code (e.g. "USD"). Empty means unset.
	Currency string
}

// New returns a Money value for the given minor-unit amount and currency.
// The currency code is normalized to uppercase.
func New(amount int64, currency string) Money {
	return Money{Amount: amount, Currency: normalizeCode(currency)}
}

// Zero returns a zero-valued Money in the given currency.
func Zero(currency string) Money {
	return Money{Amount: 0, Currency: normalizeCode(currency)}
}

func normalizeCode(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

// IsZero reports whether the amount is zero.
func (m Money) IsZero() bool { return m.Amount == 0 }

// IsPositive reports whether the amount is greater than zero.
func (m Money) IsPositive() bool { return m.Amount > 0 }

// IsNegative reports whether the amount is less than zero.
func (m Money) IsNegative() bool { return m.Amount < 0 }

// Neg returns the additive inverse of m.
func (m Money) Neg() Money {
	return Money{Amount: -m.Amount, Currency: m.Currency}
}

// Abs returns the absolute value of m.
func (m Money) Abs() Money {
	if m.Amount < 0 {
		return m.Neg()
	}
	return m
}

// Add returns m + n. It fails if the two values are in different currencies or
// if the sum would overflow the int64 minor-unit range.
func (m Money) Add(n Money) (Money, error) {
	if m.Currency != n.Currency {
		return Money{}, currencyMismatch(m, n)
	}
	sum := m.Amount + n.Amount
	// Signed addition overflows when the result's sign disagrees with the
	// (equal) signs of both operands.
	if (n.Amount > 0 && sum < m.Amount) || (n.Amount < 0 && sum > m.Amount) {
		return Money{}, fmt.Errorf("%w: %d + %d", ErrOverflow, m.Amount, n.Amount)
	}
	return Money{Amount: sum, Currency: m.Currency}, nil
}

// Sub returns m - n. It fails if the two values are in different currencies or
// if the difference would overflow the int64 minor-unit range.
func (m Money) Sub(n Money) (Money, error) {
	if m.Currency != n.Currency {
		return Money{}, currencyMismatch(m, n)
	}
	diff := m.Amount - n.Amount
	// Signed subtraction overflows when the operands' signs differ and the
	// result's sign disagrees with the minuend's.
	if (n.Amount < 0 && diff < m.Amount) || (n.Amount > 0 && diff > m.Amount) {
		return Money{}, fmt.Errorf("%w: %d - %d", ErrOverflow, m.Amount, n.Amount)
	}
	return Money{Amount: diff, Currency: m.Currency}, nil
}

// Cmp compares m and n, returning -1 if m < n, 0 if equal, and +1 if m > n.
// It fails if the two values are in different currencies.
func (m Money) Cmp(n Money) (int, error) {
	if m.Currency != n.Currency {
		return 0, currencyMismatch(m, n)
	}
	switch {
	case m.Amount < n.Amount:
		return -1, nil
	case m.Amount > n.Amount:
		return 1, nil
	default:
		return 0, nil
	}
}

// Equal reports whether m and n have the same currency and amount.
func (m Money) Equal(n Money) bool {
	return m.Currency == n.Currency && m.Amount == n.Amount
}

// String renders a debug representation such as "1234 USD" (minor units).
// Human-facing formatting (symbols, decimals, grouping) lives in the formatting
// layer, not here.
func (m Money) String() string {
	return fmt.Sprintf("%d %s", m.Amount, m.Currency)
}

func currencyMismatch(m, n Money) error {
	return fmt.Errorf("%w: %q vs %q", ErrCurrencyMismatch, m.Currency, n.Currency)
}

// Sum adds any number of same-currency Money values. With no arguments it
// returns the zero value. It fails on the first currency mismatch.
func Sum(values ...Money) (Money, error) {
	if len(values) == 0 {
		return Money{}, nil
	}
	total := values[0]
	for _, v := range values[1:] {
		next, err := total.Add(v)
		if err != nil {
			return Money{}, err
		}
		total = next
	}
	return total, nil
}
