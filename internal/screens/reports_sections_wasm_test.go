// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/reportrange"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/reportview"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type reportTestCSSSink struct{}

func (reportTestCSSSink) Emit(string, string) {}

// Hold Reports off the first mount while establishing the shared scope/window,
// so assertions measure report work rather than fixture preference setup.
func renderLazyReport(t *testing.T) (*render.Fixture, *appstate.App) {
	t.Helper()
	oldCSS := css.SetSink(reportTestCSSSink{})
	t.Cleanup(func() { css.SetSink(oldCSS) })
	// The Node fixture owns a mock DOM; report theme reads still expect a browser
	// document object. Hash tests supply their richer controlled document.
	global := js.Global()
	oldDocument := global.Get("document")
	if !oldDocument.Truthy() {
		doc := global.Get("Object").New()
		lookup := js.FuncOf(func(js.Value, []js.Value) any { return nil })
		doc.Set("getElementById", lookup)
		global.Set("document", doc)
		t.Cleanup(func() { global.Set("document", oldDocument); lookup.Release() })
	}
	doc := global.Get("document")
	for _, name := range []string{"addEventListener", "removeEventListener", "querySelector"} {
		old := doc.Get(name)
		if old.Type() != js.TypeFunction {
			callback := js.FuncOf(func(js.Value, []js.Value) any { return nil })
			doc.Set(name, callback)
			t.Cleanup(func() { doc.Set(name, old); callback.Release() })
		}
	}
	app := agrApp(t)
	if err := app.PutCategory(domain.Category{ID: "c-gro", Name: "Groceries", Kind: domain.KindExpense, Deductible: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	f := render.New(t)
	var lens, local state.Atom[scope.ReportScope]
	var window state.Atom[period.Window]
	var mount func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		lens, local, window = uistate.UseActiveScope(), uistate.UseReportScope(), uistate.UsePeriod()
		ready := ui.UseState(false)
		mount = ready.Set
		if ready.Get() {
			return ui.CreateElement(Reports)
		}
		return nil
	}))
	oldLens, oldLocal, oldWindow := lens.Get(), local.Get(), window.Get()
	oldRange, oldConfig := uistate.LoadReportRange(), uistate.ReportsConfigGet()
	t.Cleanup(func() {
		mount(false)
		lens.Set(oldLens)
		local.Set(oldLocal)
		window.Set(oldWindow)
		uistate.SaveReportRange(oldRange)
		uistate.SetReportsConfig(oldConfig)
	})
	lens.Set(scope.ReportScope{})
	local.Set(scope.ReportScope{})
	window.Set(period.NewWindow(period.Month, agrDay, time.Monday))
	uistate.SaveReportRange(reportrange.Defaults())
	uistate.SetReportsConfig(uistate.ReportsConfig{})
	mount(true)
	f.Stabilize()
	return f, app
}

func TestReportsSummarySkipsFullSectionBuildersAndRemountsFreshData(t *testing.T) {
	counts := map[string]int{}
	previous := reportSectionBuildObserver
	reportSectionBuildObserver = func(name string) { counts[name]++ }
	defer func() { reportSectionBuildObserver = previous }()
	f, app := renderLazyReport(t)
	for _, section := range []string{"moneyflow", "motion", "categories", "where", "appendix"} {
		if counts[section] != 0 {
			t.Fatalf("Summary built %s %d times", section, counts[section])
		}
	}
	if byTestID(f, "div", "rpta-summary") == nil {
		t.Fatal("default mode is not Summary")
	}
	full := byTestID(f, "button", "rpta-mode-full")
	if full == nil {
		t.Fatal("missing Full toggle")
	}
	full.Click()
	f.Stabilize()
	for _, section := range []string{"moneyflow", "motion", "categories", "where", "appendix"} {
		if counts[section] == 0 {
			t.Fatalf("Full never built %s", section)
		}
	}
	for _, id := range []string{"rpta-02", "rpta-03", "rpta-04", "rpta-05"} {
		if f.ByID(id) == nil {
			t.Errorf("Full omitted section %s", id)
		}
	}
	// Detailed reference content remains mounted inside its native disclosure;
	// closing details does not silently remove it from Full-mode print/export.
	if byTestID(f, "table", "rpta-months") == nil || byTestID(f, "span", "rpta-payee-spark-0") == nil {
		t.Fatal("Full omitted monthly table or payee sparkline")
	}
	if byTestID(f, "details", "rpta-appendix-fold") == nil || byTestID(f, "div", "deductible-section") == nil {
		t.Fatal("Full omitted content inside its closed appendix disclosure")
	}
	byTestID(f, "button", "rpta-mode-summary").Click()
	f.Stabilize()
	afterFull := map[string]int{}
	for name, n := range counts {
		afterFull[name] = n
	}
	if f.ByID("rpta-02") != nil {
		t.Fatal("Summary retained Full content")
	}
	uistate.SetReportScope(scope.ReportScope{AccountIDs: []string{"a-check"}})
	f.Stabilize()
	reset := byTestID(f, "button", "rpta-scope-reset")
	if reset == nil {
		t.Fatal("Summary omitted active scope reset")
	}
	reset.Click()
	f.Stabilize()
	if byTestID(f, "div", "rpta-summary") == nil {
		t.Fatal("scope reset shifted Summary control hooks")
	}
	if err := app.PutTransaction(domain.Transaction{ID: "lazy-new-expense", AccountID: "a-check", CategoryID: "c-gro", Desc: "Fresh expense", Date: agrDay, Amount: money.New(-12345, "USD")}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	for name, n := range afterFull {
		if counts[name] != n {
			t.Fatalf("Summary data refresh rebuilt %s", name)
		}
	}
	byTestID(f, "button", "rpta-mode-full").Click()
	f.Stabilize()
	if !strings.Contains(f.Text(), "Fresh expense") || !strings.Contains(f.Text(), "123.45") {
		t.Fatalf("remounted Full retained old activity: %s", f.Text())
	}
	if byTestID(f, "button", "rpta-mode-full").Attr("aria-pressed") != "true" {
		t.Fatal("Full toggle lost its state")
	}
	// Empty and populated snapshots must retain the controller's hook positions.
	txns := app.Transactions()
	for _, txn := range txns {
		if err := app.DeleteTransaction(txn.ID); err != nil {
			t.Fatal(err)
		}
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "button", "rpta-mode-full") != nil {
		t.Fatal("empty report retained populated controls")
	}
	for _, txn := range txns {
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if full = byTestID(f, "button", "rpta-mode-full"); full == nil || full.Attr("aria-pressed") != "true" {
		t.Fatal("populated report lost Full mode after empty snapshot")
	}
}

func TestReportsHashMountsFullBeforeScrollingAndCleansUpOnce(t *testing.T) {
	global := js.Global()
	keys := []string{"location", "document", "history", "setInterval", "clearInterval", "IntersectionObserver"}
	saved := make(map[string]js.Value, len(keys))
	for _, key := range keys {
		saved[key] = global.Get(key)
	}
	defer func() {
		for _, key := range keys {
			global.Set(key, saved[key])
		}
	}()
	location := global.Get("Object").New()
	location.Set("hash", "#rpta-04")
	location.Set("pathname", "/reports")
	location.Set("search", "?test=lazy")
	anchor := global.Get("Object").New()
	scrolls, clears := 0, 0
	scroll := js.FuncOf(func(js.Value, []js.Value) any { scrolls++; return nil })
	defer scroll.Release()
	anchor.Set("scrollIntoView", scroll)
	document := global.Get("Object").New()
	find := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].String() == "rpta-04" {
			return anchor
		}
		return nil
	})
	defer find.Release()
	document.Set("getElementById", find)
	history := global.Get("Object").New()
	replaced := ""
	replace := js.FuncOf(func(_ js.Value, args []js.Value) any {
		replaced = args[2].String()
		location.Set("hash", "")
		return nil
	})
	defer replace.Release()
	history.Set("replaceState", replace)
	var poll js.Value
	interval := js.FuncOf(func(_ js.Value, args []js.Value) any { poll = args[0]; return 17 })
	defer interval.Release()
	clear := js.FuncOf(func(js.Value, []js.Value) any { clears++; return nil })
	defer clear.Release()
	global.Set("location", location)
	global.Set("document", document)
	global.Set("history", history)
	global.Set("setInterval", interval)
	global.Set("clearInterval", clear)
	global.Set("IntersectionObserver", js.Undefined())
	f, _ := renderLazyReport(t)
	if f.ByID("rpta-04") == nil || byTestID(f, "button", "rpta-mode-full").Attr("aria-pressed") != "true" {
		t.Fatal("section hash did not mount Full")
	}
	if poll.Type() != js.TypeFunction {
		t.Fatal("section hash did not schedule target polling")
	}
	poll.Invoke()
	if scrolls != 1 || clears != 1 || replaced != "/reports?test=lazy" {
		t.Fatalf("hash completion scrolls=%d clears=%d url=%q", scrolls, clears, replaced)
	}
	byTestID(f, "button", "rpta-mode-summary").Click()
	f.Stabilize()
	if clears != 1 {
		t.Fatalf("effect cleanup cleared a finished poll twice: %d", clears)
	}
	if byTestID(f, "div", "rpta-summary") == nil {
		t.Fatal("cleared hash reopened Full after selecting Summary")
	}
}

func TestReportsCategoryRowsRefreshExplicitDisplayContext(t *testing.T) {
	f := render.New(t)
	rows := []reports.CategorySpend{{CategoryID: "groceries", Amount: 12345}}
	review, err := reportview.BuildCategoryReview(rows, nil, nil, false, 3, 12345)
	if err != nil {
		t.Fatal(err)
	}
	props := reportCategoriesProps{Context: reportSectionContext{Base: "USD", Language: "en", Accent: "#abc"}, Review: review, Names: map[string]string{"groceries": "Groceries"}}
	f.Render(ui.CreateElement(reportCategoriesSection, props))
	if !strings.Contains(f.Text(), currency.Symbol("USD")+"123.45") {
		t.Fatalf("missing initial amount: %s", f.Text())
	}
	// Same numeric readings and callback code pointers; only display context changes.
	props.Context.Base = "EUR"
	f.Rerender(ui.CreateElement(reportCategoriesSection, props))
	if !strings.Contains(f.Text(), currency.Symbol("EUR")+"123.45") || strings.Contains(f.Text(), currency.Symbol("USD")+"123.45") {
		t.Fatalf("currency context retained stale row formatter: %s", f.Text())
	}
}

func TestReportsGoalCountsAndCoverageUseCanonicalFX(t *testing.T) {
	f, app := renderLazyReport(t)
	goal := domain.Goal{ID: "report-fx-goal", Name: "Foreign reserve", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		CurrentAmount: money.New(9000, "USD"), TargetAmount: money.New(10000, "USD"), TargetDate: time.Now().AddDate(0, -1, 0),
		Allocations: []domain.GoalAllocation{{AccountID: "yen", Amount: money.New(1000, "JPY")}}}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "p", "rpta-goals-unavailable") == nil {
		t.Fatal("Summary omitted unknown-FX goal disclosure")
	}
	byTestID(f, "button", "rpta-mode-full").Click()
	f.Stabilize()
	row := byTestID(f, "div", "rpta-goal-row")
	if row == nil || !strings.Contains(row.Text(), uistate.T("goals.coverageUnavailableShort")) || strings.Contains(row.Text(), uistate.T("rpta.goalStateReached")) || strings.Contains(row.Text(), uistate.T("rpta.goalStateMissed")) {
		t.Fatalf("unknown goal coverage was treated as known completion/deadline state: %v", row)
	}
	if summary := byTestID(f, "p", "rpta-goals-summary"); summary == nil || !strings.Contains(summary.Text(), uistate.T("rpta.goalsSummary", 0, 1, 0)) {
		t.Fatal("unknown-FX goal did not stay Current")
	}
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = map[string]float64{"JPY": .005}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	row = byTestID(f, "div", "rpta-goal-row")
	if row == nil || !strings.Contains(row.Text(), "95%") || !strings.Contains(row.Text(), "95.00") || !strings.Contains(row.Text(), uistate.T("rpta.goalStateMissed")) {
		t.Fatalf("goal coverage did not use converted reserves: %v", row)
	}
	if byTestID(f, "p", "rpta-goals-unavailable") != nil {
		t.Fatal("known-FX goal retained unavailable notice")
	}
	settings.FXRates["JPY"] = .01
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if summary := byTestID(f, "p", "rpta-goals-summary"); summary == nil || !strings.Contains(summary.Text(), uistate.T("rpta.goalsSummary", 1, 0, 0)) {
		t.Fatal("completed goal tally retained prior FX state")
	}
}

func TestReportsMoneyFlowUsesCheckedPer100AndDisclosesOverflow(t *testing.T) {
	f := render.New(t)
	props := reportMoneyFlowProps{Context: reportSectionContext{Base: "USD", Accent: "#2e8b57"}, Flow: reports.PeriodFlow{Income: math.MaxInt64, Expense: 1}}
	f.Render(ui.CreateElement(reportMoneyFlowSection, props))
	table := byTestID(f, "table", "rpta-per100")
	if table == nil || !strings.Contains(table.Text(), "$99.90") {
		t.Fatalf("large valid savings share wrapped: %v", table)
	}
	props.Flow = reports.PeriodFlow{Income: 1, Expense: math.MaxInt64}
	props.Spending = []reports.CategorySpend{{CategoryID: "expense", Amount: math.MaxInt64}}
	f.Rerender(ui.CreateElement(reportMoneyFlowSection, props))
	if byTestID(f, "p", "rpta-per100-unavailable") == nil || byTestID(f, "table", "rpta-per100") != nil {
		t.Fatal("unrepresentable per-100 share looked like a valid empty table")
	}
}
