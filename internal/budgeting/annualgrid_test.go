// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
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
		got := monthlyLimitEquivalent(tc.period, usd(tc.limit))
		if got.Amount != tc.want {
			t.Errorf("monthlyLimitEquivalent(%s, %d) = %d, want %d", tc.period, tc.limit, got.Amount, tc.want)
		}
	}
}
