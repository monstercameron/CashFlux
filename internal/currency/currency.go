// SPDX-License-Identifier: MIT

// Package currency provides a currency registry and multi-currency conversion
// for CashFlux. Accounts each hold their own currency; aggregate views convert
// to a user-chosen base currency using a manually maintained rate table.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package currency

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/money"
)

// ErrUnknownRate is returned when a conversion needs a rate that the table does
// not provide.
var ErrUnknownRate = errors.New("currency: no exchange rate for currency")

// Currency describes a currency for display and minor-unit math.
type Currency struct {
	Code     string // ISO-4217 code, uppercase (e.g. "USD")
	Symbol   string // display symbol (e.g. "$")
	Decimals int    // number of minor-unit digits (2 for USD, 0 for JPY)
	Name     string // human-readable name
}

// registry holds the currencies CashFlux knows about out of the box. Users can
// transact in any code; unknown codes fall back to defaults via Decimals/Symbol.
var registry = map[string]Currency{
	"USD": {"USD", "$", 2, "US Dollar"},
	"EUR": {"EUR", "€", 2, "Euro"},
	"GBP": {"GBP", "£", 2, "British Pound"},
	"JPY": {"JPY", "¥", 0, "Japanese Yen"},
	"CAD": {"CAD", "$", 2, "Canadian Dollar"},
	"AUD": {"AUD", "$", 2, "Australian Dollar"},
	"CHF": {"CHF", "CHF", 2, "Swiss Franc"},
	"CNY": {"CNY", "¥", 2, "Chinese Yuan"},
	"INR": {"INR", "₹", 2, "Indian Rupee"},
	"MXN": {"MXN", "$", 2, "Mexican Peso"},
}

const defaultDecimals = 2

// Codes returns the registered currency codes, sorted — for building base-currency
// and exchange-rate pickers.
func Codes() []string {
	out := make([]string, 0, len(registry))
	for code := range registry {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

// Lookup returns the registered currency for a code and whether it was found.
func Lookup(code string) (Currency, bool) {
	c, ok := registry[normalize(code)]
	return c, ok
}

// Valid reports whether code is a known, registered currency (case-insensitive).
// A validated currency picker uses this to reject typos that would silently break
// conversion.
func Valid(code string) bool {
	_, ok := registry[normalize(code)]
	return ok
}

// List returns every registered currency sorted by code — for building a labelled
// currency picker (code + name).
func List() []Currency {
	out := make([]Currency, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// Decimals returns the minor-unit digit count for a code, defaulting to 2 for
// unknown codes.
func Decimals(code string) int {
	if c, ok := registry[normalize(code)]; ok {
		return c.Decimals
	}
	return defaultDecimals
}

// Symbol returns the display symbol for a code, defaulting to the code itself.
func Symbol(code string) string {
	if c, ok := registry[normalize(code)]; ok {
		return c.Symbol
	}
	return normalize(code)
}

func normalize(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

// Rates is a manually maintained exchange-rate table. Each entry is the value of
// one major unit of that currency expressed in major units of Base (e.g. with
// Base "USD", Rates["EUR"] = 1.08 means 1 EUR = 1.08 USD). The base currency
// itself always converts at 1.0 and need not appear in the map.
type Rates struct {
	Base  string
	Rates map[string]float64
}

// rateToBase returns how many base units one unit of code is worth.
func (r Rates) rateToBase(code string) (float64, error) {
	code = normalize(code)
	if code == normalize(r.Base) {
		return 1, nil
	}
	rate, ok := r.Rates[code]
	if !ok {
		return 0, fmt.Errorf("%w: %q (base %q)", ErrUnknownRate, code, r.Base)
	}
	if rate <= 0 {
		return 0, fmt.Errorf("currency: non-positive rate for %q", code)
	}
	return rate, nil
}

// ToBase converts a Money value into the base currency of the rate table.
func (r Rates) ToBase(m money.Money) (money.Money, error) {
	return r.Convert(m, r.Base)
}

// Convert converts a Money value into the target currency by routing through the
// base currency. Amounts are rounded to the target currency's minor units
// (half away from zero).
func (r Rates) Convert(m money.Money, toCurrency string) (money.Money, error) {
	to := normalize(toCurrency)
	if m.Currency == to {
		return m, nil
	}

	fromRate, err := r.rateToBase(m.Currency)
	if err != nil {
		return money.Money{}, err
	}
	toRate, err := r.rateToBase(to)
	if err != nil {
		return money.Money{}, err
	}

	fromMajor := float64(m.Amount) / pow10(Decimals(m.Currency))
	baseMajor := fromMajor * fromRate
	toMajor := baseMajor / toRate
	toMinor := int64(math.Round(toMajor * pow10(Decimals(to))))

	return money.New(toMinor, to), nil
}

// ConvertBetween converts amt (in integer minor units of from) to integer minor
// units of to, routing through the base currency of rates. It is a pure-Go
// helper for business logic that works with raw int64 amounts rather than
// money.Money values.
//
// Same-currency conversions are identity and never consult the rate table.
// When a required rate is missing, ConvertBetween returns 0 and a descriptive
// error wrapping ErrUnknownRate.
func ConvertBetween(amt int64, from, to string, rates Rates) (int64, error) {
	result, err := rates.Convert(money.New(amt, from), to)
	if err != nil {
		return 0, err
	}
	return result.Amount, nil
}

func pow10(n int) float64 {
	p := 1.0
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}
