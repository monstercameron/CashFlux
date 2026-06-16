//go:build js && wasm

package screens

import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Dashboard shows headline metrics in the candidate-C bento grid, driven by the
// live store and the shared time-resolution window.
func Dashboard() ui.Node {
	app := appstate.Default
	if app == nil {
		return Div(Class("bento"), Div(Class("w"), Div(Class("wbody"), P(Class("empty"), "App state is not ready yet."))))
	}

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	net, assets, liabilities, _ := ledger.NetWorth(accounts, txns, rates)
	w := uistate.UsePeriod().Get()
	start, end := w.Range()
	income, expense, _ := ledger.PeriodTotals(txns, start, end, rates)
	periodLabel := w.FromLabel()
	if w.ToLabel() != w.FromLabel() {
		periodLabel += " – " + w.ToLabel()
	}

	active := 0
	for _, a := range accounts {
		if !a.Archived {
			active++
		}
	}

	return Div(Class("bento"),
		dashboardHeaderCell(),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-networth", Title: "Net worth", Draggable: true, Resizable: true,
			GridColumn: "1", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(net), figTone(net), fmt.Sprintf("Assets %s", fmtAccounting(assets)), "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-income", Title: "Income", Draggable: true,
			GridColumn: "2", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(income), "text-up", periodLabel, "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-spending", Title: "Spending", Draggable: true,
			GridColumn: "3", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(expense), "text-down", periodLabel, "text-dim"),
		}),
		uiw.Widget(uiw.WidgetProps{
			ID: "kpi-liabilities", Title: "Liabilities", Draggable: true,
			GridColumn: "4", GridRow: "2", BodyClass: "flex flex-col justify-center kpi",
			Body: kpiBody(fmtAccounting(liabilities), "", fmt.Sprintf("%d accounts", active), "text-dim"),
		}),
	)
}

// dashboardHeaderCell is the full-width intro cell at the top of the bento grid.
func dashboardHeaderCell() ui.Node {
	return Div(Class("w"), Style(map[string]string{"grid-column": "1 / -1", "grid-row": "1"}),
		Div(Class("flex-1 flex items-center px-5"),
			Div(
				H1(Class("font-display text-2xl font-semibold tracking-tight"), "Your dashboard"),
				P(Class("text-dim mt-0.5 text-[13px]"), "Drag tiles to move · grab the edge handles to resize"),
			),
		),
	)
}

// kpiBody renders a KPI tile's body: a large accounting figure with a small
// subline. figTone/subTone are color classes (e.g. "text-up", "text-dim").
func kpiBody(figure, figTone, subline, subTone string) ui.Node {
	return Div(
		Div(Class("font-display fig text-[24px] leading-tight "+figTone), figure),
		Div(Class("pt-1.5 text-[12px] "+subTone), subline),
	)
}

// recentTransactions returns the n most recent transactions, newest first.
func recentTransactions(txns []domain.Transaction, n int) []domain.Transaction {
	cp := append([]domain.Transaction(nil), txns...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Date.After(cp[j].Date) })
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}
