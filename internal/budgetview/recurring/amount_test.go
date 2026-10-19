// SPDX-License-Identifier: MIT

package recurring

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func bill(id string, amount int64, code string, cadence domain.RecurringCadence) domain.Recurring {
	return domain.Recurring{ID: id, Label: id, Amount: money.New(amount, code), Cadence: cadence}
}

func TestMonthlyEquivalent(t *testing.T) {
	for _, tc := range []struct {
		cadence      domain.RecurringCadence
		amount, want int64
		overflow     bool
	}{
		{domain.CadenceWeekly, -1200, -5200, false},
		{domain.CadenceBiweekly, -1200, -2600, false},
		{domain.CadenceSemimonthly, -1200, -2400, false},
		{domain.CadenceQuarterly, -1201, -400, false},
		{domain.CadenceYearly, -1201, -100, false},
		{domain.CadenceMonthly, math.MinInt64, math.MinInt64, false},
		{"unknown", 12, 12, false},
		{domain.CadenceWeekly, 0, 0, false},
		{domain.CadenceYearly, -1, 0, false},
		{domain.CadenceWeekly, math.MinInt64, 0, true},
		{domain.CadenceSemimonthly, math.MaxInt64, 0, true},
		{domain.CadenceQuarterly, math.MaxInt64, math.MaxInt64 / 3, false},
	} {
		t.Run(string(tc.cadence)+money.FormatMinor(tc.amount, 0), func(t *testing.T) {
			got, err := MonthlyEquivalent(bill("", tc.amount, "USD", tc.cadence))
			if got != tc.want || (err != nil) != tc.overflow || tc.overflow && !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("got %d, %v; want %d overflow=%v", got, err, tc.want, tc.overflow)
			}
		})
	}
}

func TestMonthlyBase(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
		amount         int64
		cadence        domain.RecurringCadence
		rates          currency.Rates
		want           int64
		err            error
		fails          bool
	}{
		{"same no rates", "USD", "USD", -123, domain.CadenceMonthly, currency.Rates{}, 123, nil, false},
		{"normalized codes", " usd ", "usd", -12, "", currency.Rates{}, 12, nil, false},
		{"zero", "USD", "USD", 0, "", currency.Rates{}, 0, nil, false},
		{"positive remains signed", "USD", "USD", 123, "", currency.Rates{}, -123, nil, false},
		{"weekly overflow", "USD", "USD", math.MinInt64, domain.CadenceWeekly, currency.Rates{}, 0, money.ErrOverflow, true},
		{"negation overflow", "USD", "USD", math.MinInt64, "", currency.Rates{}, 0, money.ErrOverflow, true},
		{"missing from", "EUR", "USD", -10, "", currency.Rates{Base: "USD"}, 0, currency.ErrUnknownRate, true},
		{"missing to", "USD", "EUR", -10, "", currency.Rates{Base: "USD"}, 0, currency.ErrUnknownRate, true},
		{"decimal FX", "EUR", "USD", -100, "", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.08}}, 108, nil, false},
		{"cross FX", "EUR", "GBP", -150, "", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.2, "GBP": 1.5}}, 120, nil, false},
		{"JPY precision", "USD", "JPY", -100, "", currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.01}}, 100, nil, false},
		{"half up", "JPY", "USD", -1, "", currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.005}}, 1, nil, false},
		{"negative half", "JPY", "USD", 1, "", currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.005}}, -1, nil, false},
		{"large exact", "EUR", "USD", -math.MaxInt64, "", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1}}, math.MaxInt64, nil, false},
		{"FX overflow", "EUR", "USD", -math.MaxInt64, "", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, 0, money.ErrOverflow, true},
		{"zero rate", "EUR", "USD", -1, "", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0}}, 0, nil, true},
		{"negative rate", "EUR", "USD", -1, "", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": -1}}, 0, nil, true},
		{"NaN", "EUR", "USD", -1, "", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": math.NaN()}}, 0, nil, true},
		{"infinite", "EUR", "USD", -1, "", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": math.Inf(1)}}, 0, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MonthlyBase(bill("", tc.amount, tc.from, tc.cadence), tc.to, tc.rates)
			if got != tc.want || (err != nil) != tc.fails || tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("got %d, %v; want %d error=%v", got, err, tc.want, tc.err)
			}
		})
	}
}

func FuzzMonthlyEquivalentMatchesRational(f *testing.F) {
	f.Add(int64(math.MinInt64), uint8(0))
	f.Add(int64(math.MaxInt64), uint8(2))
	f.Add(int64(-1201), uint8(4))
	f.Fuzz(func(t *testing.T, amount int64, choice uint8) {
		cadences := []domain.RecurringCadence{domain.CadenceWeekly, domain.CadenceBiweekly, domain.CadenceSemimonthly, domain.CadenceQuarterly, domain.CadenceYearly, domain.CadenceMonthly}
		numerators, denominators := []int64{52, 26, 2, 1, 1, 1}, []int64{12, 12, 1, 3, 12, 1}
		i := int(choice) % len(cadences)
		want := new(big.Int).Mul(big.NewInt(amount), big.NewInt(numerators[i]))
		want.Quo(want, big.NewInt(denominators[i]))
		got, err := MonthlyEquivalent(bill("", amount, "USD", cadences[i]))
		if want.IsInt64() {
			if err != nil || got != want.Int64() {
				t.Fatalf("got %d %v want %v", got, err, want)
			}
		} else if !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("wanted overflow, got %d %v", got, err)
		}
	})
}

func TestBudgetAmount(t *testing.T) {
	for _, tc := range []struct {
		label, code, excluded string
		want                  int64
		fail                  bool
	}{
		{"Rent", "USD", "", 1200, false},
		{" Rent ", "EUR", "Rent", 0, true},
		{" \t", "EUR", "Unnamed bill", 0, true},
	} {
		r := bill(tc.label, -1200, tc.code, domain.CadenceMonthly)
		got, excluded, err := BudgetAmount(r, "USD", currency.Rates{Base: "USD"}, "Unnamed bill")
		if got != tc.want || excluded != tc.excluded || (err != nil) != tc.fail {
			t.Fatalf("%+v: %d %q %v", tc, got, excluded, err)
		}
	}
}

func FuzzMonthlyBaseIdentity(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(math.MinInt64))
	f.Add(int64(math.MaxInt64))
	f.Fuzz(func(t *testing.T, amount int64) {
		got, err := MonthlyBase(bill("", amount, "USD", domain.CadenceMonthly), "USD", currency.Rates{})
		want, wantErr := money.Neg(amount)
		if got != want || (err != nil) != (wantErr != nil) {
			t.Fatalf("identity %d: %d %v", amount, got, err)
		}
	})
}
