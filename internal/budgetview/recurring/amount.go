// SPDX-License-Identifier: MIT

// Package recurring prepares the Budgets recurring-bills tile from immutable
// snapshots. It has no browser or component-framework dependencies.
package recurring

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// MonthlyEquivalent normalizes signed minor units, truncating toward zero, with
// wide intermediates and an error when the final result cannot fit in int64.
func MonthlyEquivalent(r domain.Recurring) (int64, error) {
	switch r.Cadence {
	case domain.CadenceWeekly:
		return money.MulDiv(r.Amount.Amount, 52, 12, money.RoundTowardZero)
	case domain.CadenceBiweekly:
		return money.MulDiv(r.Amount.Amount, 26, 12, money.RoundTowardZero)
	case domain.CadenceSemimonthly:
		return money.MulDiv(r.Amount.Amount, 2, 1, money.RoundTowardZero)
	case domain.CadenceQuarterly:
		return money.MulDiv(r.Amount.Amount, 1, 3, money.RoundTowardZero)
	case domain.CadenceYearly:
		return money.MulDiv(r.Amount.Amount, 1, 12, money.RoundTowardZero)
	default:
		return r.Amount.Amount, nil
	}
}

// MonthlyBase negates the normalized bill into base-currency minor units.
// Same-currency values never consult FX. Conversion uses decimal rates and
// rounds once, half away from zero; amounts never pass through floating point.
func MonthlyBase(r domain.Recurring, base string, rates currency.Rates) (int64, error) {
	monthly, err := MonthlyEquivalent(r)
	if err != nil {
		return 0, fmt.Errorf("recurring monthly amount: %w", err)
	}
	positive, err := money.Neg(monthly)
	if err != nil {
		return 0, fmt.Errorf("recurring bill magnitude: %w", err)
	}
	from, to := normalize(r.Amount.Currency), normalize(base)
	if from == to {
		return positive, nil
	}
	fromRate, err := decimalRate(from, rates)
	if err != nil {
		return 0, err
	}
	toRate, err := decimalRate(to, rates)
	if err != nil {
		return 0, err
	}
	value := new(big.Rat).SetInt64(positive)
	value.Mul(value, fromRate).Quo(value, toRate)
	precision := new(big.Rat).SetFrac(pow10(currency.Decimals(to)), pow10(currency.Decimals(from)))
	value.Mul(value, precision)
	return roundMinor(value)
}

// BudgetAmount pairs an unavailable amount with the bill's readable exclusion
// label. Successful conversions carry no exclusion label.
func BudgetAmount(r domain.Recurring, base string, rates currency.Rates, unnamed string) (int64, string, error) {
	amount, err := MonthlyBase(r, base, rates)
	if err != nil {
		return 0, ExcludedLabel(r, unnamed), err
	}
	return amount, "", nil
}

func normalize(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

func decimalRate(code string, rates currency.Rates) (*big.Rat, error) {
	if code == normalize(rates.Base) {
		return big.NewRat(1, 1), nil
	}
	rate, ok := rates.Rates[code]
	if !ok {
		return nil, fmt.Errorf("recurring FX: %w: %q (base %q)", currency.ErrUnknownRate, code, rates.Base)
	}
	value, ok := new(big.Rat).SetString(strconv.FormatFloat(rate, 'g', -1, 64))
	if !ok || value.Sign() <= 0 {
		return nil, fmt.Errorf("recurring FX: invalid rate for %q", code)
	}
	return value, nil
}

func pow10(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }

func roundMinor(value *big.Rat) (int64, error) {
	var quotient, remainder big.Int
	quotient.QuoRem(value.Num(), value.Denom(), &remainder)
	remainder.Abs(&remainder).Lsh(&remainder, 1)
	if remainder.Cmp(value.Denom()) >= 0 {
		quotient.Add(&quotient, big.NewInt(int64(value.Sign())))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("recurring FX: %w", money.ErrOverflow)
	}
	return quotient.Int64(), nil
}

func monthlyMagnitude(r domain.Recurring) int64 {
	monthly, err := MonthlyEquivalent(r)
	if err != nil {
		return 0
	}
	abs, err := money.Abs(monthly)
	if err != nil {
		return 0
	}
	return abs
}
