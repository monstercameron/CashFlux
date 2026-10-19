// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestBCS1BudgetViewSharesRefundLedgerAndCachedSummary(t *testing.T) {
	app := br2sugTestApp(t)
	start := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "food", Name: "Food", CategoryID: "transport-cat", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(20000, "USD")}
	if err := app.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	br2sugPutExpense(t, app, "buy", start.AddDate(0, 0, 9), -10000)
	br2sugPutExpense(t, app, "refund", start.AddDate(0, 1, 9), 6000)
	if err := app.PutTxnLink(domain.TxnLink{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: money.New(6000, "USD")}); err != nil {
		t.Fatal(err)
	}
	first := budgetLedgerForApp(app)
	window := period.NewWindow(period.Month, start, time.Monday)
	v := computeBudgetView(app, "", window, prefs.Prefs{}, false)
	if len(v.Statuses) != 1 || v.Statuses[0].Spent.Amount != 4000 || !v.SummaryReady || v.SummaryActual != 4000 || len(v.SummaryIssues) != 0 {
		t.Fatalf("October view=%+v", v)
	}
	plan := budgetSummaryPlan(v, app, window)
	if plan.planView.TotalSpent != 4000 {
		t.Fatalf("cached summary=%d want4000", plan.planView.TotalSpent)
	}
	again := computeBudgetView(app, "", window, prefs.Prefs{}, false)
	if &again.Statuses[0] != &v.Statuses[0] {
		t.Fatal("unchanged view was recomputed")
	}
	next := computeBudgetView(app, "", period.NewWindow(period.Month, start.AddDate(0, 1, 0), time.Monday), prefs.Prefs{}, false)
	if next.SummaryActual != 0 || next.Statuses[0].Spent.Amount != 0 {
		t.Fatalf("November spent=%d summary=%d", next.Statuses[0].Spent.Amount, next.SummaryActual)
	}
	if &budgetLedgerForApp(app)[0] != &first[0] {
		t.Fatal("changing view windows rebuilt the ledger")
	}
	if err := app.DeleteTxnLink("pair"); err != nil {
		t.Fatal(err)
	}
	updated := computeBudgetView(app, "", window, prefs.Prefs{}, false)
	if updated.Statuses[0].Spent.Amount != 10000 || updated.SummaryActual != 10000 {
		t.Fatalf("link revision not reflected: %+v", updated)
	}
}

func TestBCS1ReviewAndInsightsUseBudgetCadence(t *testing.T) {
	app := br2sugTestApp(t)
	old := uistate.LoadPrefs()
	uistate.PersistPrefs(prefs.Prefs{WeekStart: prefs.WeekMonday})
	t.Cleanup(func() { uistate.PersistPrefs(old) })
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for _, b := range []domain.Budget{
		{ID: "weekly", Name: "Weekly", CategoryID: "transport-cat", Period: domain.PeriodWeekly, Limit: money.New(10000, "USD"), OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "yearly", Name: "Yearly", CategoryID: "year-cat", Period: domain.PeriodYearly, Limit: money.New(12000, "USD"), OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
	} {
		if err := app.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutCategory(domain.Category{ID: "year-cat", Name: "Annual", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	for i, day := range []int{5, 12, 19, 26} {
		br2sugPutExpense(t, app, string(rune('a'+i)), time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC), -7000)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "yearly-spend", Desc: "Annual purchase", AccountID: "br2sug-account", CategoryID: "year-cat", Date: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(-15000, "USD")}); err != nil {
		t.Fatal(err)
	}
	statuses := budgetStatusesAt(app, now)
	if len(statuses) != 2 {
		t.Fatalf("statuses=%+v", statuses)
	}
	for _, st := range statuses {
		switch st.Budget.ID {
		case "weekly":
			if st.State != budgeting.StateOK || st.Spent.Amount != 7000 {
				t.Fatalf("weekly judged against the month: %+v", st)
			}
		case "yearly":
			if st.State != budgeting.StateOver || st.Spent.Amount != 15000 {
				t.Fatalf("yearly lost prior spend: %+v", st)
			}
		}
	}
	source := &insightsQASource{app: app, now: now, base: "USD", rates: currency.Rates{Base: "USD"}}
	total, over, worst, amount, ok := source.BudgetStatus()
	if !ok || total != 2 || over != 1 || worst != "Yearly" || amount != 3000 {
		t.Fatalf("insights: %d %d %q %d %v", total, over, worst, amount, ok)
	}
}

func TestBCS1BudgetViewCacheTracksPreferencesAndStoreSettings(t *testing.T) {
	app := br2sugTestApp(t)
	b := domain.Budget{ID: "cache", Name: "Cache", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: "transport-cat", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")}
	if err := app.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	window := period.NewWindow(period.Month, time.Now(), time.Monday)
	pr := prefs.Prefs{}
	one := computeBudgetView(app, "", window, pr, false)
	pr.BudgetIncomeMode = "fixed"
	pr.MonthlyIncomeMinor = 87654
	two := computeBudgetView(app, "", window, pr, false)
	if &one.Statuses[0] == &two.Statuses[0] || two.BannerIncome != 87654 {
		t.Fatalf("preferences not in cache key: income=%d", two.BannerIncome)
	}
	settings := app.Settings()
	settings.BaseCurrency = "EUR"
	settings.FXRates = map[string]float64{"USD": 1}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	three := computeBudgetView(app, "", window, pr, false)
	if three.Base != "EUR" || &three.Statuses[0] == &two.Statuses[0] {
		t.Fatal("settings revision not in cache key")
	}
	budgetViewCache.Lock()
	entries := len(budgetViewCache.values)
	budgetViewCache.Unlock()
	if entries > 8 {
		t.Fatalf("unbounded cache: %d", entries)
	}
}

func TestBCS1InsightsSavingsShortfallAndMissingFX(t *testing.T) {
	for _, tt := range []struct {
		name, currency string
		wantOK         bool
	}{
		{"savings shortfall", "USD", true},
		{"unavailable conversion", "EUR", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app := br2sugTestApp(t)
			b := domain.Budget{ID: "saving", Name: "Savings", CategoryID: "transport-cat", Period: domain.PeriodMonthly, Direction: domain.DirectionSave,
				Limit: money.New(10000, tt.currency), OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
			if err := app.PutBudget(b); err != nil {
				t.Fatal(err)
			}
			source := &insightsQASource{app: app, now: time.Now(), base: "USD", rates: currency.Rates{Base: "USD"}}
			total, over, name, amount, ok := source.BudgetStatus()
			if ok != tt.wantOK {
				t.Fatalf("available=%v want=%v", ok, tt.wantOK)
			}
			if tt.wantOK && (total != 1 || over != 1 || name != "Savings" || amount != 10000) {
				t.Fatalf("saving shortfall=%d %d %q %d", total, over, name, amount)
			}
		})
	}
}
