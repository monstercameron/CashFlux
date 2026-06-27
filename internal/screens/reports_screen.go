// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"syscall/js"
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

// tableau10Palette mirrors d3.schemeTableau10 (the palette the donut renderer
// uses by slice index), so a bar colored tableau10(i) matches the donut's i-th
// slice exactly. Indices past the end cycle, matching d3.scaleOrdinal.
var tableau10Palette = []string{
	"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f",
	"#edc949", "#af7aa1", "#ff9da7", "#9c755f", "#bab0ab",
}

// tableau10 returns the i-th Tableau10 color (cycling), matching the donut palette.
func tableau10(i int) string {
	if i < 0 {
		i = 0
	}
	return tableau10Palette[i%len(tableau10Palette)]
}

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
		// "money" Y ticks → currency-aware compact axis ("$1.5k") matching the
		// rest of the app instead of bare numbers (the symbol is passed live via
		// ChartProps.CurrencySymbol so non-USD bases render the right glyph).
		Y:      chartspec.Axis{Format: "money"},
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

// toneForSavingsRate colors the savings-rate stat by sign: a negative rate
// (spending exceeded income) reads as a warning, a positive rate as healthy —
// matching the sibling runway/no-spend-day stats in the hero-secondary row.
func toneForSavingsRate(pct int) string {
	switch {
	case pct < 0:
		return "neg"
	case pct > 0:
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

	// C237: Year-over-Year comparison toggle. When on, the comparison period is
	// exactly one calendar year prior (via reports.YoYPrior); when off, it is the
	// immediately preceding window of the same length (MoM / period-over-period).
	yoyMode := ui.UseState(false)
	onToggleYoY := ui.UseEvent(func() { yoyMode.Set(!yoyMode.Get()) })

	// The viewed period is the shared top-bar window; the comparison is the
	// immediately preceding window of the same length (or the same window one year
	// prior when YoY mode is on). Persist the full window (resolution + anchors)
	// so /reports reopens on the last-viewed period after a hard reload (L45/L58).
	w := uistate.UsePeriod().Get()
	uistate.PersistPeriodWindow(w)
	cs, ce := w.Range()
	var ps, pe time.Time
	if yoyMode.Get() {
		ps, pe = reports.YoYPrior(w).Range()
	} else {
		ps, pe = w.Shift(-1).Range()
	}

	flow, _ := reports.IncomeVsExpense(txns, cs, ce, rates)
	rows, _ := reports.SpendingByCategory(txns, cs, ce, true, ps, pe, rates)

	// No-spend days: elapsed days in the period with zero spending (motivating).
	noSpendDays := reports.NoSpendDays(txns, cs, ce, time.Now())
	spendStats, _ := reports.SpendingStats(txns, cs, ce, rates)

	// Previous comparable period flow, computed once for both the headline spending
	// trend and the hero Net delta chip (G9.1: period-over-period context).
	prevFlow, prevFlowErr := reports.IncomeVsExpense(txns, ps, pe, rates)
	prevFlowOK := prevFlowErr == nil

	// Headline spending trend vs the previous comparable period (up = worse).
	spendTrend := ""
	if prevFlowOK {
		if pct, ok := ledger.PercentChange(flow.Expense, prevFlow.Expense); ok {
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
	// R-4: x-axis period captions for the trend chart, one per bucket (from bounds).
	trendLabels := make([]string, 0, len(netSeries))
	for i := 0; i < len(netSeries) && i < len(bounds); i++ {
		trendLabels = append(trendLabels, bounds[i].Format("Jan"))
	}

	// Net-worth trend: always monthly, independent of the cash-flow period selector
	// (C217). Net worth is a cumulative point-in-time series — re-bucketing it to
	// weekly or quarterly makes no sense. We always show the last trendBuckets months.
	accounts := app.Accounts()
	curMonth := dateutil.MonthStart(time.Now())
	nwBounds := make([]time.Time, 0, trendBuckets+1)
	for k := 0; k <= trendBuckets; k++ {
		nwBounds = append(nwBounds, dateutil.AddMonths(curMonth, k-trendBuckets))
	}
	nwSeries, _ := ledger.NetWorthSeries(accounts, txns, nwBounds, rates)
	// x-axis labels for the NW chart: month abbreviation from nwBounds.
	nwLabels := make([]string, 0, len(nwSeries))
	for i := 0; i < len(nwSeries) && i < len(nwBounds); i++ {
		nwLabels = append(nwLabels, nwBounds[i].Format("Jan"))
	}
	// Convert to major units (dollars) so the Y-axis ticks read "$14k" not "1400000"
	// (C216: same fix applied to the dashboard NW chart in C16).
	nwDiv := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		nwDiv *= 10
	}
	nw := make([]float64, len(nwSeries))
	for i, m := range nwSeries {
		nw[i] = float64(m.Amount) / nwDiv
	}
	// Net-worth composition (assets vs liabilities) as of now, for a breakdown card.
	nwNet, nwAssets, nwLiab, _ := ledger.NetWorth(accounts, txns, rates)
	// Net-worth change over the most recent monthly step of the trend (last step).
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
	// Per-point hover labels for the trend AreaCharts (R-4 follow-up): cash-flow values are minor
	// units → money; savings-rate is already a percent. Lets each trend point read its exact value on hover.
	moneyLabels := func(vals []float64) []string {
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = fmtMoney(money.New(int64(v), base))
		}
		return out
	}
	// nwValueLabels builds hover labels for the NW trend from the raw minor-unit
	// series (nwSeries), independent of the major-unit nw[] used for the Y-axis.
	nwValueLabels := make([]string, len(nwSeries))
	for i, m := range nwSeries {
		nwValueLabels[i] = fmtMoney(money.New(m.Amount, base))
	}
	pctLabels := func(vals []float64) []string {
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = fmt.Sprintf("%d%%", int(v))
		}
		return out
	}

	// R52(a): decision-oriented chart captions — a one-sentence plain-English
	// takeaway (direction + magnitude over the window) instead of only a generic
	// "last N months" hint, so each trend chart states its insight at a glance.
	cashFlowTakeaway := ""
	if len(netSeries) >= 2 {
		delta := int64(netSeries[len(netSeries)-1] - netSeries[0])
		latest := fmtMoney(money.New(int64(netSeries[len(netSeries)-1]), base))
		mag := fmtMoney(money.New(absMinor(delta), base))
		switch {
		case delta > 0:
			cashFlowTakeaway = uistate.T("reports.cashFlowTakeawayUp", latest, mag)
		case delta < 0:
			cashFlowTakeaway = uistate.T("reports.cashFlowTakeawayDown", latest, mag)
		default:
			cashFlowTakeaway = uistate.T("reports.cashFlowTakeawayFlat", latest)
		}
	}
	savingsTakeaway := ""
	if len(srSeries) >= 2 {
		first, last := int(srSeries[0]), int(srSeries[len(srSeries)-1])
		pts := last - first
		if pts < 0 {
			pts = -pts
		}
		switch {
		case last > first:
			savingsTakeaway = uistate.T("reports.savingsTakeawayUp", last, pts)
		case last < first:
			savingsTakeaway = uistate.T("reports.savingsTakeawayDown", last, pts)
		default:
			savingsTakeaway = uistate.T("reports.savingsTakeawayFlat", last)
		}
	}

	// Cash runway (B21): how long spendable cash would last at the average burn
	// over the last six *full* months (the current partial month is excluded so it
	// doesn't understate spending). Liquid = cash-type accounts only.
	const runwayMonths = 6
	liquid, _ := ledger.LiquidBalance(accounts, txns, rates)
	// curMonth already declared above for the NW trend bounds.
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
		return Div(css.Class("share-bar"), Style(map[string]string{"height": "8px", "max-width": "100%", "margin-top": "0.3rem", "background": "var(--border)", "border-radius": "999px", "overflow": "hidden"}),
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
	for i, r := range rows {
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
			PriorZero:  r.PriorZero,
			MaxCat:     maxCat,
			CatIdx:     i,
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
			// Color each bar with the same Tableau10 palette (by rank) the sibling
			// donut uses, so bar #1 and the donut's biggest slice share a hue and the
			// two charts in this card read as one picture (G9.1a cohesion).
			if len(spec.Series) > 0 {
				for i := range spec.Series[0].Points {
					spec.Series[0].Points[i].Color = tableau10(i)
				}
			}
			catBarNodes = append(catBarNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Top spending categories ranked by amount", CurrencySymbol: currency.Symbol(base)}))
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
			catDonutNodes = append(catDonutNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Spending share by category", CurrencySymbol: currency.Symbol(base)}))
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
	var maxMember int64
	for _, ms := range memberSpend {
		if a := absI64(ms.Amount); a > maxMember {
			maxMember = a
		}
	}
	var memberNodes []ui.Node
	for _, ms := range memberSpend {
		name := memberName[ms.MemberID]
		if name == "" {
			name = uistate.T("reports.noMember")
		}
		memberNodes = append(memberNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), name), shareBar(ms.Amount, maxMember)),
			Span(css.Class("budget-amount"), fmtMinor(ms.Amount)),
		))
	}

	// Biggest deposits: the largest individual income transactions this period.
	bigIncome, _ := reports.LargestIncome(txns, cs, ce, rates, 8)
	var maxBigIncome int64
	for _, e := range bigIncome {
		if a := absI64(e.Amount); a > maxBigIncome {
			maxBigIncome = a
		}
	}
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
				shareBar(e.Amount, maxBigIncome),
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
	// R52(a): one-sentence takeaway for the Income-by-source card, mirroring the
	// Spending card's narrative so income reads as a decision, not just rows+charts:
	// names the top source and its share of total income.
	incomeTakeaway := ""
	if len(incomeRows) > 0 && incomeRows[0].Amount > 0 {
		var incomeTotal int64
		for _, r := range incomeRows {
			incomeTotal += r.Amount
		}
		top := incomeRows[0]
		if incomeTotal > 0 {
			pct := top.Amount * 100 / incomeTotal
			incomeTakeaway = uistate.T("reports.incomeTakeaway", nameOf(top.CategoryID), fmtMinor(top.Amount), pct)
		}
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
			incomeDonutNodes = append(incomeDonutNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Income share by source", CurrencySymbol: currency.Symbol(base)}))
		}
	}

	// V8: ranked bar chart for income by source — mirrors the spending bar/donut
	// pattern so the Income section has the same visual depth as Spending. Top 8
	// sources by absolute amount; Tableau10 colors by rank (matching the sibling
	// donut palette for cohesion).
	var incomeBarNodes []ui.Node
	{
		var barPairs []struct {
			Label  string
			Amount int64
		}
		for i, r := range incomeRows {
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
			if len(spec.Series) > 0 {
				for i := range spec.Series[0].Points {
					spec.Series[0].Points[i].Color = tableau10(i)
				}
			}
			incomeBarNodes = append(incomeBarNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Top income sources ranked by amount", CurrencySymbol: currency.Symbol(base)}))
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
			payeeBarNodes = append(payeeBarNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Top payees ranked by amount", CurrencySymbol: currency.Symbol(base)}))
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
			expenseBarNodes = append(expenseBarNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Biggest individual expenses ranked by amount", CurrencySymbol: currency.Symbol(base)}))
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

	// G9.1 hero delta: signed change in Net vs the previous comparable period, shown
	// as a small chip beside the headline so the figure has context ("vs last period")
	// rather than standing alone. Up in Net is good (pos tone), down is neg.
	var netDeltaChip ui.Node = Fragment()
	if prevFlowOK {
		delta := flow.Net() - prevFlow.Net()
		if delta != 0 {
			arrow, tone := "▲", "pos"
			if delta < 0 {
				arrow, tone = "▼", "neg"
			}
			mag := delta
			if mag < 0 {
				mag = -mag
			}
			netDeltaChip = Span(ClassStr("hero-net-delta "+tone),
				Attr("title", uistate.T("reports.vsPrevPeriod")),
				arrow+" "+fmtMoney(money.New(mag, base))+" "+uistate.T("reports.vsPrev"))
		}
	}

	// Money-flow Sankey (C70): income fans out to each spending category, with the
	// leftover going to Savings. Mermaid renders the flow value as the node label
	// (with a "$" prefix from the sankey config), so the value must be a human-scale
	// MAJOR-unit amount — not raw minor units, which read as "Income 406800" instead
	// of "Income $4,068". Round minor→major (whole currency units) here; relative
	// widths are preserved and sub-unit flows round to 0 and are skipped downstream.
	sankeyFactor := int64(1)
	for i := 0; i < decimals; i++ {
		sankeyFactor *= 10
	}
	toMajor := func(minor int64) int64 { return (minor + sankeyFactor/2) / sankeyFactor }
	var moneyFlows []mermaid.SankeyFlow
	for _, r := range rows {
		if v := absI64(r.Amount); v > 0 {
			moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: "Income", To: nameOf(r.CategoryID), Value: toMajor(v)})
		}
	}
	if sav := flow.Net(); sav > 0 {
		moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: "Income", To: "Savings", Value: toMajor(sav)})
	}

	// G9.1: Advanced disclosure toggle state (wraps custom field + deductible cards).
	showAdvanced := ui.UseState(false)
	onToggleAdvanced := ui.UseEvent(func() { showAdvanced.Set(!showAdvanced.Get()) })
	advancedCaret := uiw.Icon(icon.ChevronDown, css.Class(tw.W4, tw.H4, tw.ShrinkO))
	if showAdvanced.Get() {
		advancedCaret = uiw.Icon(icon.ArrowUp, css.Class(tw.W4, tw.H4, tw.ShrinkO))
	}

	// C243 [F33]: report-type selector — four tabbed views so the user can jump
	// directly to the section they care about instead of scrolling a mega-page.
	// Defaults to "overview" (the cash-flow + spending summary the user sees first).
	reportView := ui.UseState("overview")

	// W-15: count-up the hero figures (Net / Income / Spend) when they change, reusing
	// the dashboard's countup.js scanner. Keyed on the three amounts so it fires on
	// mount and on real changes only; the scanner is a no-op under reduced-motion /
	// data-wonder=off (it just sets the final text). Hook runs before the early return
	// below so its position stays stable across renders.
	heroSig := fmt.Sprintf("%d|%d|%d", net.Amount, flow.Income, flow.Expense)
	ui.UseEffect(func() func() {
		if fn := js.Global().Get("cashfluxCountUpScan"); fn.Type() == js.TypeFunction {
			fn.Invoke()
		}
		return nil
	}, heroSig)

	// R-8: with no income and no spend in the window there's nothing to report — show
	// a single empty-state CTA instead of a page of all-zero figures and charts.
	if flow.Income == 0 && flow.Expense == 0 {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("reports.empty"), CTALabel: uistate.T("reports.addFirst"), Href: "/transactions"})
	}

	// C237: Build the hero coverage caption and the YoY toggle label from the
	// current comparison mode so both reflect whether YoY is active.
	var coveringLine string
	if yoyMode.Get() {
		coveringLine = uistate.T("reports.coveringYoY", pr.FormatDate(cs), pr.FormatDate(ce), pr.FormatDate(ps), pr.FormatDate(pe))
	} else {
		coveringLine = uistate.T("reports.covering", pr.FormatDate(cs), pr.FormatDate(ce), pr.FormatDate(ps), pr.FormatDate(pe))
	}
	yoyLabelKey := "reports.yoyOff"
	if yoyMode.Get() {
		yoyLabelKey = "reports.yoyOn"
	}

	return Div(
		// G9.1 Item 1 — Hero zone: Net / Income / Spend as a prominent headline strip
		// above the card flow. Savings rate, runway, no-spend days go in a secondary row.
		Div(css.Class("reports-hero"),
			P(css.Class("hero-period"), coveringLine),
			Div(css.Class("hero-main"),
				Div(
					P(css.Class("hero-flanker-label"), uistate.T("reports.net")),
					P(ClassStr("hero-net "+accentFor(net)), Attr("data-countup", ""), fmtMoney(net)),
					netDeltaChip,
				),
				Div(css.Class("hero-flankers"),
					Div(css.Class("hero-flanker"),
						Span(css.Class("hero-flanker-label"), uistate.T("dashboard.income")),
						Span(css.Class("hero-flanker-value", "pos"), Attr("data-countup", ""), fmtMoney(money.New(flow.Income, base))),
					),
					Div(css.Class("hero-flanker"),
						Span(css.Class("hero-flanker-label"), uistate.T("dashboard.spending")),
						Span(css.Class("hero-flanker-value", "neg"), Attr("data-countup", ""), fmtMoney(money.New(flow.Expense, base))),
					),
				),
			),
			Div(css.Class("hero-secondary"),
				// G9.1: surface Net worth (the household's headline balance) in the first
				// viewport with its most-recent monthly change, instead of only deep in a
				// trends card. Net stays the page hero; this is a secondary reference stat.
				If(len(accounts) > 0, Div(css.Class("hero-stat"), Attr("data-testid", "reports-hero-networth"),
					Span(css.Class("hero-stat-label"), uistate.T("dashboard.netWorth")),
					Span(ClassStr("hero-stat-value "+accentFor(nwNet)), fmtMoney(nwNet)),
					func() ui.Node {
						if len(nwSeries) < 2 || nwChange == 0 {
							return Fragment()
						}
						arrow, tone := "▲", "pos"
						mag := nwChange
						if nwChange < 0 {
							arrow, tone, mag = "▼", "neg", -nwChange
						}
						return Span(ClassStr("hero-stat-sub "+tone), Attr("title", uistate.T("reports.vsPrevPeriod")),
							arrow+" "+fmtMoney(money.New(mag, base))+" "+uistate.T("reports.vsPrev"))
					}(),
				)),
				Div(css.Class("hero-stat"),
					Span(css.Class("hero-stat-label"), uistate.T("dashboard.savingsRate")),
					Span(ClassStr("hero-stat-value "+toneForSavingsRate(flow.SavingsRate())), fmt.Sprintf("%d%%", flow.SavingsRate())),
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
		// R-7: one page-level Export control with labeled options, replacing the six
		// per-card download buttons that previously cluttered each card footer.
		func() ui.Node {
			csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
			taxYear := w.From.Year()
			if w.Res != period.Year {
				taxYear = time.Now().Year()
			}
			ys := time.Date(taxYear, time.January, 1, 0, 0, 0, 0, time.UTC)
			ye := time.Date(taxYear+1, time.January, 1, 0, 0, 0, 0, time.UTC)
			memberNm := func(id string) string {
				if n := memberName[id]; n != "" {
					return n
				}
				return uistate.T("reports.noMember")
			}
			opt := func(label string, on func()) ui.Node {
				return Button(css.Class("btn", tw.WFull, tw.TextLeft), Type("button"), OnClick(on), label)
			}
			return Details(css.Class("reports-export", tw.Mt2),
				Summary(css.Class("btn", "btn-sm"), Style(map[string]string{"cursor": "pointer", "width": "fit-content"}), uistate.T("reports.export")),
				Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mt1), Style(map[string]string{"max-width": "320px"}),
					opt(uistate.T("reports.byCategory"), func() {
						downloadBytes(reports.ExportFilename("spending-by-category", w.Res, w.From), "text/csv", reports.CategoryCSV(rows, nameOf, csvAmount))
					}),
					opt(uistate.T("reports.incomeBySource"), func() {
						downloadBytes(reports.ExportFilename("income-by-source", w.Res, w.From), "text/csv", reports.CategoryCSV(incomeRows, nameOf, csvAmount))
					}),
					opt(uistate.T("reports.topPayees"), func() {
						downloadBytes(reports.ExportFilename("top-payees", w.Res, w.From), "text/csv", reports.PayeeCSV(payees, csvAmount))
					}),
					opt(uistate.T("reports.biggestExpenses"), func() {
						downloadBytes(reports.ExportFilename("largest-expenses", w.Res, w.From), "text/csv", reports.LargestExpensesCSV(largest, nameOf, csvAmount))
					}),
					opt(uistate.T("reports.byMember"), func() {
						downloadBytes(reports.ExportFilename("spending-by-member", w.Res, w.From), "text/csv", reports.MemberCSV(memberSpend, memberNm, csvAmount))
					}),
					opt(uistate.T("reports.taxSummary"), func() {
						summary, _ := reports.YearTax(txns, taxYear, ys, ye, rates)
						downloadBytes(reports.ExportFilename("tax-summary", period.Year, ys), "text/csv", reports.YearTaxCSV(summary, nameOf, csvAmount))
					}),
				),
			)
		}(),
		// C236: "Save as PDF" button opens the browser print dialog, which lets users
		// save the current report as a PDF without any server-side dependency.
		func() ui.Node {
			printReport := ui.UseEvent(func(_ ui.Event) { js.Global().Call("print") })
			return Button(css.Class("btn", "btn-sm", tw.Mt1), Type("button"), OnClick(printReport),
				uistate.T("reports.saveAsPDF"))
		}(),
		If(spendStats.Count > 0, P(css.Class("muted"), uistate.T("reports.spendStats", spendStats.Count, fmtMinor(spendStats.Average), fmtMinor(spendStats.Median)))),
		// G9.1 Item 3 — Heads-up anomaly card gets .card-alert urgency border.
		If(len(anomalyNodes) > 0, uiw.Card(uiw.CardProps{
			ClassParts: []any{"card-alert"},
			Header:     H2(css.Class("card-title"), uistate.T("reports.headsUp")),
			Body:       Div(anomalyNodes),
		})),
		// C243 [F33]: report-type selector — segmented control that gates each major
		// section so users jump to what they care about instead of scrolling the whole
		// page. Uses the same ui.Segmented/radiogroup pattern as the period selector.
		Div(css.Class(tw.Mt2, tw.Mb1),
			uiw.Segmented(uiw.SegmentedProps{
				Label:    "Report type",
				Selected: reportView.Get(),
				OnSelect: func(v string) { reportView.Set(v) },
				Options: []uiw.SegOption{
					{Value: "overview", Label: uistate.T("reports.viewOverview")},
					{Value: "categories", Label: uistate.T("reports.viewCategories")},
					{Value: "networth", Label: uistate.T("reports.viewNetWorth")},
					{Value: "advanced", Label: uistate.T("reports.viewAdvanced")},
				},
			}),
		),
		// Pre-compute each view into a variable, then show exactly one based on the
		// selected tab. No On* hooks are called inside this selection — all hooks are
		// registered at stable positions above in the function body.
		func() ui.Node {
			// ── Overview: cash-flow Sankey + top payees + biggest expenses ──────────
			// The wide cash-flow Sankey stays full-width above; the ranked-list cards
			// (payees / expenses / deposits / income / by-member) pair into a responsive
			// 2-column grid on wide viewports (`.reports-grid`) so the overview reads as
			// a dashboard instead of a single long column with wasted horizontal space.
			// Each If(...) collapses to an empty Fragment when its data is absent, so a
			// missing section leaves no empty grid cell.
			overviewSection := Fragment(
				If(len(moneyFlows) > 1, uiw.EntityListSection(uiw.EntityListSectionProps{
					Title: "Money flow",
					Body:  uiw.Mermaid(uiw.MermaidProps{Source: mermaid.Sankey(moneyFlows), Label: "Income to spending categories money-flow", ValuePrefix: currency.Symbol(base)}),
				})),
				Div(css.Class("reports-grid"),
					If(len(payeeNodes) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
						Title: uistate.T("reports.topPayees"),
						Body: Fragment(
							If(len(payeeBarNodes) > 0, Div(payeeBarNodes)),
							Div(css.Class("rows"), payeeNodes),
						),
					})),
					If(len(largestNodes) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
						Title: uistate.T("reports.biggestExpenses"),
						Body: Fragment(
							If(len(expenseBarNodes) > 0, Div(expenseBarNodes)),
							Div(css.Class("rows"), largestNodes),
						),
					})),
					// Income breakdown sits in overview too: biggest deposits + by-source.
					If(len(bigIncomeNodes) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
						Title: uistate.T("reports.biggestDeposits"),
						Rows:  bigIncomeNodes,
					})),
					If(len(incomeNodes) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
						Title: uistate.T("reports.incomeBySource"),
						Body: Fragment(
							If(incomeTakeaway != "", P(css.Class("muted"), Attr("data-testid", "income-takeaway"), incomeTakeaway)),
							If(len(incomeBarNodes) > 0, Div(incomeBarNodes)),
							If(len(incomeDonutNodes) > 0, Div(incomeDonutNodes)),
							Div(css.Class("rows"), incomeNodes),
						),
					})),
					If(len(app.Members()) >= 2 && len(memberSpend) >= 1, uiw.EntityListSection(uiw.EntityListSectionProps{
						Title: uistate.T("reports.byMember"),
						Body:  Div(css.Class("rows"), memberNodes),
					})),
				),
			)
			// ── Categories: spending-by-category bar/donut + ranked rows ────────────
			categoriesSection := Fragment(
				uiw.Card(uiw.CardProps{
					Header: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.FlexWrap, tw.Gap2),
						H2(css.Class("card-title"), uistate.T("reports.byCategory")),
						Div(css.Class(tw.Flex, tw.Gap2),
							// C237: Year-over-Year comparison toggle.
							Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "reports-yoy-toggle"),
								Attr("aria-pressed", boolStr(yoyMode.Get())),
								Title(uistate.T("reports.yoyTitle")), OnClick(onToggleYoY),
								uistate.T(yoyLabelKey)),
							Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "reports-rollup-toggle"),
								Attr("aria-pressed", boolStr(rollupCats.Get())),
								Title(uistate.T("reports.rollupTitle")), OnClick(onToggleRollup),
								uistate.T(rollupLabelKey(rollupCats.Get()))),
						),
					),
					Body: Fragment(
						P(css.Class("muted"), narrative),
						If(weekdayPeakLine != "", P(css.Class("muted"), weekdayPeakLine)),
						// The ranked bar (magnitude) and donut (share) are two views of the
						// same data — pair them side-by-side on wide screens so the card reads
						// as one picture instead of two stacked 200px charts; they stack on
						// narrow screens. Ranked rows stay full-width below.
						If(len(catBarNodes) > 0 || len(catDonutNodes) > 0, Div(css.Class("reports-chart-pair"),
							If(len(catBarNodes) > 0, Div(catBarNodes)),
							If(len(catDonutNodes) > 0, Div(catDonutNodes)),
						)),
						catBody,
					),
				}),
			)
			// ── Net worth: NW composition (headline, full-width) + the two supporting
			// trend charts (cash-flow, savings-rate) paired side-by-side on wide
			// screens via .reports-grid, so the tab reads as a dashboard. ──────────
			netWorthSection := Fragment(
				// R-11: NW composition + trend as a single headline card.
				// C218: HTML id anchor so /networth can deep-link to this section.
				If(len(accounts) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
					Title: uistate.T("dashboard.netWorth"),
					Attrs: []any{Attr("id", "networth")},
					// R52(b): a nearby drill-down — net worth composes from accounts, so
					// link straight to /accounts to see (and adjust) what's behind the figure.
					HeaderAction: A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/accounts")),
						Attr("data-testid", "networth-drill"), uistate.T("reports.viewAccounts")),
					Body: Fragment(
						Div(css.Class("stat-grid"),
							stat(uistate.T("accounts.assets"), fmtMoney(nwAssets), "pos"),
							stat(uistate.T("dashboard.liabilities"), fmtMoney(nwLiab), "neg"),
							stat(uistate.T("dashboard.netWorth"), fmtMoney(nwNet), accentFor(nwNet)),
							If(len(nwSeries) >= 2, stat(uistate.T("reports.netWorthChange"), fmtMoney(money.New(nwChange, base)), accentFor(money.New(nwChange, base)))),
						),
						// C217: NW trend uses its own monthly labels, not the cash-flow period labels.
						If(len(nw) >= 2, Fragment(
							P(css.Class("muted"), uistate.T("reports.nwTrendMonthly", trendBuckets)),
							uiw.AreaChart(uiw.AreaChartProps{Values: nw, Stroke: "#7c83ff", GradientID: "nw-reports", Label: uistate.T("dashboard.netWorthTrend"), Labels: nwLabels, ValueLabels: nwValueLabels}),
						)),
					),
				})),
				// Cash-flow + savings-rate trends are two supporting period charts — pair
				// them side-by-side on wide screens (stack below 1100px). Each If(...)
				// collapses to an empty Fragment, leaving no empty grid cell.
				If(len(netSeries) >= 2 || len(srSeries) >= 2, Div(css.Class("reports-grid"),
					If(len(netSeries) >= 2, uiw.EntityListSection(uiw.EntityListSectionProps{
						Title: uistate.T("dashboard.cashFlow"),
						Body: Fragment(
							// R52(a): lead with the insight sentence; the period span is a quiet sub-line.
							If(cashFlowTakeaway != "", P(ClassStr("budget-sub "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "cashflow-takeaway"), cashFlowTakeaway)),
							P(css.Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
							uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, GradientID: "cf-reports", Label: uistate.T("dashboard.cashFlow"), Labels: trendLabels, ValueLabels: moneyLabels(netSeries)}),
						),
					})),
					If(len(srSeries) >= 2, uiw.EntityListSection(uiw.EntityListSectionProps{
						Title: uistate.T("reports.savingsTrend"),
						Body: Fragment(
							If(savingsTakeaway != "", P(ClassStr("budget-sub "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "savings-takeaway"), savingsTakeaway)),
							P(css.Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
							uiw.AreaChart(uiw.AreaChartProps{Values: srSeries, GradientID: "sr-reports", Label: uistate.T("reports.savingsTrend"), Labels: trendLabels, ValueLabels: pctLabels(srSeries)}),
						),
					})),
				)),
			)
			// ── Advanced: custom-field spend + deductible totals ─────────────────────
			// The disclosure toggle (showAdvanced) is preserved so the section stays
			// collapsible when the user lands here and wants a quick scan first.
			advancedSection := If(len(cfDefs) > 0,
				Div(
					Button(css.Class("disclosure-toggle"), Type("button"),
						Attr("aria-expanded", boolStr(showAdvanced.Get())),
						OnClick(onToggleAdvanced),
						"Advanced ", advancedCaret,
					),
					If(showAdvanced.Get(),
						Div(
							customFieldSpendSection(txns, cfDefs, selectedCFKey.Get(), onCFKeyChange, cs, ce, rates, base, fmtMinor, w),
							deductibleSection(txns, cats, cs, ce, rates, base, fmtMinor, w),
						),
					),
				),
			)

			// reportsTabEmpty renders a calm, centered note when a selected tab has
			// no data, so the area below the segmented control never goes silently
			// blank (e.g. Advanced with no custom fields, Net worth with no accounts).
			reportsTabEmpty := func(msg string) ui.Node {
				return Div(css.Class("muted"), Attr("data-testid", "reports-tab-empty"),
					Style(map[string]string{"text-align": "center", "padding": "2.5rem 1rem", "max-width": "32rem", "margin": "0 auto"}),
					msg)
			}

			switch reportView.Get() {
			case "categories":
				return categoriesSection
			case "networth":
				if len(netSeries) < 2 && len(accounts) == 0 && len(srSeries) < 2 {
					return reportsTabEmpty(uistate.T("reports.emptyNetWorth"))
				}
				return netWorthSection
			case "advanced":
				if len(cfDefs) == 0 {
					return reportsTabEmpty(uistate.T("reports.emptyAdvanced"))
				}
				return advancedSection
			default: // "overview"
				if len(moneyFlows) <= 1 && len(payeeNodes) == 0 && len(largestNodes) == 0 &&
					len(bigIncomeNodes) == 0 && len(incomeNodes) == 0 && len(memberNodes) == 0 {
					return reportsTabEmpty(uistate.T("reports.emptyOverview"))
				}
				return overviewSection
			}
		}(),
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
		bar := Div(Style(map[string]string{"height": "4px", "max-width": "100%", "margin-top": "0.3rem", "background": "var(--border)", "border-radius": "999px", "overflow": "hidden"}),
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

	return uiw.Card(uiw.CardProps{
		TestID: "customfield-spend-section",
		Header: H2(css.Class("card-title"), sectionLabel),
		Body: Fragment(
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
		),
	})
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
	PriorZero  bool // C238: comparison ran but prior period was zero and current > 0
	MaxCat     int64
	CatIdx     int // R-10: rank index → distinct share-bar hue
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
	switch {
	case props.PriorZero:
		// C238: category had spend this period but zero in the prior period — show
		// "new" instead of hiding the badge entirely (can't divide by zero for a %).
		delta = Span(ClassStr("row-meta "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)+" "+tw.ColorClass("text-down")),
			Text(uistate.T("reports.new")))
	case props.HasDelta && props.Amount != props.Prior:
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
			reportsCatShareBar(props.Amount, props.MaxCat, props.CatIdx),
		),
		delta,
		Span(css.Class("budget-amount"), props.FmtMinor(props.Amount)),
	)
}

// reportsCatShareBar is the category-row proportion bar (R-10): like the generic
// shareBar but each rank gets a distinct hue derived from its index via an inline
// --cat-idx var, so the ranked categories read as a color-coded set rather than a
// wall of identical accent bars. 47° steps spread the wheel without close repeats.
func reportsCatShareBar(amount, max int64, idx int) ui.Node {
	if max <= 0 {
		return Fragment()
	}
	if amount < 0 {
		amount = -amount
	}
	pct := int(amount * 100 / max)
	if pct > 100 {
		pct = 100
	}
	// Color each row by its rank with the SAME Tableau10 palette the sibling bar
	// chart and donut use, so a category reads as one hue across all three views in
	// the card (e.g. Mortgage is blue everywhere) instead of three palettes (G9.1a).
	return Div(css.Class("share-bar"), Style(map[string]string{"height": "8px", "max-width": "100%",
		"margin-top": "0.3rem", "background": "var(--border)", "border-radius": "999px", "overflow": "hidden"}),
		Div(Style(map[string]string{"height": "100%", "width": fmt.Sprintf("%d%%", pct),
			"background": tableau10(idx), "border-radius": "999px"})))
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
		bar := Div(Style(map[string]string{"height": "4px", "max-width": "100%", "margin-top": "0.3rem", "background": "var(--border)", "border-radius": "999px", "overflow": "hidden"}),
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

	return uiw.Card(uiw.CardProps{
		TestID: "deductible-section",
		Header: H2(css.Class("card-title"), uistate.T("reports.deductibleTitle")),
		Body: Fragment(
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
		),
	})
}

// NetWorth is the dedicated /networth view. It delegates to Reports() — the
// net-worth section inside Reports carries id="networth" (C218) so the shell
// can scroll to it when navigating to this route directly.
func NetWorth() ui.Node { return Reports() }
