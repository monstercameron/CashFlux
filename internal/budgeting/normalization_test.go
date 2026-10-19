// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNormalizeBudgetsToViewedMonth(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "annual", Name: "Splurges", Period: domain.PeriodYearly, Limit: money.New(600000, "USD")},
		{ID: "quarterly", Name: "Travel", Period: domain.PeriodQuarterly, Limit: money.New(60000, "USD")},
		{ID: "weekly", Name: "Entertainment", Period: domain.PeriodWeekly, Limit: money.New(2500, "USD")},
	}
	got, err := NormalizeBudgets(budgets, domain.PeriodMonthly, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total.Amount != 80833 {
		t.Errorf("monthly normalized total = %d, want 80833 cents", got.Total.Amount)
	}
	want := []int64{50000, 20000, 10833}
	for i, line := range got.Lines {
		if line.Amount.Amount != want[i] {
			t.Errorf("%s normalized = %d, want %d", line.BudgetID, line.Amount.Amount, want[i])
		}
	}
}

func TestAdjustAllPreviewNormalizedKeepsEachBudgetCadence(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "annual", Name: "Splurges", Period: domain.PeriodYearly, Limit: money.New(600000, "USD")},
		{ID: "quarterly", Name: "Travel", Period: domain.PeriodQuarterly, Limit: money.New(60000, "USD")},
	}
	preview, err := AdjustAllPreviewNormalized(budgets, -10, AdjustEveryPeriod, AdjustOverlays{"annual": 0, "quarterly": 0}, domain.PeriodMonthly, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Lines[0].After != 540000 || preview.Lines[1].After != 54000 {
		t.Errorf("own-period limits after adjustment = %d, %d; want 540000/year and 54000/quarter", preview.Lines[0].After, preview.Lines[1].After)
	}
	if preview.NormalizedBefore.Amount != 70000 || preview.NormalizedAfter.Amount != 63000 {
		t.Errorf("normalized preview totals = %d → %d, want 70000 → 63000 cents/month", preview.NormalizedBefore.Amount, preview.NormalizedAfter.Amount)
	}
}

func TestPeriodEquivalentRoundsOnceHalfAwayFromZero(t *testing.T) {
	got, err := PeriodEquivalent(money.New(1, "USD"), domain.PeriodYearly, domain.PeriodMonthly)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 0 {
		t.Errorf("one cent yearly equivalent = %d, want 0 cents monthly", got.Amount)
	}
	negative, err := PeriodEquivalent(money.New(-6, "USD"), domain.PeriodYearly, domain.PeriodMonthly)
	if err != nil {
		t.Fatal(err)
	}
	if negative.Amount != -1 {
		t.Errorf("negative half-cent equivalent = %d, want -1", negative.Amount)
	}
}
