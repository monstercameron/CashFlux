// SPDX-License-Identifier: MIT

package vitals

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestEvaluateCheckedRejectsUnrepresentablePosition(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Inputs
	}{
		{"debt sum", Inputs{Debts: []Debt{{BalanceMinor: math.MaxInt64/2 + 1}, {BalanceMinor: math.MaxInt64/2 + 1}}}},
		{"minimum sum", Inputs{Debts: []Debt{{BalanceMinor: 1, MinPaymentMinor: math.MaxInt64/2 + 1}, {BalanceMinor: 1, MinPaymentMinor: math.MaxInt64/2 + 1}}}},
		{"annual service", Inputs{Debts: []Debt{{BalanceMinor: 1, MinPaymentMinor: math.MaxInt64/12 + 1}}}},
		{"annual surplus", Inputs{IncomeMonthlyMinor: math.MaxInt64}},
		{"fund target", Inputs{EssentialMonthlyMinor: math.MaxInt64/6 + 1}},
		{"fund gap", Inputs{EssentialMonthlyMinor: 1, LiquidMinor: math.MinInt64}},
		{"interest conversion", Inputs{Debts: []Debt{{BalanceMinor: 100, AprPercent: math.MaxFloat64}}}},
		{"available credit", Inputs{Cards: Cards{HasCards: true, LimitMinor: math.MaxInt64, BalanceMinor: -1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvaluateChecked(tc.in)
			if !errors.Is(err, money.ErrOverflow) || got.Error == nil || got.HasIncome || got.HasDebts || got.HasCards || got.TotalDebtMinor != 0 || got.SurplusAnnualMinor != 0 {
				t.Fatalf("failed result retained apparently valid totals: %+v / %v", got, err)
			}
			if legacy := Evaluate(tc.in); !errors.Is(legacy.Error, money.ErrOverflow) {
				t.Fatalf("compatibility API hid failure: %+v", legacy)
			}
		})
	}
}

func TestEvaluateCheckedWideIntermediateRatiosRemainAccurate(t *testing.T) {
	got, err := EvaluateChecked(Inputs{IncomeMonthlyMinor: math.MaxInt64, ExpenseMonthlyMinor: math.MaxInt64,
		LiquidMinor: math.MaxInt64, EssentialMonthlyMinor: math.MaxInt64 / 10,
		Cards: Cards{HasCards: true, BalanceMinor: math.MaxInt64, LimitMinor: math.MaxInt64}})
	if err != nil {
		t.Fatal(err)
	}
	if got.SavingsRatePct != 0 || got.UtilizationPct != 100 || got.CoverageMonthsTenths != 100 || got.RunwayAfterDebtTenths != 10 || got.CardAvailableMinor != 0 {
		t.Fatalf("wide ratios = %+v", got)
	}
}
