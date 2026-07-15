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
		ID:         "t-" + date.Format("20060102"),
		CategoryID: cat,
		Date:       date,
		Amount:     money.New(-minor, "USD"),
	}
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
	fills := QuickFills(budget, txns, in)

	got := map[string]int64{}
	for _, f := range fills {
		got[f.Key] = f.Amount.Amount
	}
	if got[QuickFillLastMonth] != 30000 {
		t.Errorf("last month = %d, want 30000", got[QuickFillLastMonth])
	}
	if got[QuickFillAvg3] != 40000 { // (300+500+400)/3
		t.Errorf("avg3 = %d, want 40000", got[QuickFillAvg3])
	}
	if got[QuickFillAvg6] != 20000 { // (300+500+400+0+0+0)/6
		t.Errorf("avg6 = %d, want 20000", got[QuickFillAvg6])
	}
	if got[QuickFillLastPeriod] != 40000 {
		t.Errorf("last period = %d, want 40000", got[QuickFillLastPeriod])
	}
	if _, ok := got[QuickFillUnderfunded]; ok {
		t.Error("underfunded chip should be absent when HasUnderfunded is false")
	}
}

func TestQuickFillsLastPeriodBoost(t *testing.T) {
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
	for _, f := range QuickFills(budget, nil, in) {
		if f.Key == QuickFillLastPeriod && f.Amount.Amount != 45000 {
			t.Errorf("last period with boost = %d, want 45000", f.Amount.Amount)
		}
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
	for _, f := range QuickFills(budget, nil, in) {
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
