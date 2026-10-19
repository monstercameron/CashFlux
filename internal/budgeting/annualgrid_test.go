// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBuildAnnualGrid(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "b1", Name: "Dining", CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(20000)} // $200/mo
	all := []domain.Transaction{
		expense(15000, "USD", "food", "", "2026-01-12"), // Jan $150 (under)
		expense(25000, "USD", "food", "", "2026-02-08"), // Feb $250 (over)
		expense(5000, "USD", "food", "", "2026-06-20"),  // Jun $50
		expense(9999, "USD", "rent", "", "2026-02-01"),  // unrelated
	}

	grid, err := BuildAnnualGrid([]domain.Budget{budget}, all, 2026, rates, time.Sunday, now, nil)
	if err != nil {
		t.Fatalf("BuildAnnualGrid: %v", err)
	}
	if len(grid.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(grid.Rows))
	}
	row := grid.Rows[0]

	// Plan is $200 every month.
	if row.Cells[0].Plan.Amount != 20000 {
		t.Errorf("Jan plan = %d, want 20000", row.Cells[0].Plan.Amount)
	}
	// Jan actual $150, under.
	if row.Cells[0].Actual.Amount != 15000 || row.Cells[0].Over {
		t.Errorf("Jan cell = %d over=%v, want 15000/false", row.Cells[0].Actual.Amount, row.Cells[0].Over)
	}
	// Feb actual $250, over.
	if row.Cells[1].Actual.Amount != 25000 || !row.Cells[1].Over {
		t.Errorf("Feb cell = %d over=%v, want 25000/true", row.Cells[1].Actual.Amount, row.Cells[1].Over)
	}
	// Row totals: plan $200×12 = $2400; actual = 150+250+50 = $450.
	if row.PlanTotal.Amount != 240000 {
		t.Errorf("plan total = %d, want 240000", row.PlanTotal.Amount)
	}
	if row.ActualTotal.Amount != 45000 {
		t.Errorf("actual total = %d, want 45000", row.ActualTotal.Amount)
	}
	// Column totals.
	if grid.MonthActualTotals[1].Amount != 25000 {
		t.Errorf("Feb column = %d, want 25000", grid.MonthActualTotals[1].Amount)
	}
	if grid.GrandActual.Amount != 45000 || grid.GrandPlan.Amount != 240000 {
		t.Errorf("grand = %d/%d, want 45000/240000", grid.GrandActual.Amount, grid.GrandPlan.Amount)
	}
	// March is the current month for a 2026 grid (0-based → 2).
	if grid.CurrentMonth != 2 {
		t.Errorf("CurrentMonth = %d, want 2", grid.CurrentMonth)
	}
}

func TestBuildAnnualGridPastYearNoHighlight(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "b1", Name: "Dining", CategoryID: "food", Limit: usd(20000)}
	grid, err := BuildAnnualGrid([]domain.Budget{budget}, nil, 2025, rates, time.Sunday, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if grid.CurrentMonth != -1 {
		t.Errorf("CurrentMonth = %d, want -1 for a past year", grid.CurrentMonth)
	}
}

func TestMonthlyLimitEquivalent(t *testing.T) {
	tests := []struct {
		period domain.Period
		limit  int64
		want   int64
	}{
		{domain.PeriodMonthly, 20000, 20000},
		{domain.PeriodWeekly, 10000, (10000*52 + 6) / 12},
		{domain.PeriodQuarterly, 60000, 20000},
		{domain.PeriodYearly, 240000, 20000},
		{domain.PeriodSemimonthly, 10000, 20000},
	}
	for _, tc := range tests {
		got, err := PeriodEquivalent(usd(tc.limit), tc.period, domain.PeriodMonthly)
		if err != nil {
			t.Fatalf("PeriodEquivalent(%s, %d): %v", tc.period, tc.limit, err)
		}
		if got.Amount != tc.want {
			t.Errorf("PeriodEquivalent(%s, %d) = %d, want %d", tc.period, tc.limit, got.Amount, tc.want)
		}
	}
}

func TestAnnualGridSavingRowsDoNotCountAsSpendingOrOver(t *testing.T) {
	budget := domain.Budget{ID: "save", Name: "Emergency fund", Limit: usd(10000), Direction: domain.DirectionSave}
	transaction := expense(15000, "USD", "savings", "", "2026-01-10")
	budget.CategoryID = "savings"
	grid, err := BuildAnnualGrid([]domain.Budget{budget}, []domain.Transaction{transaction}, 2026, currency.Rates{Base: "USD"}, time.Monday,
		time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Rows[0].Cells[0].Actual.Amount != 15000 || grid.Rows[0].Cells[0].Over {
		t.Fatal("an on-target saving row must not be toned as overspending")
	}
	if grid.GrandPlan.Amount != 0 || grid.GrandActual.Amount != 0 || grid.MonthPlanTotals[0].Amount != 0 {
		t.Fatalf("saving row leaked into spending totals: grand=%+v/%+v month=%+v", grid.GrandPlan, grid.GrandActual, grid.MonthPlanTotals[0])
	}
}

func TestAnnualGridUsesMonthlyLimitHistory(t *testing.T) {
	budget := domain.Budget{
		ID: "history", Limit: usd(10000), Period: domain.PeriodMonthly,
		LimitHistory: map[string]int64{"2026-01-01": 10000, "2026-03-01": 25000},
	}
	grid, err := BuildAnnualGrid([]domain.Budget{budget}, nil, 2026, currency.Rates{Base: "USD"}, time.Monday, time.Time{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Rows[0].Cells[0].Plan.Amount != 10000 || grid.Rows[0].Cells[1].Plan.Amount != 10000 || grid.Rows[0].Cells[2].Plan.Amount != 25000 {
		t.Fatalf("monthly plans ignored history: Jan=%d Feb=%d Mar=%d", grid.Rows[0].Cells[0].Plan.Amount, grid.Rows[0].Cells[1].Plan.Amount, grid.Rows[0].Cells[2].Plan.Amount)
	}
}

func TestAnnualGridKeepsRowsWithFXFailuresVisible(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "usd", Name: "Groceries", Limit: money.New(10000, "USD")},
		{ID: "eur", Name: "Travel", Limit: money.New(10000, "EUR")},
	}
	grid, err := BuildAnnualGrid(budgets, nil, 2026, currency.Rates{Base: "USD"}, time.Monday, time.Time{}, nil)
	if err != nil {
		t.Fatalf("one missing rate should not fail the whole grid: %v", err)
	}
	if len(grid.Rows) != 2 || grid.Rows[0].Error != nil || grid.Rows[1].Error == nil {
		t.Fatalf("row errors = [%v, %v], want first complete and second failed", grid.Rows[0].Error, grid.Rows[1].Error)
	}
	if grid.GrandPlan.Amount != 120000 {
		t.Fatalf("grand plan = %d, want complete USD row only (120000)", grid.GrandPlan.Amount)
	}
}
