// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"strings"
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
	"github.com/monstercameron/CashFlux/internal/scope"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
// The default series color is overridden per-point by every reports call site
// (Tableau10 rank hues for the categorical charts, the live theme accent for the
// ranked payee/expense charts) so the charts track the active theme.
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

// reportsAccentBar recolors every bar of a spec with the resolved theme accent,
// so a single-hue ranked chart (payees, biggest expenses) tracks the active
// theme instead of a hardcoded blue.
func reportsAccentBar(spec chartspec.Spec, accent string) chartspec.Spec {
	if len(spec.Series) > 0 {
		spec.Series[0].Color = accent
		for i := range spec.Series[0].Points {
			spec.Series[0].Points[i].Color = accent
		}
	}
	return spec
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
// matching the sibling runway/no-spend-day stats in the hero chips.
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

// rptToneCls maps the "pos"/"neg" stat accents to the shared money color classes
// used inside the hero figure chips.
func rptToneCls(tone string) string {
	switch tone {
	case "pos":
		return " " + tw.ColorClass("text-up")
	case "neg":
		return " " + tw.ColorClass("text-down")
	}
	return ""
}

// rptTile wraps a tile body in the shared Widget chrome at an explicit bento
// column placement ("1 / span 4" full-width, "span 2" for a half-width pair that
// auto-flows beside its partner).
func rptTile(tid, col string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: tid, Title: "", GridColumn: col, Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// rptSection wraps a tile body with a serif section title + optional header
// action, reusing the debt-section chrome so /reports matches the other
// redesigned surfaces (/debt, /investments, /planning, /recurring).
func rptSection(sid, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if sid != "" {
		args = append(args, Attr("id", sid))
	}
	if title != "" {
		args = append(args, Div(css.Class("debt-section-head"),
			H2(css.Class("debt-section-title"), title),
			If(action != nil, action),
		))
	}
	args = append(args, body)
	return Div(args...)
}

// rptChip renders one headline figure chip (the shared debt-stat chrome), with
// optional extra nodes (e.g. a small delta sub-line) below the value.
func rptChip(label, value, valueCls string, extra ...ui.Node) ui.Node {
	args := []any{css.Class("debt-stat"),
		Div(css.Class("debt-stat-label", tw.TextDim), label),
		Div(ClassStr("debt-stat-value "+tw.Fold(tw.FontDisplay)+valueCls), value),
	}
	for _, e := range extra {
		args = append(args, e)
	}
	return Div(args...)
}

// Reports is the read-only reporting screen (B21), redesigned as a widgetized
// bento surface: a hero tile (net / income / spending in the display serif with
// figure chips), a toolbar tile (report-type tabs, the scope filter, a metrics
// toggle, and the export menu), and per-view section tiles — all computed from
// the pure internal/reports core so the figures match the rest of the app. The
// same figures are exposed as report_* engine variables (engineenv.addReportsVars)
// so anything on this page can be referenced in a formula or dashboard widget.
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

	// #444: active scope atom — hook called at a stable position (hook 2) so the
	// chain never shifts across renders. Scope resolution happens after txns and
	// accounts are available below.
	scopeAtom := uistate.UseActiveScope()

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
	accounts := app.Accounts()

	// #444: resolve the active scope to a sorted set of in-scope account IDs,
	// then derive the filtered transaction slice used by all per-period spend and
	// series calculations. Net-worth calls keep the full txns so household NW is
	// always the household total regardless of the chosen scope.
	sc := scopeAtom.Get()
	instOf := func(a domain.Account) string { return a.Institution }
	scopeIDs := scope.ResolveScope(accounts, sc, instOf)
	scopedTxns := scope.ApplyScopeToTxns(txns, scopeIDs)

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

	// The reading posture (active tab, YoY comparison, sub-category roll-up) is
	// persisted (uistate.ReportsConfig) so /reports reopens how it was being read.
	cfg := uistate.ReportsConfigGet()

	// Roll sub-categories up into their top-level parent in the by-category
	// breakdown (L28). Off by default so sub-category detail stays visible.
	rollupCats := ui.UseState(cfg.Rollup)
	onToggleRollup := ui.UseEvent(func() { rollupCats.Set(!rollupCats.Get()) })

	// C237: Year-over-Year comparison toggle. When on, the comparison period is
	// exactly one calendar year prior (via reports.YoYPrior); when off, it is the
	// immediately preceding window of the same length (MoM / period-over-period).
	yoyMode := ui.UseState(cfg.YoY)
	onToggleYoY := ui.UseEvent(func() { yoyMode.Set(!yoyMode.Get()) })

	// C243 [F33]: report-type selector — four tabbed views so the user can jump
	// directly to the section they care about instead of scrolling a mega-page.
	reportView := ui.UseState(cfg.View)

	// Toolbar disclosure states: the scope filter (chips) and the opt-in
	// report-metrics FormulaBuilder tile.
	scopeOpen := ui.UseState(!sc.IsAll())
	onToggleScope := ui.UseEvent(Prevent(func() { scopeOpen.Set(!scopeOpen.Get()) }))
	showFormulas := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))

	// Export menu (labelled dropdown; the options are fixed-position buttons built
	// below once the data is computed). DismissPopover closes on Escape/outside
	// click; AnchorPopover keeps the menu inside the viewport.
	exportOpen := ui.UseState(false)
	onToggleExport := ui.UseEvent(Prevent(func() { exportOpen.Set(!exportOpen.Get()) }))
	// An item click bubbles up to the menu container and dismisses it (the
	// KebabMenu convention), so the menu doesn't linger over the tiles below.
	onCloseExport := ui.UseEvent(Prevent(func() { exportOpen.Set(false) }))
	uiw.DismissPopover(exportOpen.Get(), "rpt-export", func() { exportOpen.Set(false) })
	uiw.AnchorPopover(exportOpen.Get(), "rpt-export")

	// Persist the reading posture silently (a keyed effect; no data-revision bump).
	persistKey := fmt.Sprintf("%s|%t|%t", reportView.Get(), yoyMode.Get(), rollupCats.Get())
	ui.UseEffect(func() func() {
		uistate.SetReportsConfig(uistate.ReportsConfig{
			View: reportView.Get(), YoY: yoyMode.Get(), Rollup: rollupCats.Get(),
		})
		return nil
	}, persistKey)

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

	flow, _ := reports.IncomeVsExpense(scopedTxns, cs, ce, rates)
	rows, _ := reports.SpendingByCategory(scopedTxns, cs, ce, true, ps, pe, rates)

	// No-spend days: elapsed days in the period with zero spending (motivating).
	noSpendDays := reports.NoSpendDays(scopedTxns, cs, ce, time.Now())
	spendStats, _ := reports.SpendingStats(scopedTxns, cs, ce, rates)

	// Previous comparable period flow, computed once for both the headline spending
	// trend and the hero Net delta chip (G9.1: period-over-period context).
	prevFlow, prevFlowErr := reports.IncomeVsExpense(scopedTxns, ps, pe, rates)
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
	accent := chartLineColor(uistate.CurrentAccent())
	weekStart := pr.WeekStartWeekday()
	startCur := period.Truncate(w.Res, w.From, weekStart)
	bounds := make([]time.Time, 0, trendBuckets+1)
	for k := 0; k <= trendBuckets; k++ {
		bounds = append(bounds, period.Step(w.Res, startCur, k-(trendBuckets-1)))
	}
	flows, _ := reports.IncomeExpenseSeries(scopedTxns, bounds, rates)
	netSeries := make([]float64, len(flows))
	for i, f := range flows {
		netSeries[i] = float64(f.Net())
	}
	// R-4: x-axis period captions for the trend chart, one per bucket (from bounds).
	trendLabels := make([]string, 0, len(netSeries))
	for i := 0; i < len(netSeries) && i < len(bounds); i++ {
		trendLabels = append(trendLabels, bounds[i].Format("Jan"))
	}

	// Net-worth snapshot and period change — used in the hero figure chips. The
	// full stat-grid + composition bar + trend chart are rendered by the shared
	// NetWorthPanel component (FEATURE_MAP §5.7b); only nwNet and nwChange stay
	// here. NW uses the full unscoped txns so the household balance-sheet is
	// always complete. The trend is always monthly (C217): net worth is a
	// cumulative point-in-time series, so re-bucketing it makes no sense.
	curMonth := dateutil.MonthStart(time.Now())
	nwBounds := make([]time.Time, 0, trendBuckets+1)
	for k := 0; k <= trendBuckets; k++ {
		nwBounds = append(nwBounds, dateutil.AddMonths(curMonth, k-trendBuckets))
	}
	nwSeries, _ := ledger.NetWorthSeries(accounts, txns, nwBounds, rates)
	nwNet, _, _, _ := ledger.NetWorth(accounts, txns, rates)
	var nwChange int64
	if n := len(nwSeries); n >= 2 {
		nwChange = nwSeries[n-1].Amount - nwSeries[n-2].Amount
	}

	// Savings-rate trend: percent of income kept per period.
	srInts, _ := reports.SavingsRateSeries(scopedTxns, bounds, rates)
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
	// doesn't understate spending). Liquid = cash-type accounts only. The same
	// figure is exposed as the report_runway_months engine variable.
	const runwayMonths = 6
	liquid, _ := ledger.LiquidBalance(accounts, scopedTxns, rates)
	monthBounds := make([]time.Time, 0, runwayMonths+1)
	for k := 0; k <= runwayMonths; k++ {
		monthBounds = append(monthBounds, dateutil.AddMonths(curMonth, k-runwayMonths))
	}
	monthFlows, _ := reports.IncomeExpenseSeries(scopedTxns, monthBounds, rates)
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
	// share of the list's largest, so the distribution is scannable at a glance (C55).
	shareBar := func(amount, max int64) ui.Node {
		if max <= 0 {
			return Fragment()
		}
		pct := int(absI64(amount) * 100 / max)
		if pct > 100 {
			pct = 100
		}
		return Div(css.Class("share-bar"),
			Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
	}
	narrative := reports.SpendingNarrative(rows, true, fmtMinor, func(id string) string { return catName[id] })

	// Heads-up: categories spending well above their recent monthly norm (top 3).
	// Reuses the shared insights detector (also behind the Insights highlights and
	// dashboard widget), filtered to overspending.
	var anomalyNodes []ui.Node
	for _, a := range detectSpendingAnomalies(scopedTxns, cats, rates) {
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
	// Categories that dropped to zero this period (spent last period, nothing now)
	// fold behind a quiet disclosure — a wall of "$0.00 ↓100%" rows would drown
	// the categories that actually have spending.
	var rowNodes, zeroedNodes []ui.Node
	for i, r := range rows {
		if r.Amount == 0 && r.Prior == 0 {
			continue
		}
		target := &rowNodes
		if r.Amount == 0 {
			target = &zeroedNodes
		}
		*target = append(*target, ui.CreateElement(reportsCatRow, reportsCatRowProps{
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
	if len(rowNodes) == 0 && len(zeroedNodes) == 0 {
		catBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("reports.empty"), CTALabel: uistate.T("reports.addFirst"), Href: "/transactions"})
	} else {
		catBody = Fragment(
			Div(css.Class("rows"), rowNodes),
			If(len(zeroedNodes) > 0, Details(css.Class("rpt-zeroed"), Attr("data-testid", "reports-zeroed"),
				Summary(uistate.T("reports.zeroedSummary", len(zeroedNodes))),
				Div(css.Class("rows"), zeroedNodes),
			)),
		)
	}

	// Top payees: where the money went by merchant/description this period.
	payees, _ := reports.TopPayees(scopedTxns, cs, ce, rates, 8)
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
	largest, _ := reports.LargestExpenses(scopedTxns, cs, ce, rates, 8)
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
	memberSpend, _ := reports.SpendingByMember(scopedTxns, cs, ce, rates)
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
	bigIncome, _ := reports.LargestIncome(scopedTxns, cs, ce, rates, 8)
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
	incomeRows, _ := reports.IncomeByCategory(scopedTxns, cs, ce, rates)
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

	// V7: ranked bar charts for top payees and biggest expenses — single-hue
	// ranked magnitudes, colored with the live theme accent (not a hardcoded blue).
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
			spec := reportsAccentBar(reportsBarSpec(barPairs, decimals), accent)
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
			spec := reportsAccentBar(reportsBarSpec(barPairs, decimals), accent)
			expenseBarNodes = append(expenseBarNodes, uiw.Chart(uiw.ChartProps{Spec: spec, Height: "200px", Label: "Biggest individual expenses ranked by amount", CurrencySymbol: currency.Symbol(base)}))
		}
	}

	// Spending-by-weekday insight: which day money tends to leave.
	weekdayPeakLine := ""
	if wd, err := reports.SpendingByWeekday(scopedTxns, cs, ce, rates); err == nil {
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
			netDeltaChip = Span(ClassStr("rpt-delta "+tone),
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
	// ONLY when no scope filter is active: a scope that matches nothing must keep
	// the toolbar (and its Scope chips) on screen, or the user can't un-scope.
	if flow.Income == 0 && flow.Expense == 0 && sc.IsAll() {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("reports.empty"), CTALabel: uistate.T("reports.addFirst"), Href: "/transactions"})
	}

	// C237: the hero coverage caption reflects whether YoY is active.
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

	// ── Hero tile: the net figure in the display serif + the figure chips. ──────
	// The chips carry the supporting stats (income, spending, net worth, savings
	// rate) plus the most timely fact — a thin cash runway (danger-toned) or the
	// motivating no-spend-day count.
	var nwSub ui.Node = Fragment()
	if len(nwSeries) >= 2 && nwChange != 0 {
		arrow, tone := "▲", "text-up"
		mag := nwChange
		if nwChange < 0 {
			arrow, tone, mag = "▼", "text-down", -nwChange
		}
		nwSub = Div(ClassStr("rpt-chip-sub "+tw.ColorClass(tone)), Attr("title", uistate.T("reports.vsPrevPeriod")),
			arrow+" "+fmtMoney(money.New(mag, base)))
	}
	heroChips := []ui.Node{
		rptChip(uistate.T("dashboard.income"), fmtMoney(money.New(flow.Income, base)), rptToneCls("pos")),
		rptChip(uistate.T("dashboard.spending"), fmtMoney(money.New(flow.Expense, base)), rptToneCls("neg")),
	}
	// Level stats (net worth, a healthy runway, no-spend days) stay NEUTRAL so
	// color keeps meaning: green/red mark direction and warnings, not decoration.
	// Net worth's tone lives in its delta sub-line; runway tones only when thin.
	if len(accounts) > 0 {
		nwTone := ""
		if accentFor(nwNet) == "neg" {
			nwTone = rptToneCls("neg")
		}
		heroChips = append(heroChips, Div(css.Class("debt-stat"), Attr("data-testid", "reports-hero-networth"),
			Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("dashboard.netWorth")),
			Div(ClassStr("debt-stat-value "+tw.Fold(tw.FontDisplay)+nwTone), fmtMoney(nwNet)),
			nwSub,
		))
	}
	heroChips = append(heroChips, rptChip(uistate.T("dashboard.savingsRate"), fmt.Sprintf("%d%%", flow.SavingsRate()), rptToneCls(toneForSavingsRate(flow.SavingsRate()))))
	if burn > 0 {
		runwayTone := ""
		if accentForRunway(runway.Months) == "neg" {
			runwayTone = rptToneCls("neg")
		}
		heroChips = append(heroChips, rptChip(uistate.T("reports.runway"), uistate.T("reports.runwayMonths", runway.Months), runwayTone))
	}
	if noSpendDays > 0 {
		heroChips = append(heroChips, rptChip(uistate.T("reports.noSpendDays"), fmt.Sprintf("%d", noSpendDays), ""))
	}
	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), coveringLine),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("reports.net")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)+rptToneCls(accentFor(net))), Attr("data-countup", ""), fmtMoney(net)),
				netDeltaChip,
			),
		),
		Div(css.Class("debt-chips"), heroChips),
		If(spendTrend != "", P(css.Class("muted", "rpt-hero-trend"), spendTrend)),
	)
	heroTile := rptTile("rpt-hero", "1 / span 4", rptSection("", uistate.T("reports.heroTitle"), nil, heroBody))

	// ── Toolbar tile: report-type tabs, the scope filter, metrics, export. ─────
	scopeCount := len(sc.Institutions) + len(sc.Owners) + len(sc.Types) + len(sc.AccountIDs)
	scopeLabel := uistate.T("reports.scope")
	if scopeCount > 0 {
		scopeLabel = uistate.T("reports.scopeCount", scopeCount)
	}
	scopeCls := "strip-toggle"
	if scopeOpen.Get() || scopeCount > 0 {
		scopeCls += " is-on"
	}
	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("reports.metricsShow")
	if showFormulas.Get() {
		metricsCls += " is-on"
		metricsLabel = uistate.T("reports.metricsHide")
	}

	// Export menu: one labelled dropdown holding every CSV export plus Save-as-PDF
	// (R-7: replaces per-card download buttons; C236: print → PDF, no server needed).
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
	exportItem := func(testID, label string, on func()) ui.Node {
		return Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", testID), OnClick(on), label)
	}
	exportHidden := ""
	if !exportOpen.Get() {
		exportHidden = " hidden-menu"
	}
	exportExpanded := "false"
	if exportOpen.Get() {
		exportExpanded = "true"
	}
	exportMenu := Div(css.Class("add-wrap"), Attr("id", "rpt-export"),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "reports-export-toggle"),
			Attr("aria-haspopup", "menu"), Attr("aria-expanded", exportExpanded),
			Title(uistate.T("reports.exportTitle")), OnClick(onToggleExport),
			uistate.T("reports.exportCsv")),
		Div(ClassStr("add-menu"+exportHidden), Attr("role", "menu"), OnClick(onCloseExport),
			exportItem("reports-export-category", uistate.T("reports.byCategory"), func() {
				downloadBytes(reports.ExportFilename("spending-by-category", w.Res, w.From), "text/csv", reports.CategoryCSV(rows, nameOf, csvAmount))
			}),
			exportItem("reports-export-income", uistate.T("reports.incomeBySource"), func() {
				downloadBytes(reports.ExportFilename("income-by-source", w.Res, w.From), "text/csv", reports.CategoryCSV(incomeRows, nameOf, csvAmount))
			}),
			exportItem("reports-export-payees", uistate.T("reports.topPayees"), func() {
				downloadBytes(reports.ExportFilename("top-payees", w.Res, w.From), "text/csv", reports.PayeeCSV(payees, csvAmount))
			}),
			exportItem("reports-export-largest", uistate.T("reports.biggestExpenses"), func() {
				downloadBytes(reports.ExportFilename("largest-expenses", w.Res, w.From), "text/csv", reports.LargestExpensesCSV(largest, nameOf, csvAmount))
			}),
			exportItem("reports-export-member", uistate.T("reports.byMember"), func() {
				downloadBytes(reports.ExportFilename("spending-by-member", w.Res, w.From), "text/csv", reports.MemberCSV(memberSpend, memberNm, csvAmount))
			}),
			exportItem("reports-export-tax", uistate.T("reports.taxSummary"), func() {
				summary, _ := reports.YearTax(scopedTxns, taxYear, ys, ye, rates)
				downloadBytes(reports.ExportFilename("tax-summary", period.Year, ys), "text/csv", reports.YearTaxCSV(summary, nameOf, csvAmount))
			}),
			exportItem("reports-export-pdf", uistate.T("reports.saveAsPDF"), func() {
				js.Global().Call("print")
			}),
		),
	)

	// The view tabs (navigation) sit alone on the left; the utility toggles
	// (scope filter, metrics panel) group with Export on the right so switching
	// views and opening drawers read as different kinds of control.
	toolbar := rptTile("rpt-toolbar", "1 / span 4", Fragment(
		Div(css.Class("filter-strip"),
			Div(css.Class("filter-strip-controls"),
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
			Div(css.Class("filter-strip-controls"),
				Button(ClassStr(scopeCls), Type("button"), Attr("aria-pressed", ariaBool(scopeOpen.Get())),
					Attr("data-testid", "reports-scope-toggle"), Title(uistate.T("reports.scopeHint")),
					OnClick(onToggleScope), Text(scopeLabel)),
				Button(ClassStr(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(showFormulas.Get())),
					Attr("data-testid", "reports-toggle-formulas"), Title(uistate.T("reports.metricsTitle")),
					OnClick(toggleFormulas), Text(metricsLabel)),
				exportMenu,
			),
		),
		// #444: the scope filter expands inside the toolbar — filter all report
		// figures by institution, owner, account type, or a saved view.
		If(scopeOpen.Get(), ui.CreateElement(ScopeSelector)),
	))

	// ── Heads-up tile: categories spending well above their norm (alert-toned). ─
	var headsUpTile ui.Node = Fragment()
	if len(anomalyNodes) > 0 {
		headsUpTile = rptTile("rpt-headsup", "1 / span 4",
			Div(css.Class("rpt-headsup"),
				rptSection("sec-headsup", uistate.T("reports.headsUp"), nil, Div(anomalyNodes))))
	}

	// ── View sections (each If(...) collapses when its data is absent). ─────────
	drillLink := func(testID string) ui.Node {
		return A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/transactions")),
			Attr("data-testid", testID), uistate.T("reports.viewTransactions"))
	}

	var viewTiles []ui.Node
	switch reportView.Get() {
	case "categories":
		// Spending by category: narrative pull-quote, the bar/donut pair (two views
		// of the same data, side by side), then the ranked drill-through rows.
		catActions := Div(css.Class(tw.Flex, tw.Gap2),
			Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "reports-yoy-toggle"),
				Attr("aria-pressed", boolStr(yoyMode.Get())),
				Title(uistate.T("reports.yoyTitle")), OnClick(onToggleYoY),
				uistate.T(yoyLabelKey)),
			Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "reports-rollup-toggle"),
				Attr("aria-pressed", boolStr(rollupCats.Get())),
				Title(uistate.T("reports.rollupTitle")), OnClick(onToggleRollup),
				uistate.T(rollupLabelKey(rollupCats.Get()))),
		)
		viewTiles = append(viewTiles, rptTile("rpt-categories", "1 / span 4",
			rptSection("sec-categories", uistate.T("reports.byCategory"), catActions, Fragment(
				P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), narrative),
				// One quiet stat line (peak weekday + purchase stats joined), not a
				// stack of three competing paragraph styles under the pull-quote.
				func() ui.Node {
					parts := []string{}
					if weekdayPeakLine != "" {
						parts = append(parts, weekdayPeakLine)
					}
					if spendStats.Count > 0 {
						parts = append(parts, uistate.T("reports.spendStats", spendStats.Count, fmtMinor(spendStats.Average), fmtMinor(spendStats.Median)))
					}
					if len(parts) == 0 {
						return Fragment()
					}
					return P(css.Class("muted"), strings.Join(parts, " · "))
				}(),
				If(len(catBarNodes) > 0 || len(catDonutNodes) > 0, Div(css.Class("rpt-chart-pair"),
					If(len(catBarNodes) > 0, Div(catBarNodes)),
					If(len(catDonutNodes) > 0, Div(catDonutNodes)),
				)),
				catBody,
			))))
	case "networth":
		if len(netSeries) < 2 && len(accounts) == 0 && len(srSeries) < 2 {
			viewTiles = append(viewTiles, rptTile("rpt-empty", "1 / span 4", reportsTabEmpty(uistate.T("reports.emptyNetWorth"))))
			break
		}
		// §5.7b: the canonical net-worth panel renders its own card chrome, so it
		// mounts as a bare full-width bento child (not nested inside a Widget tile).
		viewTiles = append(viewTiles, Div(Style(map[string]string{"grid-column": "1 / span 4"}),
			ui.CreateElement(NetWorthPanel, NetWorthPanelProps{})))
		// Cash-flow + savings-rate trends pair side-by-side on the 4-column grid.
		if len(netSeries) >= 2 {
			viewTiles = append(viewTiles, rptTile("rpt-cashtrend", "span 2",
				rptSection("sec-cashtrend", uistate.T("dashboard.cashFlow"), nil, Fragment(
					If(cashFlowTakeaway != "", P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "cashflow-takeaway"), cashFlowTakeaway)),
					P(css.Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
					uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, Stroke: accent, GradientID: "cf-reports", Label: uistate.T("dashboard.cashFlow"), Labels: trendLabels, ValueLabels: moneyLabels(netSeries)}),
				))))
		}
		if len(srSeries) >= 2 {
			viewTiles = append(viewTiles, rptTile("rpt-savingstrend", "span 2",
				rptSection("sec-savingstrend", uistate.T("reports.savingsTrend"), nil, Fragment(
					If(savingsTakeaway != "", P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "savings-takeaway"), savingsTakeaway)),
					P(css.Class("muted"), uistate.T("reports.savingsTrendHint", trendBuckets)),
					uiw.AreaChart(uiw.AreaChartProps{Values: srSeries, Stroke: accent, GradientID: "sr-reports", Label: uistate.T("reports.savingsTrend"), Labels: trendLabels, ValueLabels: pctLabels(srSeries)}),
				))))
		}
	case "advanced":
		if len(cfDefs) == 0 {
			viewTiles = append(viewTiles, rptTile("rpt-empty", "1 / span 4", reportsTabEmpty(uistate.T("reports.emptyAdvanced"))))
			break
		}
		// Custom-field spend + deductible totals; the deductible tile renders only
		// when at least one category is flagged deductible.
		deductible := deductibleSection(scopedTxns, cats, cs, ce, rates, base, fmtMinor, w)
		cfCol := "1 / span 4"
		if deductible != nil {
			cfCol = "span 2"
		}
		viewTiles = append(viewTiles, rptTile("rpt-customfield", cfCol,
			customFieldSpendSection(scopedTxns, cfDefs, selectedCFKey.Get(), onCFKeyChange, cs, ce, rates, base, fmtMinor, w)))
		if deductible != nil {
			viewTiles = append(viewTiles, rptTile("rpt-deductible", "span 2", deductible))
		}
	default: // "overview"
		if len(moneyFlows) <= 1 && len(payeeNodes) == 0 && len(largestNodes) == 0 &&
			len(bigIncomeNodes) == 0 && len(incomeNodes) == 0 && len(memberNodes) == 0 {
			viewTiles = append(viewTiles, rptTile("rpt-empty", "1 / span 4", reportsTabEmpty(uistate.T("reports.emptyOverview"))))
			break
		}
		if len(moneyFlows) > 1 {
			viewTiles = append(viewTiles, rptTile("rpt-flow", "1 / span 4",
				rptSection("sec-flow", uistate.T("reports.moneyFlow"), drillLink("moneyflow-drill"),
					uiw.Mermaid(uiw.MermaidProps{Source: mermaid.Sankey(moneyFlows), Label: "Income to spending categories money-flow", ValuePrefix: currency.Symbol(base)}))))
		}
		if len(payeeNodes) > 0 {
			viewTiles = append(viewTiles, rptTile("rpt-payees", "span 2",
				rptSection("sec-payees", uistate.T("reports.topPayees"), drillLink("payees-drill"), Fragment(
					If(len(payeeBarNodes) > 0, Div(payeeBarNodes)),
					Div(css.Class("rows"), payeeNodes),
				))))
		}
		if len(largestNodes) > 0 {
			viewTiles = append(viewTiles, rptTile("rpt-expenses", "span 2",
				rptSection("sec-expenses", uistate.T("reports.biggestExpenses"), drillLink("expenses-drill"), Fragment(
					If(len(expenseBarNodes) > 0, Div(expenseBarNodes)),
					Div(css.Class("rows"), largestNodes),
				))))
		}
		if len(bigIncomeNodes) > 0 {
			viewTiles = append(viewTiles, rptTile("rpt-deposits", "span 2",
				rptSection("sec-deposits", uistate.T("reports.biggestDeposits"), nil,
					Div(css.Class("rows"), bigIncomeNodes))))
		}
		if len(incomeNodes) > 0 {
			viewTiles = append(viewTiles, rptTile("rpt-income", "span 2",
				rptSection("sec-income", uistate.T("reports.incomeBySource"), drillLink("income-drill"), Fragment(
					If(incomeTakeaway != "", P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "income-takeaway"), incomeTakeaway)),
					If(len(incomeBarNodes) > 0, Div(incomeBarNodes)),
					If(len(incomeDonutNodes) > 0, Div(incomeDonutNodes)),
					Div(css.Class("rows"), incomeNodes),
				))))
		}
		if len(app.Members()) >= 2 && len(memberSpend) >= 1 {
			viewTiles = append(viewTiles, rptTile("rpt-member", "1 / span 4",
				rptSection("sec-member", uistate.T("reports.byMember"), nil,
					Div(css.Class("rows"), memberNodes))))
		}
	}

	// ── Opt-in report-metrics FormulaBuilder tile: every figure on this page is a
	// live report_* engine variable, buildable into formulas and widgets. ────────
	tiles := []ui.Node{heroTile, toolbar, headsUpTile}
	tiles = append(tiles, viewTiles...)
	if showFormulas.Get() {
		tiles = append(tiles, rptTile("rpt-formula", "1 / span 4", Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("reports.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("reports.metricsShow"), ShowSaved: true}),
		)))
	}

	return Div(css.Class("bento bento-reports"), tiles)
}

// reportsTabEmpty renders a calm, centered note when a selected tab has no
// data, so the area below the toolbar never goes silently blank (e.g. Advanced
// with no custom fields, Net worth with no accounts).
func reportsTabEmpty(msg string) ui.Node {
	return Div(css.Class("muted"), Attr("data-testid", "reports-tab-empty"),
		Style(map[string]string{"text-align": "center", "padding": "2.5rem 1rem", "max-width": "32rem", "margin": "0 auto"}),
		msg)
}

// customFieldSpendSection renders the "Spending by <field>" section body: a
// field selector, a ranked list of value→amount rows, and a CSV download
// button. It is extracted to keep the main Reports function readable and to
// isolate the per-field OnChange hook (called at a single stable render
// position, not in a loop).
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

	// When nothing in the period actually carries a value for this field, the
	// grouper degenerates to a single "(no value) — 100%" bar that reads like a
	// real insight. Show an honest empty state instead (the field selector stays
	// so the user can try another field).
	allUnvalued := len(cfRows) > 0
	for _, r := range cfRows {
		if r.Value != "" {
			allUnvalued = false
			break
		}
	}

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
		bar := Div(css.Class("share-bar", "share-bar-thin"),
			Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
		rowNodes = append(rowNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), label), bar),
			Span(css.Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	var body ui.Node
	switch {
	case allUnvalued:
		body = P(css.Class("empty"), Attr("data-testid", "cf-unvalued"), uistate.T("reports.customFieldUnvalued", activeDef.Label))
	case len(rowNodes) == 0:
		body = P(css.Class("empty"), uistate.T("reports.empty"))
	default:
		body = Div(css.Class("rows"), rowNodes)
	}

	sectionLabel := uistate.T("reports.byCustomField", activeDef.Label)
	selectorLabel := uistate.T("reports.customFieldSelectLabel")

	return Div(Attr("data-testid", "customfield-spend-section"),
		rptSection("sec-customfield", sectionLabel, nil, Fragment(
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Py1),
				Label(Attr("for", "cf-field-select"), selectorLabel),
				Select(css.Class("field"), Attr("id", "cf-field-select"), Attr("aria-label", selectorLabel), Attr("data-testid", "cf-field-select"), onKeyChange, fieldOpts),
			),
			body,
			If(!allUnvalued && len(rowNodes) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
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
		)))
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
		// Neutral-toned: it's metadata, and red stays reserved for negative money.
		delta = Span(ClassStr("row-meta rpt-new-tag "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1, tw.TextDim)),
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
// shareBar but each rank gets a distinct hue. Color each row by its rank with
// the SAME Tableau10 palette the sibling bar chart and donut use, so a category
// reads as one hue across all three views in the card (e.g. Mortgage is blue
// everywhere) instead of three palettes (G9.1a).
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
	return Div(css.Class("share-bar"),
		Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct),
			"background": tableau10(idx)})))
}

// rollupLabelKey is the i18n key for the by-category roll-up toggle's label,
// reflecting whether sub-categories are currently rolled up (L28).
func rollupLabelKey(on bool) string {
	if on {
		return "reports.rollupOn"
	}
	return "reports.rollupOff"
}

// deductibleSection renders the "Deductible totals" section body (L16/L58): a
// ranked list of deductible-flagged categories with their expense totals for
// the period, a headline total, and a CSV export. Returns nil when no
// categories are marked deductible, so the tile stays invisible until the user
// sets up at least one deductible category.
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
		return nil
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
		bar := Div(css.Class("share-bar", "share-bar-thin"),
			Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
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

	return Div(Attr("data-testid", "deductible-section"),
		rptSection("sec-deductible", uistate.T("reports.deductibleTitle"), nil, Fragment(
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
		)))
}
