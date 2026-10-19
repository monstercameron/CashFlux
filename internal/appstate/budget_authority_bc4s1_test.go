// SPDX-License-Identifier: MIT

package appstate

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func bc4Budget(t *testing.T, a *App, id string) domain.Budget {
	t.Helper()
	for _, b := range a.Budgets() {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("budget %q missing", id)
	return domain.Budget{}
}

func bc4Cap(t *testing.T, a *App, id, date string, want int64) {
	t.Helper()
	in, err := a.BudgetViewInput(moneyDate(date))
	if err != nil {
		t.Fatal(err)
	}
	v := budgeting.EvaluateViewBudget(bc4Budget(t, a, id), in)
	if !v.Valid || v.Error != nil || v.Status.Limit.Amount != want {
		t.Fatalf("%s on %s: cap=%v valid=%v err=%v, want %d", id, date, v.Status.Limit, v.Valid, v.Error, want)
	}
}

func TestBC4LimitWriterInvariant(t *testing.T) {
	writers := map[string]func(*App, domain.Budget) error{
		"PutBudget-inline-enter-save-suggestion-track-raise": (*App).PutBudget,
		"PutBudgetWithHistory":                               (*App).PutBudgetWithHistory,
		"PutBudgetAt-edit-topup-permanent":                   func(a *App, b domain.Budget) error { return a.PutBudgetAt(b, a.clock()) },
		"PutBudgetsAtomic-adjust-all-track":                  func(a *App, b domain.Budget) error { return a.PutBudgetsAtomic([]domain.Budget{b}) },
		"PutBudgetsAtomicAt":                                 func(a *App, b domain.Budget) error { return a.PutBudgetsAtomicAt([]domain.Budget{b}, a.clock()) },
		"price-creep":                                        func(a *App, _ domain.Budget) error { return a.RaiseBudgetForCreep("r", 11000) },
	}
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			a := newApp(t, false)
			a.now = func() time.Time { return moneyDate("2026-09-19") }
			b := sharedBudget("b", "Food", "food", 30000)
			if err := a.PutBudgetWithHistory(b); err != nil {
				t.Fatal(err)
			}
			if err := a.PutRecurring(domain.Recurring{ID: "r", Label: "Food bill", CategoryID: "food", AccountID: "a", Amount: money.New(-1000, "USD"), Cadence: domain.CadenceMonthly, NextDue: moneyDate("2026-10-20")}); err != nil {
				t.Fatal(err)
			}
			a.now = func() time.Time { return moneyDate("2026-10-19") }
			b = bc4Budget(t, a, "b")
			b.Limit.Amount = 40000
			if err := write(a, b); err != nil {
				t.Fatal(err)
			}
			bc4Cap(t, a, "b", "2026-09-19", 30000)
			bc4Cap(t, a, "b", "2026-10-19", 40000)
			bc4Cap(t, a, "b", "2027-03-19", 40000)
		})
	}
}

func TestBC4PastPermanentEditReplacesLaterHistory(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	b := sharedBudget("b", "Food", "food", 60000)
	b.LimitHistory = map[string]int64{"2026-01-01": 50000, "2026-10-01": 60000, "2026-12-01": 70000}
	if err := a.store.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	b.Limit.Amount = 66000
	if err := a.PutBudgetAt(b, moneyDate("2026-08-19")); err != nil {
		t.Fatal(err)
	}
	for date, want := range map[string]int64{"2026-07-19": 50000, "2026-08-19": 66000, "2026-10-19": 66000, "2027-01-19": 66000} {
		bc4Cap(t, a, "b", date, want)
	}
	want := map[string]int64{"2026-01-01": 50000, "2026-08-01": 66000}
	if got := bc4Budget(t, a, "b").LimitHistory; !reflect.DeepEqual(got, want) {
		t.Fatalf("history=%v, want %v", got, want)
	}
	if !reflect.DeepEqual(b.LimitHistory, map[string]int64{"2026-01-01": 50000, "2026-10-01": 60000, "2026-12-01": 70000}) {
		t.Fatal("mutated caller's history")
	}
}

func TestBC4PeriodChangeResetsFundingGrid(t *testing.T) {
	for _, from := range []domain.Period{domain.PeriodWeekly, domain.PeriodMonthly, domain.PeriodYearly} {
		for _, to := range []domain.Period{domain.PeriodWeekly, domain.PeriodMonthly, domain.PeriodYearly} {
			if from == to {
				continue
			}
			for _, mode := range []string{"simple", "rollover", "envelope", "envelope-rollover"} {
				t.Run(string(from)+"-"+string(to)+"-"+mode, func(t *testing.T) {
					a := newApp(t, false)
					if err := a.SetSettingKV("cashflux:prefs", `{"weekStart":"monday"}`); err != nil {
						t.Fatal(err)
					}
					a.now = func() time.Time { return moneyDate("2026-10-19") }
					b := sharedBudget("b", "Food", "food", 10000)
					b.Period = from
					b.Rollover = mode == "rollover" || mode == "envelope-rollover"
					if mode == "envelope" || mode == "envelope-rollover" {
						b.Methodology = "envelope"
					}
					b.RolloverStart = moneyDate("2026-01-01")
					b.LimitHistory = map[string]int64{"2026-01-01": 10000}
					b.PeriodBoosts = map[string]int64{"2026-10-01": 90000}
					if err := a.store.PutBudget(b); err != nil {
						t.Fatal(err)
					}
					b.Period, b.Limit.Amount = to, 45000
					if err := a.PutBudgetAt(b, a.clock()); err != nil {
						t.Fatal(err)
					}
					got := bc4Budget(t, a, "b")
					start, _ := budgeting.PeriodRangeAnchored(to, a.clock(), time.Monday, time.Time{})
					if !got.RolloverStart.Equal(start) || !reflect.DeepEqual(got.LimitHistory, map[string]int64{start.Format(time.DateOnly): 45000}) || len(got.PeriodBoosts) != 0 {
						t.Fatalf("grid not reset: %+v", got)
					}
					bc4Cap(t, a, "b", "2026-10-19", 45000)
				})
			}
		}
	}
}

func TestBC4CoverUsesAtomicPeriodBoosts(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-09-19") }
	for _, b := range []domain.Budget{sharedBudget("from", "Food", "food", 30000), sharedBudget("to", "Dining", "dining", 10000)} {
		if err := a.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	if err := a.CoverBudget("from", "to", money.New(5000, "USD")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		id        string
		base, cap int64
	}{{"from", 30000, 25000}, {"to", 10000, 15000}} {
		if got := bc4Budget(t, a, c.id).Limit.Amount; got != c.base {
			t.Errorf("%s standing limit=%d", c.id, got)
		}
		bc4Cap(t, a, c.id, "2026-09-19", c.base)
		bc4Cap(t, a, c.id, "2026-10-19", c.cap)
		bc4Cap(t, a, c.id, "2026-11-19", c.base)
	}
	if got := bc4Budget(t, a, "to").CoveredAt; !got.Equal(a.clock()) {
		t.Fatalf("covered at %v", got)
	}
}

func TestBC4RecurringCoverDoesNotCompound(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	from := sharedBudget("from", "Food", "food", 30000)
	to := sharedBudget("to", "Dining", "dining", 10000)
	to.RecurringCover = &domain.RecurringCover{AmountMinor: 5000, Sources: []domain.CoverShare{{BudgetID: "from", Weight: 1}}}
	for _, b := range []domain.Budget{from, to} {
		if err := a.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	shares := map[string]money.Money{"from": money.New(5000, "USD")}
	for _, date := range []string{"2026-10-19", "2026-10-20", "2026-11-19", "2026-11-20"} {
		if err := a.ApplyRecurringBudgetCoverAt("to", shares, moneyDate(date)); err != nil {
			t.Fatal(err)
		}
		bc4Cap(t, a, "from", date, 25000)
		bc4Cap(t, a, "to", date, 15000)
	}
	bc4Cap(t, a, "from", "2026-12-19", 30000)
	bc4Cap(t, a, "to", "2026-12-19", 10000)
	if got := bc4Budget(t, a, "to"); got.Limit.Amount != 10000 || got.RecurringCover.LastAppliedPeriod != "2026-11-01" {
		t.Fatalf("destination=%+v", got)
	}
}

func TestBC4RecurringCoverFailureLeavesAllLegsAndStampUnchanged(t *testing.T) {
	for _, failure := range []string{"room", "currency", "missing", "self", "overflow", "validation"} {
		t.Run(failure, func(t *testing.T) {
			a := newApp(t, false)
			a.now = func() time.Time { return moneyDate("2026-10-19") }
			from := sharedBudget("from", "Food", "food", 30000)
			other := sharedBudget("other", "Other", "other", 1000)
			to := sharedBudget("to", "Dining", "dining", 10000)
			to.RecurringCover = &domain.RecurringCover{AmountMinor: 5000}
			if failure == "validation" {
				to.Name = ""
			}
			if failure == "overflow" {
				to.PeriodBoosts = map[string]int64{"2026-10-01": math.MaxInt64}
			}
			for _, b := range []domain.Budget{from, other, to} {
				if err := a.store.PutBudget(b); err != nil {
					t.Fatal(err)
				}
			}
			before := a.Budgets()
			shares := map[string]money.Money{"from": money.New(5000, "USD")}
			switch failure {
			case "room":
				shares["other"] = money.New(1000, "USD")
			case "currency":
				shares["other"] = money.New(100, "EUR")
			case "missing":
				shares["missing"] = money.New(100, "USD")
			case "self":
				shares["to"] = money.New(100, "USD")
			}
			if err := a.ApplyRecurringBudgetCoverAt("to", shares, a.clock()); err == nil {
				t.Fatal("failed cover accepted")
			}
			if after := a.Budgets(); !reflect.DeepEqual(after, before) {
				t.Fatalf("partial write: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestBC4CoverUsesEachBudgetGridAndSourceRoom(t *testing.T) {
	a := newApp(t, false)
	if err := a.SetSettingKV("cashflux:prefs", `{"weekStart":"monday"}`); err != nil {
		t.Fatal(err)
	}
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	from := sharedBudget("from", "Food", "food", 30000)
	from.Period = domain.PeriodWeekly
	to := sharedBudget("to", "Holidays", "holiday", 10000)
	to.Period = domain.PeriodYearly
	for _, b := range []domain.Budget{from, to} {
		if err := a.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.CoverBudget("from", "to", money.New(5000, "USD")); err != nil {
		t.Fatal(err)
	}
	if got := bc4Budget(t, a, "from").PeriodBoosts; got["2026-10-19"] != -5000 || len(got) != 1 {
		t.Fatal(got)
	}
	if got := bc4Budget(t, a, "to").PeriodBoosts; got["2026-01-01"] != 5000 || len(got) != 1 {
		t.Fatal(got)
	}
	if err := a.store.PutTransaction(domain.Transaction{ID: "t", CategoryID: "food", Amount: money.New(-24000, "USD"), Date: a.clock()}); err != nil {
		t.Fatal(err)
	}
	if err := a.CoverBudget("from", "to", money.New(1000, "USD")); err == nil {
		t.Fatal("spent source's last available room")
	}
}

func TestBC4HistoryRepairAndMetadataWrites(t *testing.T) {
	for _, repair := range []bool{false, true} {
		t.Run(map[bool]string{false: "metadata", true: "legacy-drift-repair"}[repair], func(t *testing.T) {
			a := newApp(t, false)
			a.now = func() time.Time { return moneyDate("2026-10-19") }
			b := sharedBudget("b", "Food", "food", 40000)
			b.LimitHistory = map[string]int64{"2026-09-01": 30000, "2026-10-01": 40000}
			if repair {
				delete(b.LimitHistory, "2026-10-01")
			}
			if err := a.store.PutBudget(b); err != nil {
				t.Fatal(err)
			}
			b.Notes = "A note"
			b.LimitHistory = map[string]int64{"2026-09-01": 1}
			if err := a.PutBudget(b); err != nil {
				t.Fatal(err)
			}
			bc4Cap(t, a, "b", "2026-09-19", 30000)
			bc4Cap(t, a, "b", "2026-10-19", 40000)
			got := bc4Budget(t, a, "b")
			if got.Notes != "A note" || len(got.LimitHistory) != 2 {
				t.Fatalf("budget=%+v", got)
			}
		})
	}
}

func TestBC4BudgetCurrencyChangeRefused(t *testing.T) {
	a := newApp(t, false)
	b := sharedBudget("b", "Food", "food", 30000)
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	before := a.Budgets()
	b.Limit.Currency = "EUR"
	if err := a.PutBudget(b); err == nil {
		t.Fatal("currency reinterpretation accepted")
	}
	if !reflect.DeepEqual(a.Budgets(), before) {
		t.Fatal("rejected currency change persisted")
	}
}

func TestBC4PermanentEditBeforeOldestHistoryPreservesEarlierLimit(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	b := sharedBudget("b", "Food", "food", 30000)
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	b.Limit.Amount = 40000
	if err := a.PutBudgetAt(b, moneyDate("2026-08-19")); err != nil {
		t.Fatal(err)
	}
	bc4Cap(t, a, "b", "2026-06-19", 30000)
	bc4Cap(t, a, "b", "2026-08-19", 40000)
	bc4Cap(t, a, "b", "2026-10-19", 40000)
}

func TestBC4BudgetWriteTriggerUsesPreparedHistory(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	b := sharedBudget("b", "Food", "food", 30000)
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	if err := a.store.PutTransaction(domain.Transaction{ID: "t", CategoryID: "food", Date: a.clock(), Amount: money.New(-20000, "USD")}); err != nil {
		t.Fatal(err)
	}
	w := workflow.Workflow{ID: "w", Name: "Over", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerBudgetExceeded}, Actions: []workflow.Action{{Kind: workflow.ActionNotify, Message: "Over"}}}
	if err := a.PutWorkflow(w); err != nil {
		t.Fatal(err)
	}
	b.Limit.Amount = 10000
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	if got := len(a.WorkflowRuns()); got != 1 {
		t.Fatalf("new over-state runs=%d want 1", got)
	}
	b = bc4Budget(t, a, "b")
	b.Notes = "Metadata edit"
	if err := a.PutBudgetsAtomic([]domain.Budget{b}); err != nil {
		t.Fatal(err)
	}
	if got := len(a.WorkflowRuns()); got != 1 {
		t.Fatalf("unchanged over-state fired again: %d", got)
	}
}

func TestBC4PriceCreepKeepsBudgetCurrencyAndCadence(t *testing.T) {
	for _, tc := range []struct {
		name, recurringCurrency, budgetCurrency string
		period                                  domain.Period
		rates                                   map[string]float64
		want                                    int64
		fail                                    bool
	}{
		{"weekly", "USD", "USD", domain.PeriodWeekly, nil, 30300, false},
		{"foreign budget", "USD", "EUR", domain.PeriodMonthly, map[string]float64{"EUR": 2}, 30650, false},
		{"missing rate", "USD", "EUR", domain.PeriodMonthly, nil, 30000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			a.now = func() time.Time { return moneyDate("2026-10-19") }
			settings := a.Settings()
			settings.BaseCurrency = "USD"
			settings.FXRates = tc.rates
			if err := a.PutSettings(settings); err != nil {
				t.Fatal(err)
			}
			b := sharedBudget("b", "Food", "food", 30000)
			b.Period = tc.period
			b.Limit.Currency = tc.budgetCurrency
			if err := a.PutBudgetAt(b, moneyDate("2026-09-19")); err != nil {
				t.Fatal(err)
			}
			r := domain.Recurring{ID: "r", Label: "Bill", AccountID: "a", CategoryID: "food", Amount: money.New(-1000, tc.recurringCurrency), Cadence: domain.CadenceMonthly, NextDue: a.clock()}
			if err := a.PutRecurring(r); err != nil {
				t.Fatal(err)
			}
			if err := a.RaiseBudgetForCreep("r", 2300); (err != nil) != tc.fail {
				t.Fatalf("err=%v fail=%v", err, tc.fail)
			}
			got := bc4Budget(t, a, "b")
			if got.Limit != money.New(tc.want, tc.budgetCurrency) {
				t.Fatalf("limit=%v want %d %s", got.Limit, tc.want, tc.budgetCurrency)
			}
			if !tc.fail {
				bc4Cap(t, a, "b", "2026-10-19", tc.want)
				bc4Cap(t, a, "b", "2026-09-19", 30000)
			}
		})
	}
}

func TestBC4RecurringZeroCoverStampsWithoutChangingLimits(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	b := sharedBudget("b", "Food", "food", 30000)
	b.RecurringCover = &domain.RecurringCover{}
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplyRecurringBudgetCoverAt("b", nil, a.clock()); err != nil {
		t.Fatal(err)
	}
	got := bc4Budget(t, a, "b")
	if got.RecurringCover.LastAppliedPeriod != "2026-10-01" || got.Limit.Amount != 30000 || len(got.PeriodBoosts) != 0 {
		t.Fatalf("zero cover=%+v", got)
	}
}

func TestBC4CoverStampUsesLocalCalendarPeriod(t *testing.T) {
	a := newApp(t, false)
	now := time.Date(2026, 10, 31, 23, 30, 0, 0, time.FixedZone("Pacific", -7*60*60))
	a.now = func() time.Time { return now }
	for _, b := range []domain.Budget{sharedBudget("from", "Food", "food", 30000), sharedBudget("to", "Dining", "dining", 10000)} {
		if err := a.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.CoverBudget("from", "to", money.New(5000, "USD")); err != nil {
		t.Fatal(err)
	}
	in, err := a.BudgetViewInput(now)
	if err != nil {
		t.Fatal(err)
	}
	v := budgeting.EvaluateViewBudget(bc4Budget(t, a, "to"), in)
	if !v.Valid || !v.Covered || v.Status.Limit.Amount != 15000 {
		t.Fatalf("local October cover: covered=%v cap=%v err=%v", v.Covered, v.Status.Limit, v.Error)
	}
}
