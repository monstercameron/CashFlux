// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/reportview"
	"github.com/monstercameron/CashFlux/internal/scope"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// reportSectionContext carries data and display identities explicitly. Callback
// code pointers alone cannot identify a new base currency, scope or translation.
type reportSectionContext struct {
	Revision, ScopeKey, Base, Language, Accent string
	From, To                                   time.Time
	Prefs                                      prefs.Prefs
}

type reportSectionActions struct {
	ToggleRollup ui.Handler
	ChangeCFKey  ui.Handler
}

func (c reportSectionContext) formatMinor(value int64) string {
	return fmtMoney(money.New(value, c.Base))
}
func (c reportSectionContext) categoryName(names map[string]string) func(string) string {
	return func(id string) string {
		if name := names[id]; name != "" {
			return name
		}
		return uistate.T("reports.uncategorized")
	}
}

// reportSectionBuildObserver is a nil-by-default instrumentation seam for
// focused render tests. It counts actual builder invocations, not DOM presence.
var reportSectionBuildObserver func(string)

func reportSectionBuilt(section string) {
	if reportSectionBuildObserver != nil {
		reportSectionBuildObserver(section)
	}
}

type reportMoneyFlowProps struct {
	Context          reportSectionContext
	Flow             reports.PeriodFlow
	Income, Spending []reports.CategorySpend
	Names            map[string]string
}

func reportFlowPresentation(p reportMoneyFlowProps) (reports.MoneyFlowDiagram, *rptaFlowColors) {
	incomeRows, rows, flow := p.Income, p.Spending, p.Flow
	nameOf := p.Context.categoryName(p.Names)
	accent := p.Context.Accent
	flowColors := newRptaFlowColors(accent)
	// The diagram's DATA — which nodes exist, what each is worth, and which
	// categories sit behind it — is built in the pure reports package, so the
	// assistant's money_flow tool reads the very ribbons this section draws and
	// can trace each one back to its transactions. The view only picks colors
	// and pixels. (Sources → Income → categories/savings; a category named
	// exactly like the hub is disambiguated there, otherwise it forms a From==To
	// self-loop the layout drops and the money vanishes from the picture while
	// the headline still counts it — Cam's UKG paychecks, 2026-07-19.)
	diagram := reports.BuildMoneyFlow(reports.MoneyFlowInputs{
		Income:   incomeRows,
		Spending: rows,
		Net:      flow.Net(),
		Name:     nameOf,
		Labels: reports.MoneyFlowLabels{
			Income:         uistate.T("rpta.nodeIncome"),
			OtherIncome:    uistate.T("rpta.nodeOtherIncome"),
			EverythingElse: uistate.T("rpta.nodeEverythingElse"),
			Savings:        uistate.T("rpta.nodeSavings"),
			FromSavings:    uistate.T("rpta.nodeFromSavings"),
			Disambiguate:   func(n string) string { return uistate.T("rpta.nodeCatDisamb", n) },
		},
		TopSources:  rptaFlowTopSources,
		TopSpending: rptaFlowTopCategories,
	})
	for _, n := range diagram.Nodes {
		switch n.Kind {
		case reports.FlowIncomeHub:
			flowColors.hub(n.Label)
		case reports.FlowIncomeSource:
			flowColors.source(n.Label)
		case reports.FlowSpendCategory:
			flowColors.category(n.Label)
		case reports.FlowSavings:
			flowColors.savings(n.Label)
		case reports.FlowFromSavings:
			flowColors.deficit(n.Label)
		default: // the pooled "other income" / "everything else" nodes
			flowColors.rest(n.Label)
		}
	}
	return diagram, flowColors
}

func reportMoneyFlowSection(p reportMoneyFlowProps) ui.Node {
	reportSectionBuilt("moneyflow")
	base, as, ae := p.Context.Base, p.Context.From, p.Context.To
	incomeRows, rows, flow := p.Income, p.Spending, p.Flow
	nameOf, fmtMinor := p.Context.categoryName(p.Names), p.Context.formatMinor
	decimals, accent := currency.Decimals(base), p.Context.Accent
	per100, per100Err := reports.Per100Checked(rows, flow.Income, 10)
	// ── 02 · The flow of money (in-house smooth-ribbon sankey + per-$100). ───
	sankeyFactor := int64(1)
	for i := 0; i < decimals; i++ {
		sankeyFactor *= 10
	}
	diagram, flowColors := reportFlowPresentation(p)
	moneyFlows := diagram.Flows()
	srcTotal := 0
	for _, r := range incomeRows {
		if r.Amount > 0 {
			srcTotal++
		}
	}
	per100Dot := func(color string) ui.Node {
		return Span(css.Class("rpta-flow-dot", "rpta-cat-dot"), Attr("aria-hidden", "true"), Style(map[string]string{"background": color}))
	}
	var per100Rows []ui.Node
	for _, p := range per100 {
		label := nameOf(p.CategoryID)
		if p.CategoryID == "" {
			label = uistate.T("rpta.nodeEverythingElse")
		}
		per100Rows = append(per100Rows, Tr(
			Td(css.Class("rpta-td-name"), per100Dot(flowColors.of(label)), label),
			Td(css.Class("rpta-td-num"), fmt.Sprintf("$%d.%d0", p.Per100, p.Tenths)),
			Td(css.Class("rpta-td-num", "rpta-muted"), fmtMinor(p.AmountMinor)),
		))
	}
	whole, tenths, hasSavings, savingsErr := reports.PositiveSavingsPer100(flow.Income, flow.Expense)
	if hasSavings && savingsErr == nil {
		per100Rows = append(per100Rows, Tr(css.Class("rpta-tr-kept"),
			Td(css.Class("rpta-td-name"), per100Dot("#4ea777"), uistate.T("rpta.nodeSavings")),
			Td(css.Class("rpta-td-num"), fmt.Sprintf("$%d.%d0", whole, tenths)),
			Td(css.Class("rpta-td-num", "rpta-muted"), fmtMinor(flow.Net()))))
	}
	var per100Table ui.Node = Table(css.Class("rpta-table", "rpta-per100"), Attr("data-testid", "rpta-per100"),
		Thead(Tr(Th(uistate.T("rpta.per100Where")), Th("/$100"), Th(uistate.T("rpta.per100Year")))), Tbody(per100Rows))
	if per100Err != nil || savingsErr != nil {
		per100Table = P(css.Class("rpta-muted"), Attr("role", "status"), Attr("data-testid", "rpta-per100-unavailable"), uistate.T("rpta.activityIncomplete"))
	}

	// srcTotal, not srcCount — the render cap (5 named ribbons) used to leak into
	// the caption, which claimed "from 5 sources" no matter how many there were.
	askFlow := fmt.Sprintf("income %s from %d sources; spending %s; savings %s (%d%% of income)",
		fmtMinor(flow.Income), srcTotal, fmtMinor(flow.Expense), fmtMinor(flow.Net()), flow.SavingsRate())
	if len(rows) > 0 {
		askFlow += fmt.Sprintf("; biggest category %s at %s", nameOf(rows[0].CategoryID), fmtMinor(absMinor(rows[0].Amount)))
	}
	return rptaSection("rpta-02", "02", uistate.T("reports.moneyFlow"), "up", uistate.T("rpta.secFlowSub"), askFlow, Fragment(
		If(len(moneyFlows) > 1, Div(css.Class("rpta-sankey"),
			rptaMoneyFlowSVG(moneyFlows, flowColors, flow.Income, fmtMinor, sankeyFactor, currency.Symbol(base)),
			Div(css.Class("rpta-flow-key"),
				Span(css.Class("rpta-flow-key-item"), Span(css.Class("rpta-flow-dot"), Style(map[string]string{"background": rptaSrcPalette[0]})), uistate.T("rpta.keySources")),
				Span(css.Class("rpta-flow-key-item"), Span(css.Class("rpta-flow-dot"), Style(map[string]string{"background": accent})), uistate.T("rpta.nodeIncome")),
				Span(css.Class("rpta-flow-key-item"), Span(css.Class("rpta-flow-dot"), Style(map[string]string{"background": rptaCatPalette[0]})), uistate.T("rpta.keyCats")),
				Span(css.Class("rpta-flow-key-item"), Span(css.Class("rpta-flow-dot"), Style(map[string]string{"background": "#4ea777"})), uistate.T("rpta.nodeSavings")),
				Span(css.Class("rpta-flow-key-note"), uistate.T("rpta.keyWidth")),
			))),
		Div(css.Class("rpta-flow-side"),
			Div(css.Class("rpta-subhead"), uistate.T("rpta.per100Head")),
			per100Table,
			ui.CreateElement(rptaDrill, rptaDrillProps{Label: uistate.T("reports.viewTransactions"), TestID: "moneyflow-drill", From: as, To: ae}),
		),
	))

}

type reportMotionProps struct {
	// InProgress is the prepared month-label index; unrelated sections need no clock.
	InProgress int
	Context    reportSectionContext
	Series     reportview.Series
	Activity   reportview.Activity
	Bounds     []time.Time
}

func reportMotionSection(p reportMotionProps) ui.Node {
	reportSectionBuilt("motion")
	base, bounds := p.Context.Base, p.Bounds
	months := len(bounds) - 1
	flow, monthFlows, srInts := p.Series.Flow, p.Series.MonthFlows, p.Series.SavingsRates
	nwSeries, nwChange, creditSeries := p.Activity.NetWorthSeries, p.Activity.NetWorthChange, p.Series.Credit
	spendStats, weekday := p.Activity.SpendStats, p.Activity.Weekday
	hiIdx, loIdx, seasonalOK := p.Activity.HighMonth, p.Activity.LowMonth, p.Activity.HasSeasonality
	monthsRed := p.Activity.MonthsNegative
	fmtMinor, accent := p.Context.formatMinor, p.Context.Accent
	sankeyFactor := int64(1)
	for i := 0; i < currency.Decimals(base); i++ {
		sankeyFactor *= 10
	}
	monthLabels := make([]string, 0, months)
	for i := 0; i < months; i++ {
		monthLabels = append(monthLabels, bounds[i].Format("Jan"))
	}
	// ── 03 · The year in motion (monthly review table + trends). ─────────────
	var monthRows []ui.Node
	for i := 0; i < months && i < len(monthFlows); i++ {
		f := monthFlows[i]
		if f.Income == 0 && f.Expense == 0 {
			continue
		}
		rate := "—"
		if len(srInts) > i && f.Income > 0 {
			rate = fmt.Sprintf("%d%%", srInts[i])
		}
		rowCls := ""
		if f.Net() < 0 {
			rowCls = "rpta-tr-red"
		}
		var keptMeter ui.Node = Fragment()
		if len(srInts) > i && f.Income > 0 {
			pct := srInts[i]
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
			keptMeter = Div(css.Class("rpta-kept-meter"), Attr("aria-hidden", "true"),
				Div(ClassStr("rpta-kept-fill"+If2(f.Net() < 0, " rpta-kept-red", "")), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
		}
		// The in-progress month says so — a half-elapsed month sitting beside
		// eleven complete ones otherwise reads as a spending collapse (parity
		// scan: label partial periods).
		nameCell := ui.CreateElement(rptaMonthDrill, rptaMonthDrillProps{
			Label:      bounds[i].Format("January 2006"),
			From:       bounds[i].Format(dateutil.Layout),
			To:         bounds[i+1].AddDate(0, 0, -1).Format(dateutil.Layout),
			InProgress: i == p.InProgress,
		})
		monthRows = append(monthRows, Tr(ClassStr(rowCls),
			Td(css.Class("rpta-td-name"), nameCell),
			Td(css.Class("rpta-td-num"), fmtMinor(f.Income)),
			Td(css.Class("rpta-td-num"), fmtMinor(f.Expense)),
			Td(ClassStr("rpta-td-num rpta-td-strong"+If2(f.Net() < 0, " rpta-tone-down", "")), fmtMinor(f.Net())),
			Td(css.Class("rpta-td-num", "rpta-td-kept"), rate, keptMeter),
		))
	}
	netSeries := make([]float64, 0, months)
	for i := 0; i < months && i < len(monthFlows); i++ {
		netSeries = append(netSeries, float64(monthFlows[i].Net()))
	}
	srSeries := make([]float64, 0, len(srInts))
	for _, v := range srInts {
		srSeries = append(srSeries, float64(v))
	}
	nwFloat := make([]float64, 0, len(nwSeries))
	for _, m := range nwSeries {
		nwFloat = append(nwFloat, float64(m.Amount))
	}
	moneyVL := func(vals []float64) []string {
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = fmtMoney(money.New(int64(v), base))
		}
		return out
	}
	pctVL := func(vals []float64) []string {
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = fmt.Sprintf("%d%%", int(v))
		}
		return out
	}
	creditFloat := make([]float64, 0, len(creditSeries))
	for _, v := range creditSeries {
		creditFloat = append(creditFloat, float64(v))
	}
	axisMoney := func(v float64) string { return rptaShortMoney(int64(v), sankeyFactor, currency.Symbol(base)) }
	axisPct := func(v float64) string { return fmt.Sprintf("%d%%", int(v)) }
	pctlessVL := func(vals []float64) []string {
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = fmt.Sprintf("%d / 100", int(v))
		}
		return out
	}
	// Each trend chart takes the color of its verdict: green when the year's
	// story is good, amber when it's borderline, red when it works against you.
	cashStroke := rptaToneUp
	if flow.Net() < 0 {
		cashStroke = rptaToneDown
	}
	srStroke := rptaToneDown
	if sr := flow.SavingsRate(); sr >= 15 {
		srStroke = rptaToneUp
	} else if sr >= 5 {
		srStroke = rptaToneWarn
	}
	nwStroke := rptaToneUp
	if nwChange < 0 {
		nwStroke = rptaToneDown
	}
	creditStroke := rptaToneDown
	if n := len(creditSeries); n > 0 {
		switch last := creditSeries[n-1]; {
		case last >= 70:
			creditStroke = rptaToneUp
		case last >= 40:
			creditStroke = rptaToneWarn
		}
	}
	seasonLine := ""
	if seasonalOK {
		seasonLine = uistate.T("rpta.seasonal", bounds[hiIdx].Format("January"), fmtMinor(monthFlows[hiIdx].Expense), bounds[loIdx].Format("January"), fmtMinor(monthFlows[loIdx].Expense))
	}
	statsLine := ""
	if spendStats.Count > 0 {
		statsLine = uistate.T("rpta.spendStats", spendStats.Count, fmtMinor(spendStats.Average), fmtMinor(spendStats.Median))
	}
	if d, ok := reports.PeakWeekday(weekday); ok {
		if statsLine != "" {
			statsLine += " · "
		}
		statsLine += uistate.T("reports.peakWeekday", d.String(), fmtMinor(weekday[d]))
	}
	// Spending-by-weekday mini bars: seven columns, the peak day toned warm.
	var weekdayChart ui.Node = Fragment()
	if peakDay, ok := reports.PeakWeekday(weekday); ok {
		var maxWD int64
		for _, v := range weekday {
			if v > maxWD {
				maxWD = v
			}
		}
		var wdCols []ui.Node
		for d := time.Sunday; d <= time.Saturday; d++ {
			v := weekday[d]
			pct := int64(0)
			if maxWD > 0 {
				pct = v * 100 / maxWD
			}
			if pct < 3 {
				pct = 3
			}
			wdCols = append(wdCols, Div(css.Class("rpta-wd-col"), Title(d.String()+": "+fmtMinor(v)),
				Div(css.Class("rpta-wd-track"),
					Div(ClassStr("rpta-wd-fill"+If2(d == peakDay, " rpta-wd-peak", "")), Style(map[string]string{"height": fmt.Sprintf("%d%%", pct)}))),
				Span(css.Class("rpta-wd-day"), d.String()[:1]),
			))
		}
		weekdayChart = Div(css.Class("rpta-weekday"), Attr("data-testid", "rpta-weekday"),
			Div(css.Class("rpta-subhead"), uistate.T("rpta.weekdayHead")),
			Div(css.Class("rpta-chart-body"),
				Div(css.Class("rpta-yaxis", "rpta-yaxis-wd"), Attr("aria-hidden", "true"),
					Span(rptaShortMoney(maxWD, sankeyFactor, currency.Symbol(base))), Span(currency.Symbol(base)+"0")),
				Div(css.Class("rpta-wd-bars"), Attr("role", "img"), Attr("aria-label", uistate.T("rpta.weekdayHead")), wdCols),
			),
			rptaChartLegend(accent, uistate.T("rpta.legendWeekday")),
		)
	}
	askMotion := seasonLine + " " + statsLine
	if len(creditSeries) > 0 {
		askMotion += fmt.Sprintf(" Credit proxy score ended at %d/100.", creditSeries[len(creditSeries)-1])
	}
	if monthsRed > 0 {
		askMotion += fmt.Sprintf(" %d months were in the red.", monthsRed)
	}
	return rptaSection("rpta-03", "03", uistate.T("rpta.secMotion"), "neutral", uistate.T("rpta.secMotionSub"), askMotion, Fragment(
		Table(css.Class("rpta-table", "rpta-months"), Attr("data-testid", "rpta-months"),
			Thead(Tr(Th(uistate.T("rpta.colMonth")), Th(uistate.T("dashboard.income")), Th(uistate.T("dashboard.spending")), Th(uistate.T("reports.net")), Th(uistate.T("rpta.colKeptPct")))),
			Tbody(monthRows),
		),
		If(seasonLine != "", P(css.Class("rpta-muted"), Attr("data-testid", "rpta-seasonal"), seasonLine)),
		If(statsLine != "", P(css.Class("rpta-muted"), statsLine)),
		weekdayChart,
		Div(css.Class("rpta-charts3"),
			If(len(netSeries) >= 2, rptaTrendChart(uistate.T("dashboard.cashFlow"), cashStroke, uistate.T("rpta.legendCash"),
				rptaSrcLink("nav.transactions", "/transactions"),
				axisMoney(seriesMax(netSeries)), axisMoney(seriesMid(netSeries)), axisMoney(seriesMin(netSeries)),
				uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, Stroke: cashStroke, GradientID: "rpta-net", Label: uistate.T("dashboard.cashFlow"), Labels: monthLabels, ValueLabels: moneyVL(netSeries)}))),
			If(len(srSeries) >= 2, rptaTrendChart(uistate.T("reports.savingsTrend"), srStroke, uistate.T("rpta.legendSR"),
				rptaSrcLink("nav.health", "/health"),
				axisPct(seriesMax(srSeries)), axisPct(seriesMid(srSeries)), axisPct(seriesMin(srSeries)),
				uiw.AreaChart(uiw.AreaChartProps{Values: srSeries, Stroke: srStroke, GradientID: "rpta-sr", Label: uistate.T("reports.savingsTrend"), Labels: monthLabels, ValueLabels: pctVL(srSeries)}))),
			If(len(nwFloat) >= 2, rptaTrendChart(uistate.T("dashboard.netWorth"), nwStroke, uistate.T("rpta.legendNW"),
				rptaSrcLink("nav.netWorth", "/networth"),
				axisMoney(seriesMax(nwFloat)), axisMoney(seriesMid(nwFloat)), axisMoney(seriesMin(nwFloat)),
				uiw.AreaChart(uiw.AreaChartProps{Values: nwFloat, Stroke: nwStroke, GradientID: "rpta-nw", Label: uistate.T("dashboard.netWorth"), Labels: monthLabels, ValueLabels: moneyVL(nwFloat)}))),
			If(len(creditSeries) >= 2, Div(Attr("data-testid", "rpta-credit-chart"), rptaTrendChart(uistate.T("detail6.creditScoreLabel"), creditStroke, uistate.T("rpta.legendCredit"),
				rptaSrcLink("nav.credit", "/credit"),
				fmt.Sprintf("%d", int(seriesMax(creditFloat))), fmt.Sprintf("%d", int(seriesMid(creditFloat))), fmt.Sprintf("%d", int(seriesMin(creditFloat))),
				uiw.AreaChart(uiw.AreaChartProps{Values: creditFloat, Stroke: creditStroke, GradientID: "rpta-credit", Label: uistate.T("detail6.creditScoreLabel"), Labels: monthLabels, ValueLabels: pctlessVL(creditFloat)})))),
		),
	))

}

type reportCategoriesProps struct {
	Context reportSectionContext
	Review  reportview.CategoryReview
	Flow    reports.PeriodFlow
	Income  []reports.CategorySpend
	Tags    []reports.TagSpend
	Names   map[string]string
	Rollup  bool
	Actions *reportSectionActions
}

func reportCategoriesSection(p reportCategoriesProps) ui.Node {
	reportSectionBuilt("categories")
	categoryReview, rows, tagSpend := p.Review, p.Review.Spending, p.Tags
	as, ae := p.Context.From, p.Context.To
	catName, nameOf, fmtMinor := p.Names, p.Context.categoryName(p.Names), p.Context.formatMinor
	nav := router.UseNavigate()
	txFilterAtom := uistate.UseTxFilter()
	drillCategory := func(categoryID string) {
		f := uistate.TxFilter{Category: categoryID}.Normalize()
		txFilterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	_, flowColors := reportFlowPresentation(reportMoneyFlowProps{Context: p.Context, Flow: p.Flow, Income: p.Income, Spending: rows, Names: p.Names})
	// ── 04 · Categories reviewed (the full-year table with sparklines). ──────
	narrative := reports.SpendingNarrative(rows, true, fmtMinor, func(id string) string { return catName[id] })
	maxCat := categoryReview.MaxMinor
	var catRows, zeroCatRows []ui.Node
	for _, row := range categoryReview.Rows {
		r := row.Spend
		node := ui.CreateElement(rptaCatRow, rptaCatRowProps{Context: p.Context,
			CategoryID: r.CategoryID, Name: nameOf(r.CategoryID),
			Dot:    flowColors.of(nameOf(r.CategoryID)),
			Amount: row.Amount, Average: row.AverageMinor, SharePercent: row.SharePercent, WidthPercent: row.WidthPercent,
			Prior: r.Prior, HasDelta: r.HasDelta, DeltaPct: r.DeltaPct, PriorZero: r.PriorZero, Spark: row.Spark,
			FmtMinor: fmtMinor, OnDrill: drillCategory,
		})
		if r.Amount == 0 {
			zeroCatRows = append(zeroCatRows, node)
		} else {
			catRows = append(catRows, node)
		}
	}
	// The year histogram: one colored magnitude bar per category (sankey-palette
	// hues for the diagram's top ten), replacing a page of airy rows. The full
	// analytic table (run rates, sparklines) folds beneath it.
	// Top 12 bars + one aggregated "everything else" row — the long tail lives
	// in the folded table, not as fifteen more sliver bars.
	var catHist []ui.Node
	for _, row := range categoryReview.Histogram {
		r := row.Spend
		id := r.CategoryID
		catHist = append(catHist, ui.CreateElement(rptaHistRow, rptaHistRowProps{Context: p.Context,
			Label: nameOf(r.CategoryID), Color: flowColors.of(nameOf(r.CategoryID)),
			Amount: row.Amount, WidthPercent: row.HistogramPercent, Meta: fmt.Sprintf("%d%%", row.SharePercent),
			HasDelta: r.HasDelta, DeltaPct: r.DeltaPct, PriorZero: r.PriorZero,
			FmtMinor: fmtMinor, OnSelect: func() { drillCategory(id) },
		}))
	}
	if categoryReview.RestMinor > 0 {
		catHist = append(catHist, ui.CreateElement(rptaHistRow, rptaHistRowProps{Context: p.Context,
			Label: uistate.T("rpta.histRest", categoryReview.RestCount), Color: "#8a8f98",
			Amount: categoryReview.RestMinor, WidthPercent: categoryReview.RestWidthPercent, Meta: fmt.Sprintf("%d%%", categoryReview.RestSharePercent),
			FmtMinor: fmtMinor,
		}))
	}
	// Tags render beside the categories on the SAME spending scale (maxCat), so
	// a tag bar and a category bar are directly comparable.
	var tagNodes []ui.Node
	tagRows, _ := reportview.BuildTagRows(tagSpend, maxCat)
	for i, ts := range tagRows {
		if i >= 10 {
			break
		}
		tagNodes = append(tagNodes, Div(Attr("data-testid", "rpta-tag-row"), ui.CreateElement(rptaHistRow, rptaHistRowProps{Context: p.Context,
			Label: "#" + ts.Tag, Chip: true,
			Color:  rptaCatPalette[i%len(rptaCatPalette)],
			Amount: ts.Amount, WidthPercent: ts.WidthPercent, Meta: uistate.T("rpta.tagCharges", ts.Count),
			HasDelta: ts.HasDelta, DeltaPct: ts.DeltaPercent, PriorZero: ts.PriorZero,
			FmtMinor: fmtMinor,
		})))
	}
	// The roll-up control is a state TOGGLE (aria-pressed), so it wears the
	// page's toggle component — strip-toggle with is-on chrome — not a plain btn
	// whose pressed state would be invisible.
	rollupCls := "strip-toggle"
	if p.Rollup {
		rollupCls += " is-on"
	}
	catActions := Div(css.Class(tw.Flex, tw.Gap2),
		Button(ClassStr(rollupCls+" "+tw.Fold(tw.Gap2)), Type("button"), Attr("data-testid", "reports-rollup-toggle"),
			Attr("aria-pressed", boolStr(p.Rollup)), Title(uistate.T("reports.rollupTitle")),
			OnClick(p.Actions.ToggleRollup),
			uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T(rollupLabelKey(p.Rollup)))),
	)
	return rptaSectionWithAction("rpta-04", "04", uistate.T("rpta.secCats"), "neutral", uistate.T("rpta.secCatsSub"), narrative, catActions, Fragment(
		P(css.Class("rpta-narrative", tw.FontDisplay), narrative),
		Div(css.Class("rpta-hist-scale"), uistate.T("rpta.histScale", fmtMinor(maxCat))),
		Div(css.Class("rpta-hist"), Attr("data-testid", "rpta-cat-hist"), catHist),
		If(len(tagNodes) > 0, Fragment(
			rptaSubG("↘", "down", uistate.T("rpta.byTag"), ui.CreateElement(rptaDrill, rptaDrillProps{Label: uistate.T("reports.viewTransactions"), TestID: "tags-drill", From: as, To: ae})),
			Div(css.Class("rpta-hist-scale"), uistate.T("rpta.histScaleSame")),
			Div(css.Class("rpta-hist"), tagNodes),
			P(css.Class("rpta-muted", "rpta-tag-note"), uistate.T("rpta.tagOverlapNote")))),
		Details(css.Class("rpta-zeroed"), Attr("data-testid", "rpta-cat-table"),
			Summary(uistate.T("rpta.histTableFold")),
			Div(css.Class("rpta-cat-head"),
				Span(css.Class("rpta-cat-h-name"), uistate.T("reports.viewCategories")),
				Span(css.Class("rpta-cat-h"), uistate.T("rpta.colYear")),
				Span(css.Class("rpta-cat-h"), uistate.T("rpta.colPerMonth")),
				Span(css.Class("rpta-cat-h", "rpta-cat-h-spark"), uistate.T("rpta.colTrend")),
				Span(css.Class("rpta-cat-h"), uistate.T("rpta.colVsPrior")),
				Span(css.Class("rpta-cat-h"), uistate.T("rpta.colShare")),
			),
			Div(css.Class("rpta-cat-rows"), catRows),
			If(len(zeroCatRows) > 0, Details(css.Class("rpta-zeroed"), Attr("data-testid", "reports-zeroed"),
				Summary(uistate.T("reports.zeroedSummary", len(zeroCatRows))),
				Div(css.Class("rpta-cat-rows"), zeroCatRows))),
		),
	))

}

type reportWhereProps struct {
	Context  reportSectionContext
	Activity reportview.Activity
	Names    map[string]string
	Members  []domain.Member
}

func reportWhereSection(p reportWhereProps) ui.Node {
	reportSectionBuilt("where")
	activity, pr := p.Activity, p.Context.Prefs
	as, ae := p.Context.From, p.Context.To
	payees, largest, incomeRows := activity.Payees, activity.LargestExpenses, activity.Income
	memberSpend, tagSpend, trendByPayee := activity.Members, activity.Tags, activity.PayeeTrends
	nameOf, fmtMinor := p.Context.categoryName(p.Names), p.Context.formatMinor
	// ── 05 · Where it actually goes (payees, biggest, deposits, sources, members).
	//
	// This section is deliberately QUIET: short typographic ranked lists — name,
	// muted meta, right-aligned amount — with no bars at all. A first pass gave
	// every row a bar and it read as noise (Cam: "too visually noisy"); ordering
	// plus tabular numerals already carry the ranking. Repeated identical
	// deposits (eight "Paycheck (net) $4,700" rows) collapse to one line with a
	// count. The tag distribution lives in §04 beside the category histogram,
	// where the shared spending scale makes its bars meaningful.
	listRows := func(items []ui.Node) ui.Node { return Div(css.Class("rows"), items) }
	quietRow := func(desc, meta string, amount int64) ui.Node {
		return Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), desc), If(meta != "", Span(css.Class("row-meta"), meta))),
			Span(css.Class("budget-amount"), fmtMinor(amount)))
	}
	var payeeNodes []ui.Node
	for i, p := range payees {
		if i >= 6 {
			break
		}
		nm := p.Name
		if nm == "" {
			nm = uistate.T("reports.noPayee")
		}
		// C386: every MARK drills, not just the section. A reader looking at
		// "Kroger $1,240" wants those transactions, and sending them to the whole
		// window's list makes them re-filter by hand for something the chart
		// already knew.
		payeeNodes = append(payeeNodes, Div(css.Class("row"),
			Div(css.Class("row-main"),
				ui.CreateElement(rptaDrill, rptaDrillProps{
					Label: nm, TestID: "rpta-payee-drill-" + strconv.Itoa(i),
					From: as, To: ae, Text: p.Name, ExpenseOnly: true,
					Class: "rpta-drill rpta-row-drill",
				})),
			Span(css.Class("rpta-payee-spark"), Attr("data-testid", "rpta-payee-spark-"+strconv.Itoa(i)),
				sparklineSVG(trendByPayee[strings.ToLower(strings.TrimSpace(p.Name))],
					uistate.T("rpta.payeeSparkAlt", nm))),
			Span(css.Class("budget-amount"), fmtMinor(p.Amount))))
	}
	var largestNodes []ui.Node
	for i, e := range largest {
		if i >= 6 {
			break
		}
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		largestNodes = append(largestNodes, quietRow(desc, pr.FormatDate(e.Date), e.Amount))
	}
	// Deposits: fold repeats of the same description + amount into one row.
	depGroups := reportview.GroupDeposits(activity.LargestIncome, nameOf)
	var depositNodes []ui.Node
	for i, g := range depGroups {
		if i >= 4 {
			break
		}
		meta := pr.FormatDate(g.Last)
		if g.Count > 1 {
			meta = uistate.T("rpta.depTimes", g.Count)
		}
		depositNodes = append(depositNodes, quietRow(g.Description, meta, g.Amount))
	}
	var srcNodes []ui.Node
	for _, r := range incomeRows {
		if r.Amount == 0 {
			continue
		}
		srcNodes = append(srcNodes, quietRow(nameOf(r.CategoryID), "", r.Amount))
	}
	memberName := make(map[string]string, len(p.Members))
	for _, m := range p.Members {
		memberName[m.ID] = m.Name
	}
	var memberNodes []ui.Node
	for _, ms := range memberSpend {
		nm := memberName[ms.MemberID]
		if nm == "" {
			nm = uistate.T("reports.noMember")
		}
		memberNodes = append(memberNodes, quietRow(nm, "", ms.Amount))
	}
	askWhere := ""
	if len(payees) > 0 {
		askWhere += fmt.Sprintf("top payee %s at %s; ", payees[0].Name, fmtMinor(payees[0].Amount))
	}
	if len(largest) > 0 {
		askWhere += fmt.Sprintf("largest single expense %s at %s; ", largest[0].Desc, fmtMinor(largest[0].Amount))
	}
	if len(tagSpend) > 0 {
		askWhere += fmt.Sprintf("most-spent tag #%s at %s", tagSpend[0].Tag, fmtMinor(tagSpend[0].Amount))
	}
	return rptaSection("rpta-05", "05", uistate.T("rpta.secWhere"), "neutral", uistate.T("rpta.secWhereSub"), askWhere, Fragment(Div(css.Class("rpta-cols2"),
		Div(css.Class("rpta-col"),
			rptaSubG("↘", "down", uistate.T("reports.topPayees"), ui.CreateElement(rptaDrill, rptaDrillProps{Label: uistate.T("reports.viewTransactions"), TestID: "payees-drill", From: as, To: ae, ExpenseOnly: true})),
			listRows(payeeNodes),
			rptaSubG("↗", "up", uistate.T("reports.biggestDeposits"), nil),
			listRows(depositNodes),
			If(len(p.Members) >= 2 && len(memberNodes) > 0, Fragment(
				rptaSubG("↘", "down", uistate.T("reports.byMember"), nil),
				listRows(memberNodes))),
		),
		Div(css.Class("rpta-col"),
			rptaSubG("↘", "down", uistate.T("reports.biggestExpenses"), ui.CreateElement(rptaDrill, rptaDrillProps{Label: uistate.T("reports.viewTransactions"), TestID: "expenses-drill", From: as, To: ae, ExpenseOnly: true})),
			listRows(largestNodes),
			rptaSubG("↗", "up", uistate.T("reports.incomeBySource"), ui.CreateElement(rptaDrill, rptaDrillProps{Label: uistate.T("reports.viewTransactions"), TestID: "income-drill", From: as, To: ae, IncomeOnly: true})),
			listRows(srcNodes),
		),
	)))

}

type reportAppendixProps struct {
	Context       reportSectionContext
	Accounts      []domain.Account
	Transactions  []domain.Transaction
	Categories    []domain.Category
	Definitions   []customfields.Def
	SelectedCFKey string
	Actions       *reportSectionActions
	Rates         currency.Rates
}

func reportAppendixSection(p reportAppendixProps) ui.Node {
	reportSectionBuilt("appendix")
	onCFKeyChange := OnChange(p.Actions.ChangeCFKey)
	base, as, ae := p.Context.Base, p.Context.From, p.Context.To
	scopedAccounts, scopedTxns, cats, cfDefs, rates := p.Accounts, p.Transactions, p.Categories, p.Definitions, p.Rates
	fmtMinor := p.Context.formatMinor
	// ── 09 · Appendix (tax, custom fields, metrics). ──────────────────────────
	winForExports := period.Window{Res: period.Year, From: as}
	deductible := deductibleSection(p.Context, scopedTxns, cats, as, ae, rates, base, fmtMinor, winForExports)
	var appendixBits []ui.Node
	if deductible != nil {
		appendixBits = append(appendixBits, deductible)
	}
	if invperf := investmentPerformanceSection(p.Context, scopedAccounts, scopedTxns, rates, base, fmtMinor, winForExports); invperf != nil {
		appendixBits = append(appendixBits, invperf)
	}
	if len(cfDefs) > 0 {
		appendixBits = append(appendixBits, customFieldSpendSection(p.Context, scopedTxns, cfDefs, p.SelectedCFKey, onCFKeyChange, as, ae, rates, base, fmtMinor, winForExports))
	}
	var appendix ui.Node = Fragment()
	if len(appendixBits) > 0 {
		// Collapsed by default within Full: the appendix is reference material
		// (tax totals, custom-field roll-ups, live variables), so it stays folded
		// until the reader asks for it rather than adding a fourth screen of tables.
		appendix = rptaSection("rpta-11", "11", uistate.T("rpta.secAppendix"), "dim", uistate.T("rpta.secAppendixSub"), "",
			Details(css.Class("rpta-appendix-fold"), Attr("data-testid", "rpta-appendix-fold"),
				Summary(uistate.T("rpta.appendixFold")),
				Div(css.Class("rpta-appendix-body"), Fragment(anyify(appendixBits)...))))
	}

	return appendix
}

type reportMetricsProps struct {
	Context reportSectionContext
	OnClose func()
}

func reportMetricsModal(p reportMetricsProps) ui.Node {
	// #46: the Report-metrics builder opens in the app-standard flip modal
	// instead of expanding a very large builder inline in the appendix, far
	// from its toolbar trigger. This child owns FlipPanel hooks while mounted.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("reports.metricsShow"),
		Width:     uiw.FlipLargeW,
		Height:    "min(90vh, 720px)",
		CloseOnly: true,
		OnClose:   p.OnClose,
		// Title "" → the builder's own "Formula calculator" heading, so the
		// modal header and the section heading never read as duplicates.
		Back: Div(Attr("data-testid", "reports-metrics-modal"),
			P(css.Class("rpta-muted"), uistate.T("reports.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{ShowSaved: true})),
	})
}

// reportToolbar keeps export-button hooks outside the controller. Its explicit
// context refreshes captured currency, scope and translations alongside data.
type reportToolbarProps struct {
	Context                                                      reportSectionContext
	App                                                          *appstate.App
	Scope                                                        scope.ReportScope
	ScopeOpen, FormulasOn, ExportOpen                            bool
	OnToggleScope, ToggleFormulas, OnToggleExport, OnCloseExport ui.Handler
	Transactions                                                 []domain.Transaction
	Spending, Income                                             []reports.CategorySpend
	Payees                                                       []reports.PayeeTotal
	Largest                                                      []reports.ExpenseItem
	Members                                                      []reports.MemberSpend
	Names                                                        map[string]string
	Resolution                                                   period.Resolution
	Rates                                                        currency.Rates
	Monthly                                                      []reports.PeriodFlow
}

func reportToolbar(p reportToolbarProps) ui.Node {
	return rptaToolbar(p.App, p.Scope, p.ScopeOpen, p.OnToggleScope, p.FormulasOn, p.ToggleFormulas,
		p.ExportOpen, p.OnToggleExport, p.OnCloseExport, p.Transactions, p.Spending, p.Income, p.Payees, p.Largest,
		p.Members, p.Context.categoryName(p.Names), p.Context.Base, p.Resolution, p.Context.From, p.Rates, p.Monthly)
}

// Appendix analyses may appear or disappear as their data changes. The export
// button owns its event hook, independently of each analysis' early returns.
type reportDownloadProps struct {
	Context          reportSectionContext
	TestID, TitleKey string
	Download         func()
}

func reportDownloadButton(p reportDownloadProps) ui.Node {
	return Button(css.Class("btn", tw.Gap2), Type("button"),
		Attr("data-testid", p.TestID), Title(uistate.T(p.TitleKey)), Attr("aria-label", uistate.T(p.TitleKey)),
		OnClick(p.Download), uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("reports.downloadCsv")))
}
