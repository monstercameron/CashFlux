// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSOLB2BudgetWritesRecordHistory(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	b := sharedBudget("b", "Food", "food", 10000)
	b.Rollover = true
	if err := a.PutBudgetWithHistory(b); err != nil {
		t.Fatal(err)
	}
	b = a.Budgets()[0]
	if b.RolloverStart != moneyDate("2026-10-01") {
		t.Errorf("start=%v", b.RolloverStart)
	}
	a.now = func() time.Time { return moneyDate("2026-11-19") }
	b.Limit.Amount = 20000
	if err := a.PutBudgetsAtomic([]domain.Budget{b}); err != nil {
		t.Fatal(err)
	}
	b = a.Budgets()[0]
	if b.LimitHistory["2026-10-01"] != 10000 || b.LimitHistory["2026-11-01"] != 20000 {
		t.Fatalf("history=%v", b.LimitHistory)
	}
}

func TestSOLB2BudgetFlagUsesCarryBoostAndAnchor(t *testing.T) {
	for _, mode := range []string{"carry", "boost", "anchor"} {
		t.Run(mode, func(t *testing.T) {
			a := newApp(t, false)
			a.now = func() time.Time { return moneyDate("2026-10-10") }
			b := sharedBudget("b", "Food", "food", 10000)
			var txns []domain.Transaction
			switch mode {
			case "carry":
				b.Rollover = true
				b.RolloverStart = moneyDate("2026-09-01")
				txns = []domain.Transaction{{ID: "t", CategoryID: "food", Amount: b.Limit, Date: moneyDate("2026-10-08")}}
				txns[0].Amount.Amount = -15000
			case "boost":
				b.PeriodBoosts = map[string]int64{"2026-10-01": 10000}
				txns = []domain.Transaction{{ID: "t", CategoryID: "food", Amount: b.Limit, Date: moneyDate("2026-10-08")}}
				txns[0].Amount.Amount = -15000
			case "anchor":
				b.Period = domain.PeriodBiweekly
				if err := a.SetSettingKV("cashflux:prefs", `{"weekStart":"monday","payCycleAnchor":"2026-10-03"}`); err != nil {
					t.Fatal(err)
				}
				txns = []domain.Transaction{{ID: "t", CategoryID: "food", Amount: b.Limit, Date: moneyDate("2026-09-29")}}
				txns[0].Amount.Amount = -15000
			}
			if err := a.store.PutBudget(b); err != nil {
				t.Fatal(err)
			}
			for _, txn := range txns {
				if err := a.store.PutTransaction(txn); err != nil {
					t.Fatal(err)
				}
			}
			if a.isBudgetOver(b) {
				t.Fatal("automation contradicts evaluated card")
			}
			a.applyFlagBudgetOver()
			if len(a.Tasks()) != 0 {
				t.Fatal("created false over-budget task")
			}
		})
	}
}
func TestSOLB2BudgetHistoryEffectiveDateAndRepair(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	b := sharedBudget("b", "Food", "food", 10000)
	b.Rollover = true
	if err := a.store.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	if err := a.SetRolloverStart("b", moneyDate("2026-08-19")); err != nil {
		t.Fatal(err)
	}
	b = a.Budgets()[0]
	if !b.RolloverStart.Equal(moneyDate("2026-08-01")) {
		t.Fatal(b.RolloverStart)
	}
	b.Limit.Amount = 20000
	if err := a.PutBudgetAt(b, moneyDate("2026-11-19")); err != nil {
		t.Fatal(err)
	}
	b = a.Budgets()[0]
	if b.LimitHistory["2026-08-01"] != 10000 || b.LimitHistory["2026-11-01"] != 20000 {
		t.Fatal(b.LimitHistory)
	}
	if err := a.SetRolloverStart("b", time.Time{}); err == nil {
		t.Fatal("zero start accepted")
	}
}
func TestSOLB2BudgetFlagTitleMetadata(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	b := sharedBudget("b", "Food", "food", 10000)
	if err := a.store.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	txn := domain.Transaction{ID: "t", CategoryID: "food", Amount: b.Limit, Date: moneyDate("2026-10-08")}
	txn.Amount.Amount = -15000
	if err := a.store.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	a.applyFlagBudgetOver()
	a.applyFlagBudgetOver()
	tasks := a.Tasks()
	if len(tasks) != 1 || tasks[0].RelatedID != b.ID || tasks[0].Custom["titleKey"] != "tasks.budgetOverLimit" {
		t.Fatalf("tasks=%+v", tasks)
	}
}

func TestSOLB2BudgetInputHasExplicitRefundLedger(t *testing.T) {
	a := newApp(t, false)
	in, err := a.BudgetViewInput(moneyDate("2026-10-19"))
	if err != nil || in.Ledger == nil {
		t.Fatalf("input has no explicit ledger: %v", err)
	}
}

func TestSOLB2CreationRecordsStartForEnvelopeWithoutRolloverFlag(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	b := sharedBudget("envelope", "Food", "food", 10000)
	b.Methodology = "envelope"
	if err := a.CreateBudgetWithCategory(nil, b); err != nil {
		t.Fatal(err)
	}
	got := a.Budgets()[0]
	if got.RolloverStart != moneyDate("2026-10-01") {
		t.Fatalf("envelope creation start=%v", got.RolloverStart)
	}
	if err := a.SetRolloverStart(b.ID, moneyDate("2026-09-19")); err != nil {
		t.Fatalf("repair envelope: %v", err)
	}
}
