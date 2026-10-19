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

// Neg returns the additive inverse of m. It panics with ErrOverflow when the
// result cannot be represented.
//
// Deprecated: use NegChecked to handle overflow as an error.
func (m Money) Neg() Money {
	result, err := m.NegChecked()
	if err != nil {
		panic(err)
	}
	return result
}

// NegChecked returns the additive inverse of m, or ErrOverflow for MinInt64.
func (m Money) NegChecked() (Money, error) {
	amount, err := Neg(m.Amount)
	if err != nil {
		return Money{}, err
	}
	return Money{Amount: amount, Currency: m.Currency}, nil
}

// Abs returns the absolute value of m. It panics with ErrOverflow for MinInt64.
//
// Deprecated: use AbsChecked to handle overflow as an error.
func (m Money) Abs() Money {
	result, err := m.AbsChecked()
	if err != nil {
		panic(err)
	}
	return result
}

// AbsChecked returns the magnitude of m, or ErrOverflow for MinInt64.
func (m Money) AbsChecked() (Money, error) {
	amount, err := Abs(m.Amount)
	if err != nil {
		return Money{}, err
	}
	return Money{Amount: amount, Currency: m.Currency}, nil
}

// Add returns m+n, checking both currency and overflow.
func (m Money) Add(n Money) (Money, error) {
	if m.Currency != n.Currency {
		return Money{}, currencyMismatch(m, n)
	}
	amount, err := Add(m.Amount, n.Amount)
	if err != nil {
		return Money{}, err
	}
	return Money{Amount: amount, Currency: m.Currency}, nil
}

// Sub returns m-n, checking both currency and overflow.
func (m Money) Sub(n Money) (Money, error) {
	if m.Currency != n.Currency {
		return Money{}, currencyMismatch(m, n)
	}
	amount, err := Sub(m.Amount, n.Amount)
	if err != nil {
		return Money{}, err
	}
	return Money{Amount: amount, Currency: m.Currency}, nil
}

// Mul multiplies m by an integer, checking overflow and preserving currency.
func (m Money) Mul(factor int64) (Money, error) {
	return m.MulDiv(factor, 1, RoundTowardZero)
}

// MulDiv scales m by num/den with explicit rounding, checking overflow and
// preserving currency. Only the final rounded amount must fit in int64.
func (m Money) MulDiv(num, den int64, mode RoundingMode) (Money, error) {
	amount, err := MulDiv(m.Amount, num, den, mode)
	if err != nil {
		return Money{}, err
	}
	return Money{Amount: amount, Currency: m.Currency}, nil
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
// returns the zero value. It fails on the first currency mismatch or partial-sum
// overflow.
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
