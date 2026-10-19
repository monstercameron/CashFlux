// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestBV7FFutureProjectionMemoReusesTheSameScopedPeriod(t *testing.T) {
	app := guardTestApp(t)
	dataset := store.SampleDataset()
	transaction := dataset.Transactions[0]
	for i := 0; i < 20; i++ {
		copy := transaction
		copy.ID = fmt.Sprintf("bv7f-future-%02d", i)
		dataset.Transactions = append(dataset.Transactions, copy)
	}
	if err := app.Store().Load(dataset); err != nil {
		t.Fatal(err)
	}
	budget := quickFillFixtureBudget()
	if err := app.PutBudget(budget); err != nil {
		t.Fatal(err)
	}
	oldViews, oldCalculations := budgetFutureViews, budgetFutureViewCalculations
	budgetFutureViews = map[budgetFutureViewKey]budgetFutureTotals{}
	budgetFutureViewCalculations = 0
	t.Cleanup(func() {
		budgetFutureViews, budgetFutureViewCalculations = oldViews, oldCalculations
	})

	p := prefs.Default()
	month := time.Date(time.Now().Year(), time.Now().Month()+1, 1, 0, 0, 0, 0, time.Local)
	w := period.NewWindow(period.Month, month, p.WeekStartWeekday())
	reference := computeBudgetView(app, "", w, p, false)
	missStart := time.Now()
	first, err := cachedBudgetFutureView(app, "", w, p)
	if err != nil {
		t.Fatal(err)
	}
	missElapsed := time.Since(missStart)
	t.Logf("BV7F future-period first evaluation: %.3f ms", float64(missElapsed.Microseconds())/1000)
	if missElapsed > 250*time.Millisecond {
		t.Fatalf("future period first evaluation took %s", missElapsed)
	}
	if first.TotalLimit != reference.TotalLimit || first.SavingsAssigned != reference.SavingsAssigned || first.BannerIncome != reference.BannerIncome {
		t.Fatalf("future totals differ from the shared budget view: got %+v; want limit=%d savings=%d income=%d",
			first, reference.TotalLimit, reference.SavingsAssigned, reference.BannerIncome)
	}
	if budgetFutureViewCalculations != 1 {
		t.Fatalf("initial future-period calculations = %d, want 1", budgetFutureViewCalculations)
	}

	start := time.Now()
	second, err := cachedBudgetFutureView(app, "", w, p)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	t.Logf("BV7F cached future-period evaluation: %.3f ms", float64(elapsed.Microseconds())/1000)
	if elapsed > 250*time.Millisecond {
		t.Fatalf("cached future-period evaluation took %s", elapsed)
	}
	if budgetFutureViewCalculations != 1 || first.TotalLimit != second.TotalLimit || first.BannerIncome != second.BannerIncome {
		t.Fatal("same scoped period was recomputed or returned different figures")
	}

	if _, err := cachedBudgetFutureView(app, "member-scope", w, p); err != nil {
		t.Fatal(err)
	}
	if budgetFutureViewCalculations != 2 {
		t.Fatal("member-scoped future projection reused another member's view")
	}
	b := app.Budgets()[0]
	b.Limit.Amount++
	if err := app.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	if _, err := cachedBudgetFutureView(app, "", w, p); err != nil {
		t.Fatal(err)
	}
	if budgetFutureViewCalculations != 3 {
		t.Fatal("budget revision reused a stale future projection")
	}
}
