//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// trendBuckets is how many consecutive periods the cash-flow trend spans.
const trendBuckets = 6

// Reports is the read-only reporting screen (B21): for the period chosen in the
// top bar it shows income / expense / net, a plain-English summary, and spending
// by category compared to the prior period — all from the pure internal/reports
// core, so the figures match the rest of the app. Charts come in a follow-up.
func Reports() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()

	// The viewed period is the shared top-bar window; the comparison is the
	// immediately preceding window of the same length.
	w := uistate.UsePeriod().Get()
	cs, ce := w.Range()
	ps, pe := w.Shift(-1).Range()

	flow, _ := reports.IncomeVsExpense(txns, cs, ce, rates)
	rows, _ := reports.SpendingByCategory(txns, cs, ce, true, ps, pe, rates)

	// Cash-flow trend: net for each of the last trendBuckets periods of the
	// viewed resolution, ending with the current one.
	pr := uistate.UsePrefs().Get()
	weekStart := pr.WeekStartWeekday()
	startCur := period.Truncate(w.Res, w.From, weekStart)
	bounds := make([]time.Time, 0, trendBuckets+1)
	for k := 0; k <= trendBuckets; k++ {
		bounds = append(bounds, period.Step(w.Res, startCur, k-(trendBuckets-1)))
	}
	flows, _ := reports.IncomeExpenseSeries(txns, bounds, rates)
	netSeries := make([]float64, len(flows))
	for i, f := range flows {
		netSeries[i] = float64(f.Net())
	}

	// Net-worth trend: net worth as of each period boundary (cumulative, so it
	// reads the running total rather than per-period flow).
	accounts := app.Accounts()
	nwSeries, _ := ledger.NetWorthSeries(accounts, txns, bounds, rates)
	nw := make([]float64, len(nwSeries))
	for i, m := range nwSeries {
		nw[i] = float64(m.Amount)
	}

	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	nameOf := func(id string) string {
		if n := catName[id]; n != "" {
			return n
		}
		return uistate.T("reports.uncategorized")
	}
	fmtMinor := func(v int64) string { return fmtMoney(money.New(v, base)) }
	narrative := reports.SpendingNarrative(rows, true, fmtMinor, func(id string) string { return catName[id] })

	// Category rows are plain text (no interactive controls), so building them in
	// a loop is safe (no On* hooks involved).
	var rowNodes []ui.Node
	for _, r := range rows {
		if r.Amount == 0 && r.Prior == 0 {
			continue
		}
		delta := Fragment()
		if r.HasDelta && r.Amount != r.Prior {
			// Spending up is red (worse), down is green (better).
			tone, arrow := "text-down", "▲"
			if r.DeltaPct < 0 {
				tone, arrow = "text-up", "▼"
			}
			pct := r.DeltaPct
			if pct < 0 {
				pct = -pct
			}
			delta = Span(Class("row-meta "+tone), fmt.Sprintf("%s %d%%", arrow, pct))
		}
		rowNodes = append(rowNodes, Div(Class("row"),
			Div(Class("row-main"), Span(Class("row-desc"), nameOf(r.CategoryID))),
			delta,
			Span(Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	var catBody ui.Node
	if len(rowNodes) == 0 {
		catBody = P(Class("empty"), uistate.T("reports.empty"))
	} else {
		catBody = Div(Class("rows"), rowNodes)
	}

	// Top payees: where the money went by merchant/description this period.
	payees, _ := reports.TopPayees(txns, cs, ce, rates, 8)
	var payeeNodes []ui.Node
	for _, p := range payees {
		name := p.Name
		if name == "" {
			name = uistate.T("reports.noPayee")
		}
		payeeNodes = append(payeeNodes, Div(Class("row"),
			Div(Class("row-main"), Span(Class("row-desc"), name)),
			Span(Class("budget-amount"), fmtMinor(p.Amount)),
		))
	}

	// Biggest individual expenses this period.
	largest, _ := reports.LargestExpenses(txns, cs, ce, rates, 8)
	var largestNodes []ui.Node
	for _, e := range largest {
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		largestNodes = append(largestNodes, Div(Class("row"),
			Div(Class("row-main"),
				Span(Class("row-desc"), desc),
				Span(Class("row-meta"), pr.FormatDate(e.Date)),
			),
			Span(Class("budget-amount"), fmtMinor(e.Amount)),
		))
	}

	net := money.New(flow.Net(), base)
	return Div(
		Div(Class("stat-grid"),
			stat(uistate.T("dashboard.income"), fmtMoney(money.New(flow.Income, base)), "pos"),
			stat(uistate.T("dashboard.spending"), fmtMoney(money.New(flow.Expense, base)), "neg"),
			stat(uistate.T("reports.net"), fmtMoney(net), accentFor(net)),
			stat(uistate.T("dashboard.savingsRate"), fmt.Sprintf("%d%%", flow.SavingsRate()), ""),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.byCategory")),
			P(Class("muted"), narrative),
			catBody,
			If(len(rowNodes) > 0, Div(Class("flex flex-wrap gap-2 py-1"),
				Button(Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("spending-by-category.csv", "text/csv", reports.CategoryCSV(rows, nameOf, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			)),
		),
		If(len(payeeNodes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.topPayees")),
			Div(Class("rows"), payeeNodes),
		)),
		If(len(largestNodes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.biggestExpenses")),
			Div(Class("rows"), largestNodes),
		)),
		If(len(netSeries) >= 2, Section(Class("card"),
			H2(Class("card-title"), uistate.T("dashboard.cashFlow")),
			P(Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
			uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, GradientID: "cf-reports", Label: uistate.T("dashboard.cashFlow")}),
		)),
		If(len(nw) >= 2, Section(Class("card"),
			H2(Class("card-title"), uistate.T("dashboard.netWorthTrend")),
			uiw.AreaChart(uiw.AreaChartProps{Values: nw, Stroke: "#7c83ff", GradientID: "nw-reports", Label: uistate.T("dashboard.netWorthTrend")}),
		)),
	)
}
