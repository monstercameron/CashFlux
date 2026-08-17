package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func expenseOn(cat string, date time.Time, minor int64) domain.Transaction {
	return domain.Transaction{
		ID:         "t-" + cat + "-" + date.Format("20060102"),
		CategoryID: cat,
		Date:       date,
		Amount:     money.New(-minor, "USD"),
	}
}

// fillAmounts indexes a QuickFills result by key.
func fillAmounts(fills []QuickFill) map[string]int64 {
	got := map[string]int64{}
	for _, f := range fills {
		got[f.Key] = f.Amount.Amount
	}
	return got
}

func TestQuickFills(t *testing.T) {
	now := time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{
		ID:         "b1",
		CategoryID: "groceries",
		Period:     domain.PeriodMonthly,
		Limit:      money.New(40000, "USD"),
	}
	// March: 300, Feb: 500, Jan: 400. Older months empty.
	txns := []domain.Transaction{
		expenseOn("groceries", time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC), 30000),
		expenseOn("groceries", time.Date(2026, time.February, 8, 0, 0, 0, 0, time.UTC), 50000),
		expenseOn("groceries", time.Date(2026, time.January, 20, 0, 0, 0, 0, time.UTC), 40000),
	}
	in := QuickFillInput{Now: now, WeekStart: time.Sunday, Rates: currency.Rates{Base: "USD"}}
	fills, win := QuickFills(budget, txns, in)

	got := fillAmounts(fills)
	if got[QuickFillLastPeriod] != 30000 {
		t.Errorf("last period spend = %d, want 30000", got[QuickFillLastPeriod])
	}
	if got[QuickFillAvg3] != 40000 { // (300+500+400)/3
		t.Errorf("avg3 = %d, want 40000", got[QuickFillAvg3])
	}
	if got[QuickFillAvg6] != 20000 { // (300+500+400+0+0+0)/6
		t.Errorf("avg6 = %d, want 20000", got[QuickFillAvg6])
	}
	if got[QuickFillPriorLimit] != 40000 {
		t.Errorf("prior limit = %d, want 40000", got[QuickFillPriorLimit])
	}
	if _, ok := got[QuickFillUnderfunded]; ok {
		t.Error("underfunded chip should be absent when HasUnderfunded is false")
	}
	// The window names the six whole months read: Oct 2025 through Mar 2026.
	if win.Periods != 6 {
		t.Errorf("window periods = %d, want 6", win.Periods)
	}
	if wantFrom := time.Date(2025, time.October, 1, 0, 0, 0, 0, time.UTC); !win.From.Equal(wantFrom) {
		t.Errorf("window from = %v, want %v", win.From, wantFrom)
	}
	if wantTo := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC); !win.To.Equal(wantTo) {
		t.Errorf("window to = %v, want %v", win.To, wantTo)
	}
	if win.Period != domain.PeriodMonthly {
		t.Errorf("window period = %q, want monthly", win.Period)
	}
}

// C667: the suggestions must count the SAME categories the card's spending bar
// counts — the budget's tracked categories AND their descendants. A parent-
// category budget whose spend all sits in sub-categories used to be offered a
// history of ~zero beside a card reading four figures.
func TestQuickFillsCountsRollupCategories(t *testing.T) {
	now := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{
		ID:         "b1",
		CategoryID: "transportation",
		Period:     domain.PeriodMonthly,
		Limit:      money.New(130000, "USD"),
	}
	// Every charge lands in sub-categories of Transportation, never on the parent.
	txns := []domain.Transaction{
		expenseOn("gas", time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC), 90000),
		expenseOn("rideshare", time.Date(2026, time.July, 19, 0, 0, 0, 0, time.UTC), 30000),
		expenseOn("gas", time.Date(2026, time.June, 11, 0, 0, 0, 0, time.UTC), 60000),
	}
	in := QuickFillInput{Now: now, WeekStart: time.Sunday, Rates: currency.Rates{Base: "USD"}}

	bare, _ := QuickFills(budget, txns, in)
	if got := fillAmounts(bare)[QuickFillLastPeriod]; got != 0 {
		t.Fatalf("without the rollup set, last period spend = %d, want 0", got)
	}

	in.Covers = map[string]bool{"gas": true, "rideshare": true}
	rolled, _ := QuickFills(budget, txns, in)
	got := fillAmounts(rolled)
	if got[QuickFillLastPeriod] != 120000 { // July: 900 + 300
		t.Errorf("last period spend = %d, want 120000", got[QuickFillLastPeriod])
	}
	if got[QuickFillAvg3] != 60000 { // (1200 + 600 + 0)/3
		t.Errorf("avg3 = %d, want 60000", got[QuickFillAvg3])
	}
}

// C667: a non-monthly budget averages its OWN periods. A weekly budget used to be
// handed whole calendar months under a chip labelled "last period".
func TestQuickFillsUsesBudgetCadence(t *testing.T) {
	// Sunday-start weeks; now is Wed 2026-04-15, so the previous whole week is
	// Sun 2026-04-05 .. Sat 2026-04-11.
	now := time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{
		ID:         "b1",
		CategoryID: "coffee",
		Period:     domain.PeriodWeekly,
		Limit:      money.New(5000, "USD"),
	}
	txns := []domain.Transaction{
		expenseOn("coffee", time.Date(2026, time.April, 7, 0, 0, 0, 0, time.UTC), 4000),  // previous week
		expenseOn("coffee", time.Date(2026, time.April, 14, 0, 0, 0, 0, time.UTC), 9900), // current week — excluded
		expenseOn("coffee", time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC), 2000), // two weeks back
	}
	in := QuickFillInput{Now: now, WeekStart: time.Sunday, Rates: currency.Rates{Base: "USD"}}
	fills, win := QuickFills(budget, txns, in)

	got := fillAmounts(fills)
	if got[QuickFillLastPeriod] != 4000 {
		t.Errorf("last week spend = %d, want 4000", got[QuickFillLastPeriod])
	}
	if got[QuickFillAvg3] != 2000 { // (4000 + 2000 + 0)/3
		t.Errorf("avg3 = %d, want 2000", got[QuickFillAvg3])
	}
	if win.Period != domain.PeriodWeekly {
		t.Errorf("window period = %q, want weekly", win.Period)
	}
	// Six whole weeks back from Sun 2026-04-05 starts Sun 2026-03-01.
	if wantFrom := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC); !win.From.Equal(wantFrom) {
		t.Errorf("window from = %v, want %v", win.From, wantFrom)
	}
	if wantTo := time.Date(2026, time.April, 12, 0, 0, 0, 0, time.UTC); !win.To.Equal(wantTo) {
		t.Errorf("window to = %v, want %v", win.To, wantTo)
	}
}

// C667: every cadence walks its OWN grid. Monthly and weekly are covered above;
// these pin the three that step irregularly, where a hand-rolled walk-back is
// most likely to drift — semimonthly's uneven halves, quarterly's three-month
// stride, and yearly's rollover.
func TestQuickFillsWindowPerCadence(t *testing.T) {
	cases := []struct {
		name       string
		period     domain.Period
		now        time.Time
		wantFrom   time.Time
		wantTo     time.Time // exclusive
		wantPeriod domain.Period
	}{{
		// 20 Aug 2026 sits in the second half of August, so the six whole halves
		// behind it run from the first half of May to the first half of August.
		name: "semimonthly", period: domain.PeriodSemimonthly,
		now:      time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC),
		wantFrom: time.Date(2026, time.May, 16, 0, 0, 0, 0, time.UTC),
		wantTo:   time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC),
	}, {
		// Q3 2026 contains 20 Aug, so the six whole quarters behind it start in Q1 2025.
		name: "quarterly", period: domain.PeriodQuarterly,
		now:      time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC),
		wantFrom: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
		wantTo:   time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
	}, {
		// Six whole years behind 2026 start in 2020 and end at the close of 2025.
		name: "yearly", period: domain.PeriodYearly,
		now:      time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC),
		wantFrom: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
		wantTo:   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			budget := domain.Budget{ID: "b1", CategoryID: "groceries", Period: c.period, Limit: money.New(40000, "USD")}
			in := QuickFillInput{Now: c.now, WeekStart: time.Sunday, Rates: currency.Rates{Base: "USD"}}
			_, win := QuickFills(budget, nil, in)
			if win.Periods != 6 {
				t.Fatalf("window periods = %d, want 6", win.Periods)
			}
			if !win.From.Equal(c.wantFrom) {
				t.Errorf("window from = %v, want %v", win.From, c.wantFrom)
			}
			if !win.To.Equal(c.wantTo) {
				t.Errorf("window to = %v, want %v", win.To, c.wantTo)
			}
			if win.Period != c.period {
				t.Errorf("window period = %q, want %q", win.Period, c.period)
			}
		})
	}
}

// C667: a biweekly budget snaps to the household's real payday, exactly as the
// cards do — the pay-cycle anchor must reach the history walk, or the chips
// average a fortnight grid the bar never uses.
func TestQuickFillsHonoursPayCycleAnchor(t *testing.T) {
	budget := domain.Budget{ID: "b1", CategoryID: "groceries", Period: domain.PeriodBiweekly, Limit: money.New(40000, "USD")}
	now := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	base := QuickFillInput{Now: now, WeekStart: time.Sunday, Rates: currency.Rates{Base: "USD"}}
	_, plain := QuickFills(budget, nil, base)

	shifted := base
	shifted.PayAnchor = time.Date(2026, time.August, 7, 0, 0, 0, 0, time.UTC)
	_, anchored := QuickFills(budget, nil, shifted)

	if plain.From.Equal(anchored.From) && plain.To.Equal(anchored.To) {
		t.Fatalf("the pay-cycle anchor did not move the fortnight grid: both windows are %v – %v", plain.From, plain.To)
	}
	// Every boundary must land on the anchored grid: a whole number of fortnights
	// from the payday itself.
	for _, boundary := range []time.Time{anchored.From, anchored.To} {
		if days := int(boundary.Sub(shifted.PayAnchor).Hours() / 24); days%14 != 0 {
			t.Errorf("window boundary %v is %d days off the payday — not on the fortnight grid", boundary, days)
		}
	}
}

// C667: each suggestion says whether it is spending or a plan, so the prior limit
// can never be read as last period's spend.
func TestQuickFillsLabelsKinds(t *testing.T) {
	now := time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "b1", CategoryID: "groceries", Period: domain.PeriodMonthly, Limit: money.New(40000, "USD")}
	in := QuickFillInput{
		Now: now, WeekStart: time.Sunday, Rates: currency.Rates{Base: "USD"},
		Underfunded: money.New(6000, "USD"), HasUnderfunded: true,
	}
	want := map[string]QuickFillKind{
		QuickFillLastPeriod:  QuickFillSpend,
		QuickFillAvg3:        QuickFillSpend,
		QuickFillAvg6:        QuickFillSpend,
		QuickFillPriorLimit:  QuickFillLimit,
		QuickFillUnderfunded: QuickFillTarget,
	}
	fills, _ := QuickFills(budget, nil, in)
	if len(fills) != len(want) {
		t.Fatalf("got %d fills, want %d", len(fills), len(want))
	}
	for _, f := range fills {
		if f.Kind != want[f.Key] {
			t.Errorf("%s kind = %q, want %q", f.Key, f.Kind, want[f.Key])
		}
	}
}

func TestQuickFillsPriorLimitBoost(t *testing.T) {
	now := time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)
	// Boost the previous period (March).
	budget := domain.Budget{
		ID:           "b1",
		CategoryID:   "groceries",
		Period:       domain.PeriodMonthly,
		Limit:        money.New(40000, "USD"),
		PeriodBoosts: map[string]int64{"2026-03-01": 5000},
	}
	in := QuickFillInput{Now: now, WeekStart: time.Sunday, Rates: currency.Rates{Base: "USD"}}
	fills, _ := QuickFills(budget, nil, in)
	if got := fillAmounts(fills)[QuickFillPriorLimit]; got != 45000 {
		t.Errorf("prior limit with boost = %d, want 45000", got)
	}
}

func TestQuickFillsUnderfunded(t *testing.T) {
	now := time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "b1", CategoryID: "groceries", Period: domain.PeriodMonthly, Limit: money.New(40000, "USD")}
	in := QuickFillInput{
		Now:            now,
		WeekStart:      time.Sunday,
		Rates:          currency.Rates{Base: "USD"},
		Underfunded:    money.New(6000, "USD"),
		HasUnderfunded: true,
	}
	found := false
	fills, _ := QuickFills(budget, nil, in)
	for _, f := range fills {
		if f.Key == QuickFillUnderfunded {
			found = true
			if f.Amount.Amount != 6000 {
				t.Errorf("underfunded = %d, want 6000", f.Amount.Amount)
			}
		}
	}
	if !found {
		t.Error("underfunded chip missing when HasUnderfunded is true")
	}
}
