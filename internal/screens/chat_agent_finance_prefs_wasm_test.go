// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestAssistantFinancePreferencesStayBoundFreshAndFallible(t *testing.T) {
	first := primaryAccountsGoalsApp(t)
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	previousClock := assistantFinanceNow
	assistantFinanceNow = func() time.Time { return now }
	t.Cleanup(func() { assistantFinanceNow = previousClock })
	if err := first.Store().Load(store.Dataset{Settings: store.Settings{BaseCurrency: "USD"}, Budgets: []domain.Budget{{ID: "weekly", Name: "Weekly calendar", CategoryID: "food", Scope: domain.ScopeShared, Period: domain.PeriodWeekly, Limit: money.New(10000, "USD")}, {ID: "biweekly", Name: "Pay-cycle calendar", CategoryID: "food", Scope: domain.ScopeShared, Period: domain.PeriodBiweekly, Limit: money.New(10000, "USD")}}}); err != nil {
		t.Fatal(err)
	}
	setPrefs := func(appStore *store.SQLiteStore, p prefs.Prefs) {
		t.Helper()
		b, _ := json.Marshal(p)
		if err := appStore.SetSettingKV("cashflux:prefs", string(b)); err != nil {
			t.Fatal(err)
		}
	}
	p := prefs.Default()
	p.WeekStart, p.PayCycleAnchor, p.BudgetRolloverLeftover = prefs.WeekMonday, "2026-08-07", true
	setPrefs(first.Store(), p)
	tool := assistantFinanceTestTool(t, agToolsFinance(first, "USD", currency.Rates{Base: "USD"}), "list_budgets")
	second := primaryAccountsGoalsApp(t)
	other := prefs.Default()
	other.WeekStart, other.PayCycleAnchor = prefs.WeekSunday, "2026-08-14"
	setPrefs(second.Store(), other)
	out := tool.run(json.RawMessage(`{"period":"this_month"}`))
	if !strings.Contains(out, "Weekly calendar [2026-08-10, 2026-08-17)") || !strings.Contains(out, "Pay-cycle calendar [2026-08-07, 2026-08-21)") {
		t.Fatal("foreign Default calendar/prefs leaked", out)
	}
	// Persist a new bound-store policy without touching its global preference
	// cache/atom or rebuilding the already-created tool closure.
	p.WeekStart, p.PayCycleAnchor, p.BudgetRolloverLeftover = prefs.WeekSaturday, "2026-08-14", false
	setPrefs(first.Store(), p)
	out = tool.run(json.RawMessage(`{"period":"this_month"}`))
	if !strings.Contains(out, "Weekly calendar [2026-08-08, 2026-08-15)") || !strings.Contains(out, "Pay-cycle calendar [2026-07-31, 2026-08-14)") {
		t.Fatal("bound persisted calendar remained stale", out)
	}
	if err := first.Store().SetSettingKV("cashflux:prefs", "{broken"); err != nil {
		t.Fatal(err)
	}
	_, err := assistantFinanceForCall(first, "USD", nil)
	var readError *assistantFinanceReadError
	if !errors.As(err, &readError) || readError.Collection != "financial preferences" || errors.Unwrap(readError) == nil {
		t.Fatal("lost preference source/cause", err)
	}
	out = tool.run(json.RawMessage(`{"period":"this_month"}`))
	if !strings.Contains(out, "Calculation unavailable: read finance financial preferences") || strings.Contains(out, "Weekly calendar") {
		t.Fatal("corrupt preferences became healthy default facts", out)
	}
}
