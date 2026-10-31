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
	"math/big"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/money"
)

// ErrUnknownRate is returned when a conversion needs a rate that the table does
// not provide.
var ErrUnknownRate = errors.New("currency: no exchange rate for currency")

// ErrInvalidRate is returned when a required rate is not finite and positive.
var ErrInvalidRate = errors.New("currency: exchange rate must be finite and positive")

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
	"CAD": {"CAD", "CA$", 2, "Canadian Dollar"},
	"AUD": {"AUD", "A$", 2, "Australian Dollar"},
	"CHF": {"CHF", "CHF", 2, "Swiss Franc"},
	"CNY": {"CNY", "CN¥", 2, "Chinese Yuan"}, // C85: distinct from JPY "¥"
	"INR": {"INR", "₹", 2, "Indian Rupee"},
	"MXN": {"MXN", "MX$", 2, "Mexican Peso"},
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

// MinorFromMajor converts a major-unit amount (e.g. 12.34 dollars) to integer
// minor units (e.g. 1234 cents) for the given currency code, rounding to the
// nearest minor unit using the code's own decimal precision (so JPY's 0 decimals
// yield whole units, not hundredths). Use this instead of a hardcoded *100 at any
// boundary that ingests a floating major amount (AI tool args, parsed input).
func MinorFromMajor(major float64, code string) int64 {
	return int64(math.Round(major * math.Pow10(Decimals(code))))
}

// MajorFromMinor converts integer minor units (e.g. 1234 cents) to a major-unit
// float (e.g. 12.34) for the given currency code, using its own decimal precision
// (JPY's 0 decimals yield whole units). Inverse of MinorFromMajor. Intended for
// display/charting boundaries that need a float; domain math should stay in
// integer minor units.
func MajorFromMinor(minor int64, code string) float64 {
	return float64(minor) / math.Pow10(Decimals(code))
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
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, fmt.Errorf("%w: %q", ErrInvalidRate, code)
	}
	return rate, nil
}

// ToBase converts a Money value into the base currency of the rate table.
func (r Rates) ToBase(m money.Money) (money.Money, error) {
	return r.Convert(m, r.Base)
}

// Convert converts a Money value into the target currency by routing through the
// base currency. Amounts are rounded to the target currency's minor units
// (half away from zero). Invalid rates return ErrInvalidRate; a rounded result
// outside the int64 range returns an error wrapping money.ErrOverflow.
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
	rounded := math.Round(toMajor * pow10(Decimals(to)))
	// Ordinary amounts retain the existing fast rounding path. Near the integer
	// limits, float64 loses whole minor units; an exact fallback distinguishes
	// valid MaxInt64/MinInt64 results from overflow before converting to int64.
	// It also preserves finite final values when an intermediate overflowed or
	// lost precision to underflow before division by the target rate.
	if math.IsNaN(rounded) || math.IsInf(rounded, 0) || math.Abs(rounded) >= 0x1p62 ||
		m.Amount != 0 && math.Abs(baseMajor) < 0x1p-1022 {
		return convertExact(m, to, fromRate, toRate)
	}
	toMinor := int64(rounded)

	return money.New(toMinor, to), nil
}

func convertExact(m money.Money, to string, fromRate, toRate float64) (money.Money, error) {
	var result, rate big.Rat
	result.SetInt64(m.Amount)
	rate.SetFloat64(fromRate)
	result.Mul(&result, &rate)
	rate.SetFloat64(toRate)
	result.Quo(&result, &rate)
	rate.SetFrac64(int64(pow10(Decimals(to))), int64(pow10(Decimals(m.Currency))))
	result.Mul(&result, &rate)

	var rounded, remainder big.Int
	rounded.QuoRem(result.Num(), result.Denom(), &remainder)
	remainder.Abs(&remainder)
	remainder.Lsh(&remainder, 1)
	if remainder.Cmp(result.Denom()) >= 0 {
		rounded.Add(&rounded, big.NewInt(int64(result.Sign())))
	}
	if !rounded.IsInt64() {
		return money.Money{}, fmt.Errorf("currency: convert %s to %s: %w", m, to, money.ErrOverflow)
	}
	return money.New(rounded.Int64(), to), nil
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
