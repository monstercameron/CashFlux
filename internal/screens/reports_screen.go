//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/chartspec"
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
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// reportsBarSpec builds a horizontal Bar chart spec from label+amount pairs.
// amounts are in minor currency units; pass decimals (e.g. 2) to convert to major units.
func reportsBarSpec(pairs []struct {
	Label  string
	Amount int64
}, decimals int) chartspec.Spec {
	divisor := math.Pow(10, float64(decimals))
	var points []chartspec.Point
	for i, p := range pairs {
		points = append(points, chartspec.Point{
			X:     float64(i),
			Y:     float64(p.Amount) / divisor,
			Label: p.Label,
		})
	}
	return chartspec.Spec{
		Kind: chartspec.Bar,
		Series: []chartspec.Series{
			{Name: "Amount", Color: "#4f8ef7", Points: points},
		},
		Legend: false,
	}
}

// reportsDonutSpec builds a Donut chart spec from label+amount pairs. Donut
// charts are single-series with one point per slice (per chartspec invariant).
func reportsDonutSpec(pairs []struct {
	Label  string
	Amount int64
}, decimals int) chartspec.Spec {
	divisor := math.Pow(10, float64(decimals))
	var points []chartspec.Point
	for i, p := range pairs {
		points = append(points, chartspec.Point{
			X:     float64(i),
			Y:     float64(p.Amount) / divisor,
			Label: p.Label,
		})
	}
	return chartspec.Spec{
		Kind:   chartspec.Donut,
		Series: []chartspec.Series{{Name: "Spending", Points: points}},
		Legend: true,
	}
}

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
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	// Subscribe to the shared data-revision atom so Reports re-renders whenever a
	// recategorize (or any other mutation) bumps the revision mid-session
	// (RECAT_UPDATES, L58). The atom is read for its side-effect of registering
	// the subscription; the value itself is not used here.
	_ = uistate.UseDataRevision().Get()

	// Navigation + filter wiring for the category drill-through (L58): clicking a
	// category row opens /transactions pre-filtered to that category, mirroring
	// the budgets→transactions drill (C30/C50). Hooks are called once at a stable
	// position; the per-row handler is threaded down as a plain func.
	nav := router.UseNavigate()
	txFilterAtom := uistate.UseTxFilter()
	viewCategoryTransactions := func(categoryID string) {
		f := uistate.TxFilter{Category: categoryID}.Normalize()
		txFilterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
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

	// Roll sub-categories up into their top-level parent in the by-category
	// breakdown (L28). Off by default so sub-category detail stays visible.
	rollupCats := ui.UseState(false)
	onToggleRollup := ui.UseEvent(func() { rollupCats.Set(!rollupCats.Get()) })

	// The viewed period is the shared top-bar window; the comparison is the
	// immediately preceding window of the same length. Persist the full window
	// (resolution + anchors) so /reports reopens on the last-viewed period
	// after a hard reload (L45/L58).
	w := uistate.UsePeriod().Get()
	uistate.PersistPeriodWindow(w)
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
	if rollupCats.Get() {
		rows = reports.RollUpByParent(rows, cats)
	}
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

	// Category rows each carry a drill-through click handler. Because On* hooks
	// must not be called inside a variable-length loop, each row is its own
	// component (reportsCatRow); the handler func is passed as a plain prop.
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
		rowNodes = append(rowNodes, ui.CreateElement(reportsCatRow, reportsCatRowProps{
			CategoryID: r.CategoryID,
			Name:       nameOf(r.CategoryID),
			Amount:     r.Amount,
			Prior:      r.Prior,
			HasDelta:   r.HasDelta,
			DeltaPct:   r.DeltaPct,
			MaxCat:     maxCat,
			FmtMinor:   fmtMinor,
			ShareBar:   shareBar,
			OnDrill:    func(id string) { viewCategoryTransactions(id) },
		}))
	}

	// V2: top-8 spending categories as a ranked bar chart above the text list.
	// V3: top-5 categories + "Other" as a donut in the same card.
	decimals := currency.Decimals(base)
	var catBarNodes []ui.Node
	var catDonutNodes []ui.Node
	if len(rows) > 0 {
		type labelAmt struct {
			Label  string
			Amount int64
		}
		// Bar: top 8 by absolute amount (spending rows are negative — negate for display).
		var barPairs []struct {
			Label  string
			Amount int64
		}
		for i, r := range rows {
			if i >= 8 {
				break
			}
			if r.Amount == 0 {
				continue
			}
			barPairs = append(barPairs, struct {
				Label  string
				Amount int64
			}{Label: nameOf(r.CategoryID), Amount: absI64(r.Amount)})
		}
		if len(barPairs) > 0 {
			spec := reportsBarSpec(barPairs, decimals)
			catBarNodes = append(catBarNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Top spending categories ranked by amount"}))
		}
		// Donut: top 5 + "Other" bucket.
		var donutPairs []struct {
			Label  string
			Amount int64
		}
		var otherAmt int64
		for i, r := range rows {
			if r.Amount == 0 {
				continue
			}
			if i < 5 {
				donutPairs = append(donutPairs, struct {
					Label  string
					Amount int64
				}{Label: nameOf(r.CategoryID), Amount: absI64(r.Amount)})
			} else {
				otherAmt += absI64(r.Amount)
			}
		}
		if otherAmt > 0 {
			donutPairs = append(donutPairs, struct {
				Label  string
				Amount int64
			}{Label: "Other", Amount: otherAmt})
		}
		if len(donutPairs) > 0 {
			spec := reportsDonutSpec(donutPairs, decimals)
			catDonutNodes = append(catDonutNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Spending share by category"}))
		}
	}

	var catBody ui.Node
	if len(rowNodes) == 0 {
		catBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("reports.empty"), CTALabel: uistate.T("reports.addFirst"), Href: "/transactions"})
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

	// V4: income-by-source donut chart.
	var incomeDonutNodes []ui.Node
	{
		var donutPairs []struct {
			Label  string
			Amount int64
		}
		for _, r := range incomeRows {
			if r.Amount == 0 {
				continue
			}
			donutPairs = append(donutPairs, struct {
				Label  string
				Amount int64
			}{Label: nameOf(r.CategoryID), Amount: absI64(r.Amount)})
		}
		if len(donutPairs) > 0 {
			spec := reportsDonutSpec(donutPairs, decimals)
			incomeDonutNodes = append(incomeDonutNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Income share by source"}))
		}
	}

	// V7: ranked bar charts for top payees and biggest expenses.
	var payeeBarNodes []ui.Node
	{
		var barPairs []struct {
			Label  string
			Amount int64
		}
		for _, p := range payees {
			name := p.Name
			if name == "" {
				name = uistate.T("reports.noPayee")
			}
			barPairs = append(barPairs, struct {
				Label  string
				Amount int64
			}{Label: name, Amount: absI64(p.Amount)})
		}
		if len(barPairs) > 0 {
			spec := reportsBarSpec(barPairs, decimals)
			payeeBarNodes = append(payeeBarNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Top payees ranked by amount"}))
		}
	}
	var expenseBarNodes []ui.Node
	{
		var barPairs []struct {
			Label  string
			Amount int64
		}
		for _, e := range largest {
			desc := e.Desc
			if desc == "" {
				desc = nameOf(e.CategoryID)
			}
			barPairs = append(barPairs, struct {
				Label  string
				Amount int64
			}{Label: desc, Amount: absI64(e.Amount)})
		}
		if len(barPairs) > 0 {
			spec := reportsBarSpec(barPairs, decimals)
			expenseBarNodes = append(expenseBarNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Biggest individual expenses ranked by amount"}))
		}
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

	// G9.1: Advanced disclosure toggle state (wraps custom field + deductible cards).
	showAdvanced := ui.UseState(false)
	onToggleAdvanced := ui.UseEvent(func() { showAdvanced.Set(!showAdvanced.Get()) })
	advancedLabel := "Advanced ▾"
	if showAdvanced.Get() {
		advancedLabel = "Advanced ▲"
	}

	return Div(
		// G9.1 Item 1 — Hero zone: Net / Income / Spend as a prominent headline strip
		// above the card flow. Savings rate, runway, no-spend days go in a secondary row.
		Div(css.Class("reports-hero"),
			P(css.Class("hero-period"), uistate.T("reports.covering", pr.FormatDate(cs), pr.FormatDate(ce), pr.FormatDate(ps), pr.FormatDate(pe))),
			Div(css.Class("hero-main"),
				Div(
					P(css.Class("hero-flanker-label"), uistate.T("reports.net")),
					P(ClassStr("hero-net "+accentFor(net)), fmtMoney(net)),
				),
				Div(css.Class("hero-flankers"),
					Div(css.Class("hero-flanker"),
						Span(css.Class("hero-flanker-label"), uistate.T("dashboard.income")),
						Span(css.Class("hero-flanker-value", "pos"), fmtMoney(money.New(flow.Income, base))),
					),
					Div(css.Class("hero-flanker"),
						Span(css.Class("hero-flanker-label"), uistate.T("dashboard.spending")),
						Span(css.Class("hero-flanker-value", "neg"), fmtMoney(money.New(flow.Expense, base))),
					),
				),
			),
			Div(css.Class("hero-secondary"),
				Div(css.Class("hero-stat"),
					Span(css.Class("hero-stat-label"), uistate.T("dashboard.savingsRate")),
					Span(css.Class("hero-stat-value"), fmt.Sprintf("%d%%", flow.SavingsRate())),
				),
				If(burn > 0, Div(css.Class("hero-stat"),
					Span(css.Class("hero-stat-label"), uistate.T("reports.runway")),
					Span(ClassStr("hero-stat-value "+accentForRunway(runway.Months)), uistate.T("reports.runwayMonths", runway.Months)),
				)),
				If(noSpendDays > 0, Div(css.Class("hero-stat"),
					Span(css.Class("hero-stat-label"), uistate.T("reports.noSpendDays")),
					Span(css.Class("hero-stat-value", "pos"), fmt.Sprintf("%d", noSpendDays)),
				)),
			),
		),
		If(spendTrend != "", P(css.Class("muted"), spendTrend)),
		If(spendStats.Count > 0, P(css.Class("muted"), uistate.T("reports.spendStats", spendStats.Count, fmtMinor(spendStats.Average), fmtMinor(spendStats.Median)))),
		// G9.1 Item 3 — Heads-up anomaly card gets .card-alert urgency border.
		If(len(anomalyNodes) > 0, Section(css.Class("card", "card-alert"),
			H2(css.Class("card-title"), uistate.T("reports.headsUp")),
			Div(anomalyNodes),
		)),
		// Section dividers group the 13-card scroll into Spending / Income / Trends so
		// Priya can navigate the page instead of reading it top-to-bottom (G9/C55).
		H3(css.Class("section-divider"), uistate.T("reports.sectionSpending")),
		Section(css.Class("card"),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.FlexWrap, tw.Gap2),
				H2(css.Class("card-title"), uistate.T("reports.byCategory")),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "reports-rollup-toggle"),
					Attr("aria-pressed", boolStr(rollupCats.Get())),
					Title(uistate.T("reports.rollupTitle")), OnClick(onToggleRollup),
					uistate.T(rollupLabelKey(rollupCats.Get()))),
			),
			P(css.Class("muted"), narrative),
			If(weekdayPeakLine != "", P(css.Class("muted"), weekdayPeakLine)),
			If(len(catBarNodes) > 0, Div(catBarNodes)),
			If(len(catDonutNodes) > 0, Div(catDonutNodes)),
			catBody,
			If(len(rowNodes) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes(reports.ExportFilename("spending-by-category", w.Res, w.From), "text/csv", reports.CategoryCSV(rows, nameOf, csvAmount))
				}), uistate.T("reports.downloadCsv")),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "reports-tax-summary"), Title(uistate.T("reports.taxSummaryTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					// Tax summary always covers a full calendar year. When viewing
					// a Year window use that year; otherwise fall back to the current
					// calendar year so the export is always a complete annual set.
					yr := w.From.Year()
					if w.Res != period.Year {
						yr = time.Now().Year()
					}
					ys := time.Date(yr, time.January, 1, 0, 0, 0, 0, time.UTC)
					ye := time.Date(yr+1, time.January, 1, 0, 0, 0, 0, time.UTC)
					summary, _ := reports.YearTax(txns, yr, ys, ye, rates)
					downloadBytes(reports.ExportFilename("tax-summary", period.Year, ys), "text/csv", reports.YearTaxCSV(summary, nameOf, csvAmount))
				}), uistate.T("reports.taxSummary")),
			)),
		),
		// G9.1 Item 5 — Sankey moved up: directly after category → Sankey → payees → biggest expenses.
		If(len(moneyFlows) > 1, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: "Money flow",
			Body:  uiw.Mermaid(uiw.MermaidProps{Source: mermaid.Sankey(moneyFlows), Label: "Income to spending categories money-flow"}),
		})),
		If(len(payeeNodes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.topPayees")),
			If(len(payeeBarNodes) > 0, Div(payeeBarNodes)),
			Div(css.Class("rows"), payeeNodes),
			Div(css.Class(tw.Fold(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1)),
				Button(css.Class("btn"), Type("button"),
					Attr("data-testid", "reports-payees-csv"),
					Title(uistate.T("reports.downloadCsvTitle")),
					OnClick(func() {
						csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
						downloadBytes(reports.ExportFilename("top-payees", w.Res, w.From), "text/csv", reports.PayeeCSV(payees, csvAmount))
					}),
					uistate.T("reports.downloadCsv")),
			),
		)),
		If(len(largestNodes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.biggestExpenses")),
			If(len(expenseBarNodes) > 0, Div(expenseBarNodes)),
			Div(css.Class("rows"), largestNodes),
			Div(css.Class(tw.Fold(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1)),
				Button(css.Class("btn"), Type("button"),
					Attr("data-testid", "reports-largest-csv"),
					Title(uistate.T("reports.downloadCsvTitle")),
					OnClick(func() {
						csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
						downloadBytes(reports.ExportFilename("largest-expenses", w.Res, w.From), "text/csv", reports.LargestExpensesCSV(largest, nameOf, csvAmount))
					}),
					uistate.T("reports.downloadCsv")),
			),
		)),
		H3(css.Class("section-divider"), uistate.T("reports.sectionIncome")),
		If(len(bigIncomeNodes) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("reports.biggestDeposits"),
			Rows:  bigIncomeNodes,
		})),
		If(len(incomeNodes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("reports.incomeBySource")),
			If(len(incomeDonutNodes) > 0, Div(incomeDonutNodes)),
			Div(css.Class("rows"), incomeNodes),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes(reports.ExportFilename("income-by-source", w.Res, w.From), "text/csv", reports.CategoryCSV(incomeRows, nameOf, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			),
		)),
		// L21: show the member-spend section whenever the household has ≥2 members
		// and at least one has attributed spending — not just when both have spend.
		// With one member doing all the spending, the section still answers "who
		// spent what?" and surfaces the unattributed remainder.
		If(len(app.Members()) >= 2 && len(memberSpend) >= 1, Section(css.Class("card"),
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
					downloadBytes(reports.ExportFilename("spending-by-member", w.Res, w.From), "text/csv", reports.MemberCSV(memberSpend, nm, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			),
		)),
		H3(css.Class("section-divider"), uistate.T("reports.sectionTrends")),
		If(len(netSeries) >= 2, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("dashboard.cashFlow"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
				uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, GradientID: "cf-reports", Label: uistate.T("dashboard.cashFlow")}),
			),
		})),
		If(len(accounts) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("dashboard.netWorth"),
			Body: Div(css.Class("stat-grid"),
				stat(uistate.T("accounts.assets"), fmtMoney(nwAssets), "pos"),
				stat(uistate.T("dashboard.liabilities"), fmtMoney(nwLiab), "neg"),
				stat(uistate.T("dashboard.netWorth"), fmtMoney(nwNet), accentFor(nwNet)),
				If(len(nwSeries) >= 2, stat(uistate.T("reports.netWorthChange"), fmtMoney(money.New(nwChange, base)), accentFor(money.New(nwChange, base)))),
			),
		})),
		If(len(nw) >= 2, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("dashboard.netWorthTrend"),
			Body:  uiw.AreaChart(uiw.AreaChartProps{Values: nw, Stroke: "#7c83ff", GradientID: "nw-reports", Label: uistate.T("dashboard.netWorthTrend")}),
		})),
		If(len(srSeries) >= 2, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("reports.savingsTrend"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
				uiw.AreaChart(uiw.AreaChartProps{Values: srSeries, GradientID: "sr-reports", Label: uistate.T("reports.savingsTrend")}),
			),
		})),
		// G9.1 Item 6 — Advanced collapse: wraps custom field spend and deductible totals.
		// Both are behind a disclosure toggle ("Advanced ▾/▲"), collapsed by default.
		If(len(cfDefs) > 0,
			Div(
				Button(css.Class("disclosure-toggle"), Type("button"),
					Attr("aria-expanded", boolStr(showAdvanced.Get())),
					OnClick(onToggleAdvanced),
					advancedLabel,
				),
				If(showAdvanced.Get(),
					Div(
						customFieldSpendSection(txns, cfDefs, selectedCFKey.Get(), onCFKeyChange, cs, ce, rates, base, fmtMinor, w),
						deductibleSection(txns, cats, cs, ce, rates, base, fmtMinor, w),
					),
				),
			),
		),
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
	win period.Window,
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
					filename := reports.ExportFilename("spending-by-"+activeDef.Key, win.Res, win.From)
					downloadBytes(filename, "text/csv", reports.CustomFieldCSV(cfRows, activeDef.Label, csvAmount))
				}),
				uistate.T("reports.downloadCsv"),
			),
		)),
	)
}

// reportsCatRowProps carries the display data and drill callback for one
// spending-by-category row. The OnDrill func is called with the category ID
// when the user clicks the row label, navigating to /transactions filtered to
// that category (L58 FILTER_CARRY drill-through).
type reportsCatRowProps struct {
	CategoryID string
	Name       string
	Amount     int64
	Prior      int64
	HasDelta   bool
	DeltaPct   int64
	MaxCat     int64
	FmtMinor   func(int64) string
	ShareBar   func(amount, max int64) ui.Node
	OnDrill    func(id string)
}

// reportsCatRow renders one row of the spending-by-category table with a
// clickable label that drills through to /transactions filtered by that
// category. It is a standalone component so its OnClick hook is registered at
// a stable render position (not inside a variable-length loop in Reports).
func reportsCatRow(props reportsCatRowProps) ui.Node {
	drill := ui.UseEvent(func() { props.OnDrill(props.CategoryID) })

	delta := Fragment()
	if props.HasDelta && props.Amount != props.Prior {
		tone, arrow := "text-down", icon.ArrowUp
		if props.DeltaPct < 0 {
			tone, arrow = "text-up", icon.ArrowDown
		}
		pct := props.DeltaPct
		if pct < 0 {
			pct = -pct
		}
		delta = Span(ClassStr("row-meta "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)+" "+tw.ColorClass(tone)),
			uiw.Icon(arrow, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Text(fmt.Sprintf("%d%%", int(pct))))
	}

	return Div(css.Class("row"), Attr("data-testid", "reports-cat-row"), Attr("data-category-id", props.CategoryID),
		Div(css.Class("row-main"),
			Button(
				css.Class("row-desc", "btn-link"),
				Type("button"),
				Attr("data-testid", "reports-cat-drill"),
				Attr("aria-label", "View transactions: "+props.Name),
				OnClick(drill),
				props.Name,
			),
			props.ShareBar(props.Amount, props.MaxCat),
		),
		delta,
		Span(css.Class("budget-amount"), props.FmtMinor(props.Amount)),
	)
}

// rollupLabelKey is the i18n key for the by-category roll-up toggle's label,
// reflecting whether sub-categories are currently rolled up (L28).
func rollupLabelKey(on bool) string {
	if on {
		return "reports.rollupOn"
	}
	return "reports.rollupOff"
}

// deductibleSection renders the "Deductible totals" card (L16/L58): a ranked
// list of deductible-flagged categories with their expense totals for the
// period, a headline total, and a CSV export.  Returns an empty fragment when
// no categories are marked deductible, so the section stays invisible until the
// user sets up at least one deductible category.
func deductibleSection(
	txns []domain.Transaction,
	cats []domain.Category,
	start, end time.Time,
	rates currency.Rates,
	base string,
	fmtMinor func(int64) string,
	win period.Window,
) ui.Node {
	// Only show the section when at least one deductible category exists.
	hasDeductible := false
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
		if c.Deductible {
			hasDeductible = true
		}
	}
	if !hasDeductible {
		return Fragment()
	}

	summary, _ := reports.DeductibleTotals(txns, cats, start, end, rates)
	nameOf := func(id string) string {
		if n := catName[id]; n != "" {
			return n
		}
		return uistate.T("reports.uncategorized")
	}

	var rowNodes []ui.Node
	var maxAmt int64
	for _, r := range summary.Rows {
		if r.Amount > maxAmt {
			maxAmt = r.Amount
		}
	}
	for _, r := range summary.Rows {
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
			Div(css.Class("row-main"), Span(css.Class("row-desc"), nameOf(r.CategoryID)), bar),
			Span(css.Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	var body ui.Node
	if len(rowNodes) == 0 {
		body = P(css.Class("empty"), uistate.T("reports.empty"))
	} else {
		body = Div(css.Class("rows"), rowNodes)
	}

	return Section(css.Class("card"), Attr("data-testid", "deductible-section"),
		H2(css.Class("card-title"), uistate.T("reports.deductibleTitle")),
		P(css.Class("muted"), uistate.T("reports.deductibleHint")),
		If(summary.Total > 0, P(css.Class("muted"), uistate.T("reports.deductibleTotal", fmtMinor(summary.Total)))),
		body,
		If(len(rowNodes) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			Button(css.Class("btn"), Type("button"),
				Attr("data-testid", "deductible-download-csv"),
				Title(uistate.T("reports.deductibleDownloadTitle")),
				Attr("aria-label", uistate.T("reports.deductibleDownloadTitle")),
				OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes(reports.ExportFilename("deductible-totals", win.Res, win.From), "text/csv", reports.DeductibleCSV(summary, nameOf, csvAmount))
				}),
				uistate.T("reports.downloadCsv"),
			),
		)),
	)
}
