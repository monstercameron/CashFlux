// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
)

func assistantFinanceTestTool(t *testing.T, tools []chatTool, name string) chatTool {
	t.Helper()
	for _, tool := range tools {
		if tool.spec.Function.Name == name {
			return tool
		}
	}
	t.Fatalf("missing %s", name)
	return chatTool{}
}

func TestAssistantFinanceToolsPreserveStoreReadFailure(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	tools := agToolsFinance(app, "USD", currency.Rates{Base: "USD"})
	if out := assistantFinanceTestTool(t, tools, "account_balances").run(json.RawMessage(`{}`)); !strings.Contains(out, "Checking") || strings.Contains(out, "Calculation unavailable:") {
		t.Fatalf("healthy account read failed: %s", out)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	_, err := assistantFinanceForCall(app, "USD", nil)
	var readError *assistantFinanceReadError
	if !errors.As(err, &readError) || readError.Collection != "settings" || errors.Unwrap(readError) == nil {
		t.Fatalf("store error lost its source or cause: %T %v", err, err)
	}
	// Reuse the exact handlers that succeeded before the store closed. Every
	// required argument is present, so a query error cannot mask the read failure.
	args := json.RawMessage(`{"category":"Groceries","amount":"1.00","from":"2026-08-01","to":"2026-08-31"}`)
	if len(tools) != 6 {
		t.Fatalf("finance reads=%d, want 6", len(tools))
	}
	for _, tool := range tools {
		t.Run(tool.spec.Function.Name, func(t *testing.T) {
			out := tool.run(args)
			if !strings.HasPrefix(out, "Calculation unavailable: read finance settings:") {
				t.Fatalf("store failure became financial data or an empty list: %s", out)
			}
			for _, falseClaim := range []string{"No active accounts", "No visible budgets", "No spending", "Income $0.00", "Spent $0.00", "Projected cash covers"} {
				if strings.Contains(out, falseClaim) {
					t.Fatalf("failed read claimed %q: %s", falseClaim, out)
				}
			}
		})
	}
}

func TestAssistantFinanceToolsReadLiveRefundSplitScopeAndFX(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close(); reports.SetRefundLinks(nil) })
	clock := agrDay.AddDate(0, 0, 11)
	oldClock := assistantFinanceNow
	assistantFinanceNow = func() time.Time { return clock }
	t.Cleanup(func() { assistantFinanceNow = oldClock })
	sc := scope.ReportScope{}
	tools := agToolsFinance(app, "USD", currency.Rates{Base: "USD"}, func() scope.ReportScope { return sc })
	if len(tools) != 6 {
		t.Fatalf("replacement tools=%d", len(tools))
	}
	for _, tool := range tools {
		if !json.Valid(tool.spec.Function.Parameters) || tool.mutates {
			t.Fatalf("bad read-tool schema/mutability: %s", tool.spec.Function.Name)
		}
	}
	byCategory := assistantFinanceTestTool(t, tools, "spending_by_category")
	args := json.RawMessage(`{"category":"Groceries","from":"2026-08-01","to":"2026-08-31"}`)
	if out := byCategory.run(args); !strings.Contains(out, "$200.00") || !strings.Contains(out, "[2026-08-01, 2026-09-01)") {
		t.Fatalf("initial facts: %s", out)
	}
	for _, txn := range []domain.Transaction{
		{ID: "finance-refund", AccountID: "a-check", Date: agrDay.AddDate(0, 1, 0), Desc: "Returned groceries", CategoryID: "c-gro", Amount: money.New(4000, "USD")},
		{ID: "finance-split", AccountID: "a-check", Date: agrDay, Desc: "Split receipt", Amount: money.New(-3000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "c-gro", Amount: money.New(-1000, "USD")}, {CategoryID: "c-rent", Amount: money.New(-2000, "USD")}}},
		{ID: "finance-excluded", AccountID: "a-check", Date: agrDay, Desc: "Excluded receipt", CategoryID: "c-gro", Amount: money.New(-99999, "USD"), ExcludeFromReports: true},
	} {
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutTxnLink(domain.TxnLink{ID: "finance-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"cc33333333333333", "finance-refund"}}); err != nil {
		t.Fatal(err)
	}
	if out := byCategory.run(args); !strings.Contains(out, "$170.00") || !strings.Contains(out, "3 contributing") {
		t.Fatalf("refund/split/exclusion live facts: %s", out)
	}
	if err := app.PutBudgetAt(domain.Budget{ID: "finance-budget", Name: "Food plan", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CategoryID: "c-gro", Period: domain.PeriodMonthly, Limit: money.New(20000, "USD")}, clock); err != nil {
		t.Fatal(err)
	}
	budget := assistantFinanceTestTool(t, tools, "list_budgets").run(json.RawMessage(`{"from":"2026-08-01","to":"2026-08-31"}`))
	for _, want := range []string{"Food plan", "effective $200.00", "spent $170.00", "remaining $30.00", "state near (85%)"} {
		if !strings.Contains(budget, want) {
			t.Fatalf("budget missed %q: %s", want, budget)
		}
	}
	foreign := netWorthFixtureAccount("finance-euro", "EUR", 1000, false)
	if err := app.PutAccount(foreign); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "finance-euro-spend", AccountID: foreign.ID, Date: agrDay, Desc: "Euro groceries", CategoryID: "c-gro", Amount: money.New(-500, "EUR")}); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if out := byCategory.run(args); !strings.Contains(out, "$180.00") {
		t.Fatalf("new FX/data not read: %s", out)
	}
	sc = scope.ReportScope{AccountIDs: []string{"a-check"}}
	if out := byCategory.run(args); !strings.Contains(out, "$170.00") || !strings.Contains(out, "selected active-account scope (1 accounts)") {
		t.Fatalf("scope provider stale: %s", out)
	}
	summaryTool := assistantFinanceTestTool(t, tools, "financial_summary")
	out := summaryTool.run(json.RawMessage(`{"from":"2026-08-01","to":"2026-08-31"}`))
	if !strings.Contains(out, "spending $1,715.00") || !strings.Contains(out, "household-wide and current") {
		t.Fatalf("summary scope/window/balance basis: %s", out)
	}
	sc = scope.ReportScope{}
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if out := byCategory.run(args); !strings.Contains(out, "Calculation unavailable:") || strings.Contains(out, "Spent $0.00") {
		t.Fatalf("missing FX became zero/partial spend: %s", out)
	}
	out = summaryTool.run(json.RawMessage(`{"from":"2026-08-01","to":"2026-08-31"}`))
	if !strings.Contains(out, "Activity: Calculation unavailable:") || !strings.Contains(out, "net-worth subtotal (incomplete; missing FX for EUR)") {
		t.Fatalf("unknown/subtotal semantics: %s", out)
	}
	balances := assistantFinanceTestTool(t, tools, "account_balances").run(json.RawMessage(`{}`))
	if !strings.Contains(balances, "Private finance-euro") || !strings.Contains(balances, "(EUR)") {
		t.Fatalf("account currency dropped: %s", balances)
	}
}

func TestAssistantFinanceToolsExactAmountAvailabilityAndDateRefresh(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	clock := agrDay
	oldClock := assistantFinanceNow
	assistantFinanceNow = func() time.Time { return clock }
	t.Cleanup(func() { assistantFinanceNow = oldClock })
	tools := agToolsFinance(app, "USD", currency.Rates{Base: "USD"})
	afford := assistantFinanceTestTool(t, tools, "check_affordability")
	if out := afford.run(json.RawMessage(`{"amount":"100.00","months":0,"reserve":"25.00"}`)); !strings.Contains(out, "Projected cash covers") || !strings.Contains(out, "explicit reserve $25.00") || !strings.Contains(out, "goal earmarks are not deducted") || !strings.Contains(out, "not a safe-to-spend amount") {
		t.Fatalf("projection basis omitted: %s", out)
	}
	for _, raw := range []string{`{"amount":"1.001"}`, `{"amount":1e100}`, `{"amount":-1}`, `{"amount":100,"months":-1}`, `{"amount":"92233720368547758.08"}`} {
		if out := afford.run(json.RawMessage(raw)); !strings.Contains(out, "Calculation unavailable:") {
			t.Fatalf("invalid amount/horizon accepted %s: %s", raw, out)
		}
	}
	settings := app.Settings()
	settings.BaseCurrency = "JPY"
	settings.FXRates = map[string]float64{"USD": 1}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if out := afford.run(json.RawMessage(`{"amount":9007199254740993,"months":0}`)); !strings.Contains(out, "9,007,199,254,740,993") || strings.Contains(out, "9,007,199,254,740,992") {
		t.Fatalf("decimal coerced through float: %s", out)
	}
	settings.BaseCurrency = "USD"
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	summary := assistantFinanceTestTool(t, tools, "financial_summary")
	if out := summary.run(json.RawMessage(`{}`)); !strings.Contains(out, "Income $4,500.00") {
		t.Fatalf("August read: %s", out)
	}
	clock = time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	if err := app.PutTransaction(domain.Transaction{ID: "finance-sept", AccountID: "a-check", Desc: "September purchase", Date: clock, Amount: money.New(-500, "USD")}); err != nil {
		t.Fatal(err)
	}
	if out := summary.run(json.RawMessage(`{}`)); !strings.Contains(out, "[2026-09-01, 2026-10-01)") || !strings.Contains(out, "Income $0.00; spending $5.00") {
		t.Fatalf("tool retained old date/data: %s", out)
	}
	if out := assistantFinanceTestTool(t, tools, "spending_breakdown").run(json.RawMessage(`{"from":"bad-date","to":"2026-09-30"}`)); !strings.Contains(out, "Calculation unavailable:") {
		t.Fatalf("bad window silently fell back: %s", out)
	}
	// A valid int64 amount whose two-month projection exceeds int64 must not
	// produce the old wrapped yes/no result, on the actual WASM architecture.
	if _, err := money.ParseMinor("92233720368547758.07", 2); err != nil {
		t.Fatal(err)
	}
	if result, err := assistantFinanceAmount(json.RawMessage(`92233720368547758.07`), "USD", false); err != nil || result.Amount != math.MaxInt64 {
		t.Fatalf("exact amount boundary=%+v %v", result, err)
	}
}
