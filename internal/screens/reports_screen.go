//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/mermaid"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// trendBuckets is how many consecutive periods the cash-flow trend spans.
const trendBuckets = 6

// accentForRunway tones the cash-runway stat: a thin buffer (under three months)
// reads as a warning, a healthy one (six months or more) as positive.
func accentForRunway(months int) string {
	switch {
	case months < 3:
		return "neg"
	case months >= 6:
		return "pos"
	default:
		return ""
	}
}

// Reports is the read-only reporting screen (B21): for the period chosen in the
// top bar it shows income / expense / net, a plain-English summary, and spending
// by category compared to the prior period — all from the pure internal/reports
// core, so the figures match the rest of the app. Charts come in a follow-up.
func Reports() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()

	// Custom-field grouper: collect transaction-scoped field defs and track which
	// one the user has chosen for the roll-up section. Defaults to the first field.
	txnDefs := app.CustomFieldDefsFor("transaction")
	var cfDefs []customfields.Def
	for _, d := range txnDefs {
		cfDefs = append(cfDefs, d)
	}
	firstCFKey := ""
	if len(cfDefs) > 0 {
		firstCFKey = cfDefs[0].Key
	}
	selectedCFKey := ui.UseState(firstCFKey)
	onCFKeyChange := OnChange(func(v string) { selectedCFKey.Set(v) })

	// The viewed period is the shared top-bar window; the comparison is the
	// immediately preceding window of the same length.
	w := uistate.UsePeriod().Get()
	cs, ce := w.Range()
	ps, pe := w.Shift(-1).Range()

	flow, _ := reports.IncomeVsExpense(txns, cs, ce, rates)
	rows, _ := reports.SpendingByCategory(txns, cs, ce, true, ps, pe, rates)

	// No-spend days: elapsed days in the period with zero spending (motivating).
	noSpendDays := reports.NoSpendDays(txns, cs, ce, time.Now())
	spendStats, _ := reports.SpendingStats(txns, cs, ce, rates)

	// Headline spending trend vs the previous comparable period (up = worse).
	spendTrend := ""
	if pf, err := reports.IncomeVsExpense(txns, ps, pe, rates); err == nil {
		if pct, ok := ledger.PercentChange(flow.Expense, pf.Expense); ok {
			mag := pct
			if mag < 0 {
				mag = -mag
			}
			if pct > 0 {
				spendTrend = uistate.T("reports.spendUp", mag)
			} else if pct < 0 {
				spendTrend = uistate.T("reports.spendDown", mag)
			}
		}
	}

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
	// Net-worth composition (assets vs liabilities) as of now, for a breakdown card.
	nwNet, nwAssets, nwLiab, _ := ledger.NetWorth(accounts, txns, rates)
	// Net-worth change over the most recent period of the trend (last step).
	var nwChange int64
	if n := len(nwSeries); n >= 2 {
		nwChange = nwSeries[n-1].Amount - nwSeries[n-2].Amount
	}

	// Savings-rate trend: percent of income kept per period.
	srInts, _ := reports.SavingsRateSeries(txns, bounds, rates)
	srSeries := make([]float64, len(srInts))
	for i, v := range srInts {
		srSeries[i] = float64(v)
	}

	// Cash runway (B21): how long spendable cash would last at the average burn
	// over the last six *full* months (the current partial month is excluded so it
	// doesn't understate spending). Liquid = cash-type accounts only.
	const runwayMonths = 6
	liquid, _ := ledger.LiquidBalance(accounts, txns, rates)
	curMonth := dateutil.MonthStart(time.Now())
	monthBounds := make([]time.Time, 0, runwayMonths+1)
	for k := 0; k <= runwayMonths; k++ {
		monthBounds = append(monthBounds, dateutil.AddMonths(curMonth, k-runwayMonths))
	}
	monthFlows, _ := reports.IncomeExpenseSeries(txns, monthBounds, rates)
	burn := reports.AverageMonthlyExpense(monthFlows)
	runway := reports.EstimateRunway(liquid.Amount, burn)

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
	absI64 := func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	}
	// shareBar is a thin proportion bar for a ranked-list row: the row's amount as a
	// share of the list's largest, so the distribution is scannable at a glance
	// (C55). Inline-styled to avoid a stylesheet dependency.
	shareBar := func(amount, max int64) ui.Node {
		if max <= 0 {
			return Fragment()
		}
		pct := int(absI64(amount) * 100 / max)
		if pct > 100 {
			pct = 100
		}
		return Div(css.Class("share-bar"), Style(map[string]string{"height": "4px", "max-width": "260px", "margin-top": "0.3rem", "background": "var(--border)", "border-radius": "999px", "overflow": "hidden"}),
			Div(Style(map[string]string{"height": "100%", "width": fmt.Sprintf("%d%%", pct), "background": "var(--accent)", "border-radius": "999px"})))
	}
	narrative := reports.SpendingNarrative(rows, true, fmtMinor, func(id string) string { return catName[id] })

	// Heads-up: categories spending well above their recent monthly norm (top 3).
	// Reuses the shared insights detector (also behind the Insights highlights and
	// dashboard widget), filtered to overspending.
	var anomalyNodes []ui.Node
	for _, a := range detectSpendingAnomalies(txns, cats, rates) {
		if a.Direction != insights.Up {
			continue
		}
		anomalyNodes = append(anomalyNodes, P(css.Class("muted"), uistate.T("reports.anomaly", a.Category, a.PctChange)))
		if len(anomalyNodes) >= 3 {
			break
		}
	}

	// Category rows are plain text (no interactive controls), so building them in
	// a loop is safe (no On* hooks involved).
	var maxCat int64
	for _, r := range rows {
		if a := absI64(r.Amount); a > maxCat {
			maxCat = a
		}
	}
	var rowNodes []ui.Node
	for _, r := range rows {
		if r.Amount == 0 && r.Prior == 0 {
			continue
		}
		delta := Fragment()
		if r.HasDelta && r.Amount != r.Prior {
			// Spending up is red (worse), down is green (better).
			tone, arrow := "text-down", icon.ArrowUp
			if r.DeltaPct < 0 {
				tone, arrow = "text-up", icon.ArrowDown
			}
			pct := r.DeltaPct
			if pct < 0 {
				pct = -pct
			}
			delta = Span(ClassStr("row-meta "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)+" "+tw.ColorClass(tone)), uiw.Icon(arrow, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Text(fmt.Sprintf("%d%%", pct)))
		}
		rowNodes = append(rowNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), nameOf(r.CategoryID)), shareBar(r.Amount, maxCat)),
			delta,
			Span(css.Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	var catBody ui.Node
	if len(rowNodes) == 0 {
		catBody = P(css.Class("empty"), uistate.T("reports.empty"))
	} else {
		catBody = Div(css.Class("rows"), rowNodes)
	}

	// Top payees: where the money went by merchant/description this period.
	payees, _ := reports.TopPayees(txns, cs, ce, rates, 8)
	var maxPayee int64
	for _, p := range payees {
		if a := absI64(p.Amount); a > maxPayee {
			maxPayee = a
		}
	}
	var payeeNodes []ui.Node
	for _, p := range payees {
		name := p.Name
		if name == "" {
			name = uistate.T("reports.noPayee")
		}
		payeeNodes = append(payeeNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), name), shareBar(p.Amount, maxPayee)),
			Span(css.Class("budget-amount"), fmtMinor(p.Amount)),
		))
	}

	// Biggest individual expenses this period.
	largest, _ := reports.LargestExpenses(txns, cs, ce, rates, 8)
	var maxExp int64
	for _, e := range largest {
		if a := absI64(e.Amount); a > maxExp {
			maxExp = a
		}
	}
	var largestNodes []ui.Node
	for _, e := range largest {
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		largestNodes = append(largestNodes, Div(css.Class("row"),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), desc),
				Span(css.Class("row-meta"), pr.FormatDate(e.Date)),
				shareBar(e.Amount, maxExp),
			),
			Span(css.Class("budget-amount"), fmtMinor(e.Amount)),
		))
	}

	// Spending by member: the household "who spent what" view this period.
	memberSpend, _ := reports.SpendingByMember(txns, cs, ce, rates)
	memberName := make(map[string]string, len(app.Members()))
	for _, m := range app.Members() {
		memberName[m.ID] = m.Name
	}
	var memberNodes []ui.Node
	for _, ms := range memberSpend {
		name := memberName[ms.MemberID]
		if name == "" {
			name = uistate.T("reports.noMember")
		}
		memberNodes = append(memberNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), name)),
			Span(css.Class("budget-amount"), fmtMinor(ms.Amount)),
		))
	}

	// Biggest deposits: the largest individual income transactions this period.
	bigIncome, _ := reports.LargestIncome(txns, cs, ce, rates, 8)
	var bigIncomeNodes []ui.Node
	for _, e := range bigIncome {
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		bigIncomeNodes = append(bigIncomeNodes, Div(css.Class("row"),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), desc),
				Span(css.Class("row-meta"), pr.FormatDate(e.Date)),
			),
			Span(css.Class("budget-amount"), fmtMinor(e.Amount)),
		))
	}

	// Income by source: where the money comes from this period.
	incomeRows, _ := reports.IncomeByCategory(txns, cs, ce, rates)
	var incomeNodes []ui.Node
	for _, r := range incomeRows {
		if r.Amount == 0 {
			continue
		}
		incomeNodes = append(incomeNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), nameOf(r.CategoryID))),
			Span(css.Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	// Spending-by-weekday insight: which day money tends to leave.
	weekdayPeakLine := ""
	if wd, err := reports.SpendingByWeekday(txns, cs, ce, rates); err == nil {
		if d, ok := reports.PeakWeekday(wd); ok {
			weekdayPeakLine = uistate.T("reports.peakWeekday", d.String(), fmtMinor(wd[d]))
		}
	}

	net := money.New(flow.Net(), base)

	// Money-flow Sankey (C70): income fans out to each spending category, with the
	// leftover going to Savings. Values are minor units — only relative widths matter.
	var moneyFlows []mermaid.SankeyFlow
	for _, r := range rows {
		if v := absI64(r.Amount); v > 0 {
			moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: "Income", To: nameOf(r.CategoryID), Value: v})
		}
	}
	if sav := flow.Net(); sav > 0 {
		moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: "Income", To: "Savings", Value: sav})
	}

	return Div(
		Div(css.Class("stat-grid"),
			stat(uistate.T("dashboard.income"), fmtMoney(money.New(flow.Income, base)), "pos"),
			stat(uistate.T("dashboard.spending"), fmtMoney(money.New(flow.Expense, base)), "neg"),
			stat(uistate.T("reports.net"), fmtMoney(net), accentFor(net)),
			stat(uistate.T("dashboard.savingsRate"), fmt.Sprintf("%d%%", flow.SavingsRate()), ""),
			If(burn > 0, stat(uistate.T("reports.runway"), uistate.T("reports.runwayMonths", runway.Months), accentForRunway(runway.Months))),
			If(noSpendDays > 0, stat(uistate.T("reports.noSpendDays"), fmt.Sprintf("%d", noSpendDays), "pos")),
		),
		If(spendTrend != "", P(css.Class("muted"), spendTrend)),
		If(spendStats.Count > 0, P(css.Class("muted"), uistate.T("reports.spendStats", spendStats.Count, fmtMinor(spendStats.Average), fmtMinor(spendStats.Median)))),
		If(len(anomalyNodes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.headsUp")),
			Div(anomalyNodes),
		)),
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.byCategory")),
			P(css.Class("muted"), narrative),
			If(weekdayPeakLine != "", P(css.Class("muted"), weekdayPeakLine)),
			catBody,
			If(len(rowNodes) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("spending-by-category.csv", "text/csv", reports.CategoryCSV(rows, nameOf, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			)),
		),
		If(len(moneyFlows) > 1, Section(css.Class("card"),
			H2(css.Class("card-title"), "Money flow"),
			uiw.Mermaid(uiw.MermaidProps{Source: mermaid.Sankey(moneyFlows), Label: "Income to spending categories money-flow"}),
		)),
		If(len(bigIncomeNodes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.biggestDeposits")),
			Div(css.Class("rows"), bigIncomeNodes),
		)),
		If(len(incomeNodes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.incomeBySource")),
			Div(css.Class("rows"), incomeNodes),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("income-by-source.csv", "text/csv", reports.CategoryCSV(incomeRows, nameOf, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			),
		)),
		If(len(payeeNodes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.topPayees")),
			Div(css.Class("rows"), payeeNodes),
		)),
		If(len(largestNodes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.biggestExpenses")),
			Div(css.Class("rows"), largestNodes),
		)),
		If(len(memberSpend) > 1, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.byMember")),
			Div(css.Class("rows"), memberNodes),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					nm := func(id string) string {
						if n := memberName[id]; n != "" {
							return n
						}
						return uistate.T("reports.noMember")
					}
					downloadBytes("spending-by-member.csv", "text/csv", reports.MemberCSV(memberSpend, nm, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			),
		)),
		If(len(netSeries) >= 2, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("dashboard.cashFlow")),
			P(css.Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
			uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, GradientID: "cf-reports", Label: uistate.T("dashboard.cashFlow")}),
		)),
		If(len(accounts) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("dashboard.netWorth")),
			Div(css.Class("stat-grid"),
				stat(uistate.T("accounts.assets"), fmtMoney(nwAssets), "pos"),
				stat(uistate.T("dashboard.liabilities"), fmtMoney(nwLiab), "neg"),
				stat(uistate.T("dashboard.netWorth"), fmtMoney(nwNet), accentFor(nwNet)),
				If(len(nwSeries) >= 2, stat(uistate.T("reports.netWorthChange"), fmtMoney(money.New(nwChange, base)), accentFor(money.New(nwChange, base)))),
			),
		)),
		If(len(nw) >= 2, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("dashboard.netWorthTrend")),
			uiw.AreaChart(uiw.AreaChartProps{Values: nw, Stroke: "#7c83ff", GradientID: "nw-reports", Label: uistate.T("dashboard.netWorthTrend")}),
		)),
		If(len(srSeries) >= 2, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.savingsTrend")),
			P(css.Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
			uiw.AreaChart(uiw.AreaChartProps{Values: srSeries, GradientID: "sr-reports", Label: uistate.T("reports.savingsTrend")}),
		)),
		If(len(cfDefs) > 0, customFieldSpendSection(txns, cfDefs, selectedCFKey.Get(), onCFKeyChange, cs, ce, rates, base, fmtMinor)),
	)
}

// customFieldSpendSection renders the "Spending by <field>" card: a field
// selector, a ranked list of value→amount rows, and a CSV download button.
// It is extracted to keep the main Reports function readable and to isolate the
// per-field OnChange hook (called at a single stable render position, not in a
// loop).
func customFieldSpendSection(
	txns []domain.Transaction,
	defs []customfields.Def,
	selectedKey string,
	onKeyChange any,
	start, end time.Time,
	rates currency.Rates,
	base string,
	fmtMinor func(int64) string,
) ui.Node {
	// Resolve the active definition; fall back to the first if selectedKey is stale.
	activeDef := defs[0]
	for _, d := range defs {
		if d.Key == selectedKey {
			activeDef = d
			break
		}
	}

	cfRows, _ := reports.ByCustomField(txns, activeDef.Key, start, end, rates)

	// Field selector options — built outside of a loop hook (no On* here).
	var fieldOpts []ui.Node
	for _, d := range defs {
		fieldOpts = append(fieldOpts, Option(Value(d.Key), SelectedIf(d.Key == activeDef.Key), d.Label))
	}

	// Value rows are plain display (no On* in the loop).
	noValueLabel := uistate.T("reports.customFieldNoValue")
	var rowNodes []ui.Node
	var maxAmt int64
	for _, r := range cfRows {
		if r.Amount > maxAmt {
			maxAmt = r.Amount
		}
	}
	for _, r := range cfRows {
		label := r.Value
		if label == "" {
			label = noValueLabel
		}
		pct := 0
		if maxAmt > 0 {
			pct = int(r.Amount * 100 / maxAmt)
		}
		if pct > 100 {
			pct = 100
		}
		bar := Div(Style(map[string]string{"height": "4px", "max-width": "260px", "margin-top": "0.3rem", "background": "var(--border)", "border-radius": "999px", "overflow": "hidden"}),
			Div(Style(map[string]string{"height": "100%", "width": fmt.Sprintf("%d%%", pct), "background": "var(--accent)", "border-radius": "999px"})))
		rowNodes = append(rowNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), label), bar),
			Span(css.Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	var body ui.Node
	if len(rowNodes) == 0 {
		body = P(css.Class("empty"), uistate.T("reports.empty"))
	} else {
		body = Div(css.Class("rows"), rowNodes)
	}

	sectionLabel := uistate.T("reports.byCustomField", activeDef.Label)
	selectorLabel := uistate.T("reports.customFieldSelectLabel")

	return Section(css.Class("card"), Attr("data-testid", "customfield-spend-section"),
		H2(css.Class("card-title"), sectionLabel),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Py1),
			Label(Attr("for", "cf-field-select"), selectorLabel),
			Select(css.Class("field"), Attr("id", "cf-field-select"), Attr("aria-label", selectorLabel), Attr("data-testid", "cf-field-select"), onKeyChange, fieldOpts),
		),
		body,
		If(len(rowNodes) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			Button(css.Class("btn"), Type("button"),
				Attr("data-testid", "cf-download-csv"),
				Title(uistate.T("reports.customFieldDownloadTitle")),
				Attr("aria-label", uistate.T("reports.customFieldDownloadTitle")),
				OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					filename := "spending-by-" + activeDef.Key + ".csv"
					downloadBytes(filename, "text/csv", reports.CustomFieldCSV(cfRows, activeDef.Label, csvAmount))
				}),
				uistate.T("reports.downloadCsv"),
			),
		)),
	)
}
