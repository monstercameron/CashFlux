// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func bcs2RecurringApp(t *testing.T) *appstate.App {
	t.Helper()
	a, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = previous; _ = a.Store().Close() })
	if err := uistate.PersistPrefs(prefs.Default()); err != nil {
		t.Fatal(err)
	}
	return a
}

func bcs2RecurringBudget(id string, amount int64) domain.Budget {
	return domain.Budget{ID: id, Name: id, CategoryID: id, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(amount, "USD")}
}

func TestRecurringBCS2RekeyThenBootDoesNotReplay(t *testing.T) {
	a := bcs2RecurringApp(t)
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	from, to := bcs2RecurringBudget("from", 30000), bcs2RecurringBudget("to", 10000)
	to.Period = domain.PeriodWeekly
	to.RecurringCover = &domain.RecurringCover{AmountMinor: 5000, Sources: []domain.CoverShare{{BudgetID: "from", Weight: 1}}}
	for _, b := range []domain.Budget{from, to} {
		if err := a.PutBudgetAt(b, now); err != nil {
			t.Fatal(err)
		}
	}
	applyRecurringCoversAt(now)
	next := prefs.Default()
	next.WeekStart = prefs.WeekMonday
	if err := uistate.PersistPrefs(next); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(a.Budgets())
	applyRecurringCoversAt(now)
	after, _ := json.Marshal(a.Budgets())
	if string(before) != string(after) {
		t.Fatal("boot debited sources again after re-key")
	}
	for _, b := range a.Budgets() {
		if b.ID == "to" && (b.PeriodBoosts["2026-10-12"] != 5000 || b.RecurringCover.LastAppliedPeriod != "2026-10-12") {
			t.Fatalf("destination=%+v", b)
		}
	}
}

func TestRecurringBCS2FormulaSettlesLastClosedPeriodAtEffectiveCap(t *testing.T) {
	a := bcs2RecurringApp(t)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	from, to := bcs2RecurringBudget("from", 30000), bcs2RecurringBudget("to", 10000)
	to.RecurringCover = &domain.RecurringCover{AmountFormula: "overspend", Sources: []domain.CoverShare{{BudgetID: "from", Weight: 1}}}
	var err error
	to, err = to.WithPeriodBoostChecked(closed, 2000)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []domain.Budget{from, to} {
		if err := a.PutBudgetAt(b, closed); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Store().PutTransaction(domain.Transaction{ID: "last-spend", CategoryID: "to", Date: closed.AddDate(0, 0, 10), Amount: money.New(-13000, "USD")}); err != nil {
		t.Fatal(err)
	}
	applyRecurringCoversAt(now)
	for _, b := range a.Budgets() {
		want := int64(-1000)
		if b.ID == "to" {
			want = 3000
			if b.RecurringCover.LastAppliedPeriod != "2026-09-01" {
				t.Fatal("open period stamped")
			}
		}
		if b.PeriodBoost(closed) != want || b.PeriodBoost(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) != 0 {
			t.Fatalf("wrong settlement: %+v", b)
		}
	}
	before, _ := json.Marshal(a.Budgets())
	applyRecurringCoversAt(now)
	after, _ := json.Marshal(a.Budgets())
	if string(before) != string(after) {
		t.Fatal("closed-period settlement replayed")
	}
}
