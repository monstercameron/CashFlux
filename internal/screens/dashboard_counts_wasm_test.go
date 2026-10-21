// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dashboardview"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
	"github.com/monstercameron/GoWebComponents/v5/css"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestDashboardTransactionCountsAgreeWithReportEligibleKpiCaptions(t *testing.T) {
	stubBudgetRowBrowser(t)
	doc := js.Global().Get("document")
	previousQuery := doc.Get("querySelectorAll")
	query := js.FuncOf(func(js.Value, []js.Value) any { return []any{} })
	doc.Set("querySelectorAll", query)
	defer func() { doc.Set("querySelectorAll", previousQuery); query.Release() }()
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app := agrApp(t)
	defer app.Store().Close()
	for _, id := range []string{"aa11111111111111", "cc33333333333333"} {
		txn, ok := app.TransactionByID(id)
		if !ok {
			t.Fatalf("missing fixture transaction %s", id)
		}
		txn.ExcludeFromReports = true
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutAccount(domain.Account{ID: "other", Name: "Other", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	for _, txn := range []domain.Transaction{
		{ID: "other-income", AccountID: "other", Desc: "Other salary", Date: agrDay, Amount: money.New(10000, "USD")},
		{ID: "other-expense", AccountID: "other", Desc: "Other expense", Date: agrDay, Amount: money.New(-2000, "USD")},
	} {
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	var observed dashboardview.TransactionCounts
	ready := make(chan struct{}, 1)
	widgetrender.RegisterFrame("recent", func(frame domain.Frame, ctx widgetrender.RenderCtx) ui.Node {
		observed = dashboardview.TransactionCounts{Income: ctx.IncCount, Expense: ctx.ExpCount}
		select {
		case ready <- struct{}{}:
		default:
		}
		return recentFrame(frame, ctx)
	})
	defer widgetrender.RegisterFrame("recent", recentFrame)
	f := render.New(t)
	defer f.Cleanup()
	var items state.Atom[[]dashlayout.Item]
	var configs state.Atom[uistate.WidgetConfigs]
	var hidden state.Atom[widgetvis.Set]
	var lens state.Atom[scope.ReportScope]
	var window state.Atom[period.Window]
	var mount func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		items, configs, hidden = uistate.UseLayoutItems(), uistate.UseWidgetConfigs(), uistate.UseHiddenWidgets()
		lens, window = uistate.UseActiveScope(), uistate.UsePeriod()
		mounted := ui.UseState(false)
		mount = mounted.Set
		if mounted.Get() {
			return ui.CreateElement(Dashboard)
		}
		return nil
	}))
	oldItems, oldConfigs, oldHidden, oldLens, oldWindow := items.Get(), configs.Get(), hidden.Get(), lens.Get(), window.Get()
	defer func() {
		mount(false)
		items.Set(oldItems)
		configs.Set(oldConfigs)
		hidden.Set(oldHidden)
		lens.Set(oldLens)
		window.Set(oldWindow)
	}()
	items.Set([]dashlayout.Item{{ID: "kpi-income", ColSpan: 1, RowSpan: 1}, {ID: "kpi-spending", ColSpan: 1, RowSpan: 1}, {ID: "recent", ColSpan: 2, RowSpan: 2}})
	configs.Set(uistate.WidgetConfigs{})
	hidden.Set(widgetvis.Set{})
	lens.Set(scope.ReportScope{})
	window.Set(period.NewWindow(period.Month, agrDay, time.Monday))
	mount(true)
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("Dashboard did not finish deferred KPI render")
	}
	f.Stabilize()
	widgetText := func(id string) string {
		for _, node := range f.AllByTag("div") {
			if node.Attr("data-widget") == id {
				return node.Text()
			}
		}
		return ""
	}
	assertCounts := func(income, expense int, incomeLabel, expenseLabel string) {
		t.Helper()
		if observed != (dashboardview.TransactionCounts{Income: income, Expense: expense}) {
			t.Fatalf("context counts=%+v want income=%d expense=%d", observed, income, expense)
		}
		if text := widgetText("kpi-income"); !strings.Contains(text, incomeLabel) {
			t.Fatalf("income KPI caption=%q want %q", text, incomeLabel)
		}
		if text := widgetText("kpi-spending"); !strings.Contains(text, expenseLabel) {
			t.Fatalf("spending KPI caption=%q want %q", text, expenseLabel)
		}
	}
	assertCounts(2, 4, "2 deposits", "4 expenses")
	lens.Set(scope.ReportScope{AccountIDs: []string{"a-check"}})
	f.Stabilize()
	assertCounts(1, 3, "1 deposit", "3 expenses")
	txn, _ := app.TransactionByID("cc33333333333333")
	txn.ExcludeFromReports = false
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertCounts(1, 4, "1 deposit", "4 expenses")
	window.Set(period.NewWindow(period.Month, agrDay.AddDate(0, 1, 0), time.Monday))
	f.Stabilize()
	assertCounts(0, 0, "0 deposits", "0 expenses")
}
