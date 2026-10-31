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

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
)

func TestAssistantReportsPreserveExactPartialWindows(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	oldNow := agReportNow
	agReportNow = func() time.Time { return now }
	t.Cleanup(func() { agReportNow = oldNow })
	for _, row := range []domain.Transaction{
		{ID: "window-income", AccountID: "a-check", Desc: "Window deposit", CategoryID: "c-sal", Date: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(10000, "USD")},
		{ID: "window-start", AccountID: "a-check", Desc: "Window shop", CategoryID: "c-gro", Date: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(-2000, "USD")},
		{ID: "window-last", AccountID: "a-check", Desc: "Window shop", CategoryID: "c-gro", Date: time.Date(2026, 8, 20, 23, 59, 59, 0, time.UTC), Amount: money.New(-1000, "USD")},
		{ID: "window-end", AccountID: "a-check", Desc: "Outside window", CategoryID: "c-gro", Date: time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC), Amount: money.New(-500000, "USD")},
	} {
		if err := app.PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	tool := agrTool(t, app, "report_section", aicontext.TierFull)
	for _, tc := range []struct {
		id    string
		wants []string
	}{
		{"overview", []string{"income $100.00", "spending $30.00", "net $70.00", "70%"}},
		{"monthly_flow", []string{"income $100.00", "spending $30.00", "net $70.00", "70%"}},
		{"spending_by_category", []string{"total $30.00", "Groceries: $30.00"}},
		{"category_trends", []string{"total $30.00"}},
		{"top_payees", []string{"Window shop: $30.00"}},
		{"payee_trends", []string{"Window shop: $30.00"}},
		{"savings_rate", []string{"70%", "kept $70.00 of $100.00"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			out := tool.run(json.RawMessage(`{"section":"` + tc.id + `","from":"2026-08-15","to":"2026-08-20"}`))
			for _, want := range append(tc.wants, "[2026-08-15, 2026-08-21)") {
				if !strings.Contains(out, want) {
					t.Fatalf("missing %q: %s", want, out)
				}
			}
			if strings.Contains(out, "Report unavailable:") {
				t.Fatalf("valid partial period failed: %s", out)
			}
		})
	}
	partial := tool.run(json.RawMessage(`{"section":"runway","from":"2026-08-15","to":"2026-08-20"}`))
	if !strings.Contains(partial, "requires complete calendar months") || !strings.Contains(partial, "not extrapolated") {
		t.Fatalf("partial burn was extrapolated: %s", partial)
	}
	trace := agrTool(t, app, "trace_report_row", aicontext.TierFull).run(json.RawMessage(`{"section":"monthly_flow","row":"2026-08","from":"2026-08-15","to":"2026-08-20"}`))
	for _, want := range []string{"window-i", "window-s", "window-l", "before split attribution"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("trace missing %q: %s", want, trace)
		}
	}
	if strings.Contains(trace, "window-e") || strings.Contains(trace, "cc333333") {
		t.Fatalf("trace included an outside-window receipt: %s", trace)
	}
}

func TestAssistantReportsRejectTruncatedSeriesAndReadFailures(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	tool := agrTool(t, app, "report_section", aicontext.TierFull)
	for _, id := range []string{"monthly_flow", "category_trends", "payee_trends", "savings_rate", "runway"} {
		valid := tool.run(json.RawMessage(`{"section":"` + id + `","from":"2024-01-01","to":"2026-12-31"}`))
		if strings.Contains(valid, "Report unavailable:") {
			t.Fatalf("exactly 36 calendar buckets rejected (%s): %s", id, valid)
		}
		out := tool.run(json.RawMessage(`{"section":"` + id + `","from":"2023-12-01","to":"2026-12-31"}`))
		if !strings.Contains(out, "exceeds 36 monthly buckets") || !strings.Contains(out, "narrower from/to") {
			t.Fatalf("%s silently truncated: %s", id, out)
		}
	}
	for _, args := range []string{`{"section":"overview","from":"not-a-date"}`, `{"section":"overview","period":"typo"}`, `{"section":"overview","from":"2026-08-20","to":"2026-08-01"}`, `{`} {
		if out := tool.run(json.RawMessage(args)); !strings.HasPrefix(out, "Report unavailable:") {
			t.Fatalf("invalid arguments became financial data: %s", out)
		}
	}
	trace := agrTool(t, app, "trace_report_row", aicontext.TierFull)
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	_, err := agBuildRptCtx(app, "USD", currency.Rates{Base: "USD"}, "this_month", "", "", "")
	var readErr *agReportReadError
	if !errors.As(err, &readErr) || readErr.Collection != "settings" || errors.Unwrap(readErr) == nil {
		t.Fatalf("lost typed store cause: %T %v", err, err)
	}
	if len(agReportSections()) != 24 {
		t.Fatalf("catalog changed: %d", len(agReportSections()))
	}
	for _, section := range agReportSections() {
		out := tool.run(json.RawMessage(`{"section":"` + section.ID + `",` + agrArgs + `}`))
		if !strings.HasPrefix(out, "Report unavailable: read report settings:") {
			t.Fatalf("%s read failure became empty data: %s", section.ID, out)
		}
	}
	if out := trace.run(json.RawMessage(`{"section":"spending_by_category","row":"Groceries",` + agrArgs + `}`)); !strings.HasPrefix(out, "Report unavailable: read report settings:") {
		t.Fatalf("trace masked read failure: %s", out)
	}
}

func TestAssistantReportsUseFreshFXScopeAndSectionInputs(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close(); reports.SetRefundLinks(nil) })
	oldNow := agReportNow
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	agReportNow = func() time.Time { return now }
	t.Cleanup(func() { agReportNow = oldNow })
	account := netWorthFixtureAccount("report-euro", "EUR", 1000, false)
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "report-fees", Name: "Bank fees", Kind: domain.KindExpense, Deductible: true}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCustomFieldDef(customfields.Def{ID: "report-purpose", EntityType: "transaction", Key: "purpose", Label: "Purpose", Type: customfields.TypeText}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "report-euro-fee", AccountID: account.ID, Desc: "Bank fee", CategoryID: "report-fees", Date: agrDay, Amount: money.New(-500, "EUR"), Tags: []string{"bank"}, Custom: map[string]any{"purpose": "Banking"}}); err != nil {
		t.Fatal(err)
	}
	sc := scope.ReportScope{}
	tools := agToolsReports(app, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 999}}, aicontext.TierFull, func() scope.ReportScope { return sc })
	tool := assistantFinanceTestTool(t, tools, "report_section")
	independent := map[string]bool{"income_by_category": true, "largest_income": true, "no_spend_days": true, "investment_performance": true, "net_worth": true}
	for _, sec := range agReportSections() {
		out := tool.run(json.RawMessage(`{"section":"` + sec.ID + `",` + agrArgs + `,"field":"purpose"}`))
		if independent[sec.ID] {
			if strings.Contains(out, "Report unavailable:") {
				t.Fatalf("%s required unrelated expense FX: %s", sec.ID, out)
			}
		} else if !strings.Contains(out, "Report unavailable:") {
			t.Fatalf("%s treated missing FX as zero/empty: %s", sec.ID, out)
		}
	}
	nw := tool.run(json.RawMessage(`{"section":"net_worth",` + agrArgs + `}`))
	if !strings.Contains(nw, "Known household net-worth subtotal") || !strings.Contains(nw, "missing exchange rates for EUR") {
		t.Fatalf("missing-FX household subtotal undisclosed: %s", nw)
	}
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	args := json.RawMessage(`{"section":"spending_by_category",` + agrArgs + `}`)
	if out := tool.run(args); !strings.Contains(out, "Bank fees: $10.00") {
		t.Fatalf("handler retained registration FX: %s", out)
	}
	settings.FXRates = map[string]float64{"EUR": 3}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if out := tool.run(args); !strings.Contains(out, "Bank fees: $15.00") {
		t.Fatalf("handler did not read changed FX: %s", out)
	}
	sc = scope.ReportScope{AccountIDs: []string{"a-check"}}
	if out := tool.run(args); strings.Contains(out, "Bank fees:") || !strings.Contains(out, "total $1,725.00") || !strings.Contains(out, "account IDs a-check") {
		t.Fatalf("selected activity scope stale: %s", out)
	}
	nw = tool.run(json.RawMessage(`{"section":"net_worth",` + agrArgs + `}`))
	if !strings.Contains(nw, account.Name) || !strings.Contains(nw, "Household net worth now") {
		t.Fatalf("activity scope narrowed household balance sheet: %s", nw)
	}
	// The no-spend-days reference is read at invocation, including same-month day changes.
	before := tool.run(json.RawMessage(`{"section":"no_spend_days",` + agrArgs + `}`))
	now = now.AddDate(0, 0, 1)
	after := tool.run(json.RawMessage(`{"section":"no_spend_days",` + agrArgs + `}`))
	if before == after || !strings.Contains(before, "13 days") || !strings.Contains(after, "14 days") {
		t.Fatalf("stale report clock: before=%s after=%s", before, after)
	}
}

func TestAssistantReportsRetainExplicitRefundAndOverflowAvailability(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close(); reports.SetRefundLinks(nil) })
	if err := app.PutTransaction(domain.Transaction{ID: "report-refund", AccountID: "a-check", Date: agrDay.AddDate(0, 1, 0), Desc: "Return", Amount: money.New(4000, "USD")}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTxnLink(domain.TxnLink{ID: "report-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"cc33333333333333", "report-refund"}}); err != nil {
		t.Fatal(err)
	}
	tool := agrTool(t, app, "report_section", aicontext.TierFull)
	for _, id := range []string{"overview", "monthly_flow", "spending_by_category"} {
		out := tool.run(json.RawMessage(`{"section":"` + id + `",` + agrArgs + `}`))
		want := "$1,685.00"
		if id == "spending_by_category" {
			want = "Groceries: $160.00"
		}
		if !strings.Contains(out, want) {
			t.Fatalf("%s applied global refunds twice or not at all: %s", id, out)
		}
	}
	if err := app.PutTransaction(domain.Transaction{ID: "report-split", AccountID: "a-check", Date: agrDay, Desc: "Split purchase", Amount: money.New(-3000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "c-gro", Amount: money.New(-1000, "USD")}, {CategoryID: "c-rent", Amount: money.New(-2000, "USD")}}}); err != nil {
		t.Fatal(err)
	}
	out := tool.run(json.RawMessage(`{"section":"spending_by_category",` + agrArgs + `}`))
	if !strings.Contains(out, "Groceries: $170.00") || !strings.Contains(out, "Rent: $1,520.00") {
		t.Fatalf("lost split attribution: %s", out)
	}
	// Preserve the legacy ambiguity explicitly; do not invent a refund distribution.
	if err := app.PutTransaction(domain.Transaction{ID: "report-split-refund", AccountID: "a-check", Date: agrDay.AddDate(0, 1, 0), Desc: "Split return", Amount: money.New(1000, "USD")}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTxnLink(domain.TxnLink{ID: "report-split-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"report-split", "report-split-refund"}}); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{"section":"spending_by_category",` + agrArgs + `}`))
	if !strings.Contains(out, "category attribution is ambiguous") {
		t.Fatalf("split refund looked exact: %s", out)
	}

	fund := netWorthFixtureAccount("report-fund", "USD", 10000, false)
	fund.Type = domain.TypeInvestment
	fund.Name = "Report fund"
	if err := app.PutAccount(fund); err != nil {
		t.Fatal(err)
	}
	for _, row := range []domain.Transaction{
		{ID: "report-old-basis", AccountID: fund.ID, Date: agrDay.AddDate(0, -1, 0), Desc: "Fund contribution", Amount: money.New(1000, "USD"), TransferAccountID: "a-check"},
		{ID: "report-value-update", AccountID: fund.ID, Date: agrDay.AddDate(0, 1, 0), Desc: "Fund value update", Amount: money.New(500, "USD"), ExcludeFromReports: true},
	} {
		if err := app.PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	investment := tool.run(json.RawMessage(`{"section":"investment_performance",` + agrArgs + `}`))
	for _, want := range []string{"put in $110.00", "now worth $115.00", "gain $5.00", "full recorded history"} {
		if !strings.Contains(investment, want) {
			t.Fatalf("investment basis missed %q: %s", want, investment)
		}
	}
	trace := agrTool(t, app, "trace_report_row", aicontext.TierFull).run(json.RawMessage(`{"section":"investment_performance","row":"Report fund",` + agrArgs + `}`))
	for _, want := range []string{"Fund contribution", "Fund value update", "full recorded account history", "does not restrict investment basis"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("full-history evidence missed %q: %s", want, trace)
		}
	}
	// A valid wide return must survive the actual js/wasm host without narrowing.
	extreme := netWorthFixtureAccount("report-return-boundary", "USD", 1, false)
	extreme.Type = domain.TypeInvestment
	if err := app.PutAccount(extreme); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "report-return-gain", AccountID: extreme.ID, Date: agrDay, Desc: "Large relative gain", Amount: money.New(400000, "USD")}); err != nil {
		t.Fatal(err)
	}
	investment = tool.run(json.RawMessage(`{"section":"investment_performance",` + agrArgs + `}`))
	if !strings.Contains(investment, "+40000000.0%") {
		t.Fatalf("representable wide investment return narrowed: %s", investment)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "report-return-gain", AccountID: extreme.ID, Date: agrDay, Desc: "Unrepresentable relative gain", Amount: money.New(math.MaxInt64-1, "USD")}); err != nil {
		t.Fatal(err)
	}
	investment = tool.run(json.RawMessage(`{"section":"investment_performance",` + agrArgs + `}`))
	if !strings.Contains(investment, "Report unavailable:") {
		t.Fatalf("unrepresentable investment return became a figure: %s", investment)
	}
	// Finite source values whose sum cannot fit must never become a zero or negative total.
	if err := app.PutTransaction(domain.Transaction{ID: "report-overflow", AccountID: "a-check", Date: agrDay, Desc: "Large expense", Amount: money.New(-math.MaxInt64, "USD")}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"overview", "monthly_flow", "spending_by_category", "spending_stats", "year_tax"} {
		out := tool.run(json.RawMessage(`{"section":"` + id + `",` + agrArgs + `}`))
		if !strings.Contains(out, "unavailable") {
			t.Fatalf("%s wrapped finite money: %s", id, out)
		}
	}
}
