package budgetview

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAnnualBuildAndProjection(t *testing.T) {
	now := day(2026, 10, 19)
	b := budget("b", 1200)
	cats := []domain.Category{{ID: "c", Name: "Food", Kind: domain.KindExpense}}
	build := BuildAnnual([]domain.Budget{b}, nil, cats, 2026, usd(), time.Monday, now)
	if build.Err != nil || build.Grid.GrandPlan.Amount != 14400 || !build.Covers["b"]["c"] {
		t.Fatal(build)
	}
	rec := domain.Recurring{ID: "r", CategoryID: "c", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, NextDue: day(2026, 11, 1)}
	projection := ProjectAnnual(build.Grid, build.Covers, []domain.Recurring{rec}, nil, now, usd())
	if projection.Err != nil || projection.FromMonth != 10 || projection.PerBudget["b"][10] != 100 || projection.Totals[10] != 100 || projection.Totals[9] != 0 {
		t.Fatal(projection)
	}
	// Overlapping rows retain their individual projection, but totals count it once.
	grid := build.Grid
	copyRow := grid.Rows[0]
	copyRow.BudgetID = "other"
	grid.Rows = append(grid.Rows, copyRow)
	covers := map[string]map[string]bool{"b": {"c": true}, "other": {"c": true}}
	projection = ProjectAnnual(grid, covers, []domain.Recurring{rec}, nil, now, usd())
	if projection.Err != nil || projection.Totals[10] != 100 || projection.PerBudget["other"][10] != 100 {
		t.Fatal(projection)
	}
	rec.Amount = money.New(math.MinInt64, "USD")
	if got := ProjectAnnual(grid, covers, []domain.Recurring{rec}, nil, now, usd()); got.Err == nil {
		t.Fatal("projection overflow")
	}
	for _, year := range []int{2025, 2026, 2027} {
		month := CurrentMonth(now, year)
		wantMonth, wantFuture := -1, 12
		if year == 2026 {
			wantMonth, wantFuture = 9, 10
		}
		if year == 2027 {
			wantFuture = 0
		}
		if month != wantMonth || FutureMonth(budgeting.AnnualGrid{Year: year, CurrentMonth: month}, now) != wantFuture {
			t.Fatal(year)
		}
	}
}

func TestAnnualScenarios(t *testing.T) {
	grid := budgeting.AnnualGrid{Currency: "USD", Rows: []budgeting.AnnualGridRow{{BudgetID: "first"}, {BudgetID: "last"}, {BudgetID: "save", Saving: true}, {BudgetID: "error", Error: errors.New("bad")}}}
	for m := range 12 {
		grid.Rows[0].Cells[m].Plan = money.New(100, "USD")
		grid.Rows[1].Cells[m].Plan = money.New(200, "USD")
		grid.MonthPlanTotals[m] = money.New(300, "USD")
	}
	for _, tc := range []struct {
		delta       int64
		count       int
		first, last int64
	}{{0, 0, 0, 0}, {100, 0, 0, 0}, {-150, 12, 0, 150}, {-300, 24, 100, 200}, {math.MinInt64, 24, 100, 200}} {
		got, n := AnnualScenario(grid, tc.delta)
		if n != tc.count || got["first"][0] != tc.first || got["last"][11] != tc.last {
			t.Fatal(tc, got, n)
		}
	}
	if got, n := AnnualScenario(budgeting.AnnualGrid{}, 0); len(got) != 0 || n != 0 {
		t.Fatal(got, n)
	}
	for _, tc := range []struct{ current, step, want int64 }{{0, 10000, 10000}, {0, -10000, -10000}, {math.MaxInt64, 10000, math.MaxInt64}, {math.MinInt64, -10000, math.MinInt64}} {
		if got := StepScenario(tc.current, tc.step); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestAnnualFormattingAndDates(t *testing.T) {
	for _, tc := range []struct {
		i, current, future int
		want               string
	}{{12, 12, 0, " is-total"}, {9, 9, 10, " is-current"}, {10, 9, 10, " is-future"}, {0, 9, 10, ""}} {
		if got := GridColClass(tc.i, tc.current, tc.future); got != tc.want {
			t.Fatal(got)
		}
	}
	for _, tc := range []struct {
		win         string
		first, last int
	}{{"all", 0, 11}, {"h1", 0, 5}, {"h2", 6, 11}, {"unknown", 0, 11}} {
		a, b := WindowRange(tc.win)
		if a != tc.first || b != tc.last {
			t.Fatal(a, b)
		}
	}
	for _, tc := range []struct {
		year, month int
		from, to    string
	}{{2024, 1, "2024-02-01", "2024-03-01"}, {2026, 11, "2026-12-01", "2027-01-01"}} {
		a, b := MonthDates(tc.year, tc.month)
		if a != tc.from || b != tc.to {
			t.Fatal(a, b)
		}
	}
	if MonthLabel(-1, 2026, tr) != "2026" || MonthLabel(12, 2026, tr) != "2026" || MonthLabel(0, 2026, tr) != "budgets.month.january 2026" {
		t.Fatal("month labels")
	}
	for _, tc := range []struct {
		budgets []domain.Budget
		rates   currency.Rates
		want    string
	}{
		{nil, usd(), "$0.00"},
		{[]domain.Budget{budget("b", 100)}, usd(), "$12.00"},
		{[]domain.Budget{func() domain.Budget { b := budget("x", 1); b.Limit.Currency = "EUR"; return b }()}, usd(), "$0.00 bud2.excludesOneBudget"},
		{[]domain.Budget{func() domain.Budget { b := budget("x", 1); b.Limit.Currency = "EUR"; return b }(), func() domain.Budget { b := budget("y", 1); b.Limit.Currency = "GBP"; return b }()}, usd(), "$0.00 bud2.excludesManyBudgets2"},
	} {
		if got := AnnualPlanLabel(tc.budgets, tc.rates, tr); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
	b := budget("b", 20000)
	b.Period = domain.PeriodSemimonthly
	if AnnualPlanTotal([]domain.Budget{b}, "USD") != 480000 || AnnualPlanTotal([]domain.Budget{b}, "USD", usd()) != 480000 {
		t.Fatal("annual cadence")
	}
	if got := BudgetCategories([]domain.Budget{b}); !reflect.DeepEqual(got["b"], []string{"c"}) {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		current, future, over             bool
		projected, short                  int64
		class, primary, titleKey, ariaKey string
	}{
		{false, false, false, 0, 0, "budget-annualgrid-td", "$1.00", "budgets.gridCellAria", "budgets.gridCellAria"},
		{true, false, true, 0, 0, "budget-annualgrid-td is-current is-over", "$1.00", "budgets.gridCellAria", "budgets.gridCellAria"},
		{false, true, true, 250, 0, "budget-annualgrid-td is-future", "$2.50", "budgets.gridProjectedTitle", "budgets.gridCellAriaProjected"},
		{false, true, false, -1, 20, "budget-annualgrid-td is-future is-underfunded", "$1.00", "budgets.scenarioUnderTitle", "budgets.gridCellAria"},
		{true, true, true, 250, 20, "budget-annualgrid-td is-current is-future is-underfunded", "$2.50", "budgets.scenarioUnderTitle", "budgets.gridCellAriaProjected"},
	} {
		got := PresentAnnualCell(budgeting.AnnualGridCell{Actual: money.New(100, "USD"), Plan: money.New(200, "USD"), Over: tc.over}, tc.current, tc.future, tc.projected, tc.short, "USD", "Food", "January 2026", tr)
		if got.Class != tc.class || got.Primary != tc.primary || !strings.HasPrefix(got.Title, tc.titleKey) || !strings.HasPrefix(got.Aria, tc.ariaKey) {
			t.Fatal(got, tc)
		}
		if tc.short > 0 && !strings.Contains(got.Aria, "budgets.gridCellAriaShort") {
			t.Fatal(got)
		}
	}
}

func FuzzAnnualScenarioReconciles(f *testing.F) {
	f.Add(uint32(100), uint32(200), int32(-150))
	f.Fuzz(func(t *testing.T, a, b uint32, delta int32) {
		grid := budgeting.AnnualGrid{Rows: []budgeting.AnnualGridRow{{BudgetID: "a"}, {BudgetID: "b"}}}
		for m := range 12 {
			grid.Rows[0].Cells[m].Plan = money.New(int64(a), "USD")
			grid.Rows[1].Cells[m].Plan = money.New(int64(b), "USD")
			grid.MonthPlanTotals[m] = money.New(int64(a)+int64(b), "USD")
		}
		short, _ := AnnualScenario(grid, int64(delta))
		income := max(int64(0), int64(a)+int64(b)+int64(delta))
		want := max(int64(0), int64(a)+int64(b)-income)
		for m := range 12 {
			if short["a"][m]+short["b"][m] != want {
				t.Fatal(short, want)
			}
		}
	})
}

func TestAnnualMixedCurrenciesAndFailures(t *testing.T) {
	now := day(2026, 10, 19)
	euro := budget("eur", 100)
	euro.Limit.Currency = "EUR"
	for _, tc := range []struct {
		name             string
		budgets          []domain.Budget
		rates            currency.Rates
		total            int64
		rowErr, buildErr bool
	}{
		{"empty", nil, usd(), 0, false, false},
		{"zero", []domain.Budget{budget("zero", 0)}, usd(), 0, false, false},
		{"mixed", []domain.Budget{budget("usd", 100), euro}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": .5}}, 1800, false, false},
		{"missing FX", []domain.Budget{euro}, usd(), 0, true, false},
		{"row overflow", []domain.Budget{budget("huge", math.MaxInt64)}, usd(), 0, true, false},
		{"total overflow", []domain.Budget{budget("first", math.MaxInt64/16), budget("second", math.MaxInt64/16)}, usd(), 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildAnnual(tc.budgets, nil, nil, 2026, tc.rates, time.Monday, now)
			if (got.Err != nil) != tc.buildErr {
				t.Fatal(got.Err)
			}
			if tc.buildErr {
				return
			}
			if got.Grid.GrandPlan.Amount != tc.total {
				t.Fatal(got.Grid.GrandPlan)
			}
			if len(got.Grid.Rows) > 0 && (got.Grid.Rows[0].Error != nil) != tc.rowErr {
				t.Fatal(got.Grid.Rows)
			}
		})
	}
	grid := budgeting.AnnualGrid{Year: 2027, CurrentMonth: -1, Currency: "USD", Rows: []budgeting.AnnualGridRow{{BudgetID: "a"}, {BudgetID: "b"}}}
	covers := map[string]map[string]bool{"a": {"a": true}, "b": {"b": true}}
	recs := []domain.Recurring{}
	for _, id := range []string{"a", "b"} {
		recs = append(recs, domain.Recurring{ID: id, CategoryID: id, Amount: money.New(-math.MaxInt64/2-1, "USD"), Cadence: domain.CadenceYearly, NextDue: day(2027, 1, 1)})
	}
	if got := ProjectAnnual(grid, covers, recs, nil, now, usd()); !errors.Is(got.Err, money.ErrOverflow) {
		t.Fatal(got)
	}
	recs[0].Amount = money.New(-100, "EUR")
	if got := ProjectAnnual(grid, covers, recs[:1], nil, now, usd()); got.Err != nil || got.Totals[0] != 0 {
		t.Fatal(got)
	}
}

func TestStepYear(t *testing.T) {
	for _, tc := range []struct{ year, step, want int }{{2026, 0, 2026}, {2026, -1, 2025}, {2026, 1, 2027}, {0, -1, -1}, {math.MaxInt, 1, math.MinInt}} {
		if got := StepYear(tc.year, tc.step); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
