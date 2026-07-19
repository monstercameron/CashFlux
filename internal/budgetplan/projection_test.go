// SPDX-License-Identifier: MIT

package budgetplan

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(minor int64) money.Money { return money.New(minor, "USD") }

var usdRates = currency.Rates{Base: "USD"}

// sumMonths reports the total across a year of amounts (test convenience).
func sumMonths(a MonthAmounts) int64 {
	var s int64
	for _, v := range a {
		s += v
	}
	return s
}

func TestProjectRecurring(t *testing.T) {
	feb15 := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		rec       domain.Recurring
		year      int
		fromMonth int
		wantCat   string
		wantMonth map[int]int64 // monthIndex -> expected minor
	}{
		{
			name:      "monthly bill fills every occurrence from Feb",
			rec:       domain.Recurring{ID: "r1", Amount: usd(-12000), Cadence: domain.CadenceMonthly, NextDue: feb15, CategoryID: "rent"},
			year:      2026,
			fromMonth: 1,
			wantCat:   "rent",
			wantMonth: map[int]int64{0: 0, 1: 12000, 6: 12000, 11: 12000},
		},
		{
			name:      "fromMonth cutoff skips earlier occurrences",
			rec:       domain.Recurring{ID: "r2", Amount: usd(-5000), Cadence: domain.CadenceMonthly, NextDue: feb15, CategoryID: "rent"},
			year:      2026,
			fromMonth: 6,
			wantCat:   "rent",
			wantMonth: map[int]int64{5: 0, 6: 5000, 11: 5000},
		},
		{
			name:      "quarterly bill lands on its cadence months",
			rec:       domain.Recurring{ID: "r3", Amount: usd(-30000), Cadence: domain.CadenceQuarterly, NextDue: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), CategoryID: "insurance"},
			year:      2026,
			fromMonth: 0,
			wantCat:   "insurance",
			wantMonth: map[int]int64{2: 30000, 5: 30000, 8: 30000, 11: 30000, 0: 0, 1: 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Project([]domain.Recurring{tt.rec}, nil, tt.year, tt.fromMonth, "USD", usdRates)
			got := p.Recurring[tt.wantCat]
			for m, want := range tt.wantMonth {
				if got[m] != want {
					t.Errorf("month %d = %d, want %d", m, got[m], want)
				}
			}
		})
	}
}

func TestProjectIgnoresIncomeAndBlankDue(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "inc", Amount: usd(200000), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), CategoryID: "pay"}, // income, ignored
		{ID: "blank", Amount: usd(-5000), Cadence: domain.CadenceMonthly, CategoryID: "x"},                                                                   // zero NextDue, skipped
	}
	p := Project(recs, nil, 2026, 0, "USD", usdRates)
	if len(p.Recurring) != 0 {
		t.Fatalf("expected no projected recurring outflow, got %v", p.Recurring)
	}
}

func TestProjectGoals(t *testing.T) {
	byCat := domain.Goal{ID: "g1", MonthlyContribution: usd(5000), CategoryID: "savings"}
	byBudget := domain.Goal{ID: "g2", MonthlyContribution: usd(3000), BudgetIDs: []string{"b1", "b2"}}
	archived := domain.Goal{ID: "g3", MonthlyContribution: usd(9999), CategoryID: "savings", Archived: true}

	p := Project(nil, []domain.Goal{byCat, byBudget, archived}, 2026, 1, "USD", usdRates)

	// 11 projected months (Feb..Dec), $50 each.
	if got := p.GoalsByCategory["savings"]; sumMonths(got) != 11*5000 || got[0] != 0 || got[1] != 5000 {
		t.Errorf("GoalsByCategory[savings] = %v", got)
	}
	for _, b := range []string{"b1", "b2"} {
		if got := p.GoalsByBudget[b]; sumMonths(got) != 11*3000 || got[0] != 0 || got[11] != 3000 {
			t.Errorf("GoalsByBudget[%s] = %v", b, got)
		}
	}
}

func TestPerBudgetFold(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "r1", Amount: usd(-12000), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC), CategoryID: "rent"},
	}
	gs := []domain.Goal{
		{ID: "g1", MonthlyContribution: usd(5000), CategoryID: "rent"},        // folds via covered category
		{ID: "g2", MonthlyContribution: usd(2000), BudgetIDs: []string{"b1"}}, // folds directly onto the budget
	}
	p := Project(recs, gs, 2026, 1, "USD", usdRates)

	covers := map[string]map[string]bool{"b1": {"rent": true}}
	per := p.PerBudget([]string{"b1"}, covers)
	row := per["b1"]
	// Feb (index 1): 12000 recurring + 5000 goal-by-category + 2000 goal-by-budget.
	if row[1] != 12000+5000+2000 {
		t.Errorf("b1 Feb = %d, want %d", row[1], 12000+5000+2000)
	}
	// Jan (index 0) is before fromMonth — nothing projected.
	if row[0] != 0 {
		t.Errorf("b1 Jan = %d, want 0", row[0])
	}
}

func TestProjectPastYearEmpty(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "r1", Amount: usd(-12000), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC), CategoryID: "rent"},
	}
	p := Project(recs, nil, 2026, 12, "USD", usdRates) // fromMonth past the year end
	if len(p.Recurring) != 0 {
		t.Fatalf("expected empty projection when fromMonth > 11, got %v", p.Recurring)
	}
}
