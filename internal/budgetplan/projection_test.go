// SPDX-License-Identifier: MIT

package budgetplan

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smoothing"
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

func TestProjectMonthEndRecurringKeepsTwelveOccurrences(t *testing.T) {
	r := domain.Recurring{
		ID: "month-end", Amount: usd(-100), Cadence: domain.CadenceMonthly,
		NextDue: time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC), CategoryID: "bills",
	}
	p := Project([]domain.Recurring{r}, nil, 2026, 0, "USD", usdRates)
	for month, got := range p.Recurring["bills"] {
		if got != 100 {
			t.Errorf("month %d amount = %d, want 100", month, got)
		}
	}
	if got := sumMonths(p.Recurring["bills"]); got != 1200 {
		t.Fatalf("annual amount = %d, want 1200 across all 12 months", got)
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
		if got := p.GoalsByBudget[b]; sumMonths(got) != 11*1500 || got[0] != 0 || got[11] != 1500 {
			t.Errorf("GoalsByBudget[%s] = %v", b, got)
		}
	}
}

func TestProjectManagedSmoothingGoalDoesNotDoubleCountProjectedBill(t *testing.T) {
	rec := domain.Recurring{
		ID: "annual-insurance", Amount: usd(-12000), Cadence: domain.CadenceYearly,
		NextDue:    time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
		CategoryID: "insurance", SmoothIntoBudgets: true,
	}
	goal := domain.Goal{
		ID: "managed-insurance", CategoryID: "insurance", MonthlyContribution: usd(1000),
		Custom: map[string]any{smoothing.GoalCustomKey: rec.ID},
	}
	p := Project([]domain.Recurring{rec}, []domain.Goal{goal}, 2026, 0, "USD", usdRates)
	if got := p.GoalsByCategory["insurance"]; got != (MonthAmounts{}) {
		t.Fatalf("managed smoothing goal was projected with its bill: %v", got)
	}
	if got := p.Recurring["insurance"][11]; got != 12000 {
		t.Fatalf("December recurring = %d, want 12000", got)
	}
}

func TestProjectGoalBudgetSharesPreserveWholeAssignment(t *testing.T) {
	goal := domain.Goal{ID: "shared", MonthlyContribution: usd(1001), BudgetIDs: []string{"b2", "b1", "b1"}}
	p := Project(nil, []domain.Goal{goal}, 2026, 0, "USD", usdRates)
	if got := p.GoalsByBudget["b1"][0]; got != 501 {
		t.Errorf("first budget share = %d, want 501 remainder assigned by ID", got)
	}
	if got := p.GoalsByBudget["b2"][0]; got != 500 {
		t.Errorf("second budget share = %d, want 500", got)
	}
	total, err := p.TotalUnique([]string{"b1", "b2"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total[0] != 1001 {
		t.Fatalf("unique total = %d, want the full assignment 1001", total[0])
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
	per, err := p.PerBudget([]string{"b1"}, covers)
	if err != nil {
		t.Fatal(err)
	}
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

func TestProjectSkipsPausedRecurring(t *testing.T) {
	r := domain.Recurring{ID: "paused", Amount: usd(-9000), Cadence: domain.CadenceMonthly,
		NextDue: time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC), CategoryID: "rent", Paused: true}
	p := Project([]domain.Recurring{r}, nil, 2026, 0, "USD", usdRates)
	if got := sumMonths(p.Recurring["rent"]); got != 0 {
		t.Fatalf("paused bill projection = %d, want 0", got)
	}
}

func TestScaleMonthlyIncomeAcrossWindow(t *testing.T) {
	start := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC)
	got, err := ScaleMonthlyIncome(usd(500000), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 1500000 {
		t.Fatalf("three-month income = %d, want 1500000", got.Amount)
	}
}

func TestScaleMonthlyIncomeChecksOverflow(t *testing.T) {
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, err := ScaleMonthlyIncome(usd(int64(^uint64(0)>>1)), start, end); err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestTotalUniqueCountsOverlappingRecurringCategoryOnce(t *testing.T) {
	p := Projection{Recurring: map[string]MonthAmounts{"rent": {0: 12000}}}
	covers := map[string]map[string]bool{"a": {"rent": true}, "b": {"rent": true}}
	got, err := p.TotalUnique([]string{"a", "b"}, covers)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != 12000 {
		t.Fatalf("unique total = %d, want 12000", got[0])
	}
}

func TestProjectReportsProjectionOverflow(t *testing.T) {
	due := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	recs := []domain.Recurring{
		{ID: "a", Amount: money.New(-math.MaxInt64, "USD"), Cadence: domain.CadenceMonthly, NextDue: due, CategoryID: "rent"},
		{ID: "b", Amount: money.New(-math.MaxInt64, "USD"), Cadence: domain.CadenceMonthly, NextDue: due, CategoryID: "rent"},
	}
	p := Project(recs, nil, 2026, 0, "USD", usdRates)
	if p.Error == nil {
		t.Fatal("overflowing recurring projection must be incomplete")
	}
}
