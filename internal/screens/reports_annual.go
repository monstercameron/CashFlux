// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/mermaid"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Reports is the Annual Review: one long, dense document that reviews and trends
// a full year of transactions, reading from strengths to problem spots to a
// dollar-quantified plan. Its structure IS its message — the "verdict spine"
// tones each section's left edge from healthy green through watch amber to
// problem red, ending on the accent-toned plan. All figures come from the pure
// internal/reports + internal/healthscore cores, so they match the rest of the
// app and remain available as report_* engine variables.
func Reports() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	scopeAtom := uistate.UseActiveScope()

	// Drill wiring (category / payee-less: plain ledger) — hooks at stable positions.
	nav := router.UseNavigate()
	txFilterAtom := uistate.UseTxFilter()
	drillCategory := func(categoryID string) {
		f := uistate.TxFilter{Category: categoryID}.Normalize()
		txFilterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	pr := uistate.UsePrefs().Get()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()
	accounts := app.Accounts()

	sc := scopeAtom.Get()
	instOf := func(a domain.Account) string { return a.Institution }
	scopeIDs := scope.ResolveScope(accounts, sc, instOf)
	scopedTxns := scope.ApplyScopeToTxns(txns, scopeIDs)

	// ── The review window: 12 whole months ending with the month the top-bar
	// period lands in (so stepping the period walks the year; the newest month may
	// be the in-progress one). Prior year = the same window shifted back 12 months.
	w := uistate.UsePeriod().Get()
	uistate.PersistPeriodWindow(w)
	_, wEnd := w.Range()
	lastMonth := dateutil.MonthStart(wEnd.AddDate(0, 0, -1))
	as := dateutil.AddMonths(lastMonth, -11) // annual start (inclusive)
	ae := dateutil.AddMonths(lastMonth, 1)   // annual end (exclusive)
	ps, pe := dateutil.AddMonths(as, -12), as

	// Reading-posture toggles (persisted): rollup + YoY drive the category review.
	cfg := uistate.ReportsConfigGet()
	rollupCats := ui.UseState(cfg.Rollup)
	onToggleRollup := ui.UseEvent(func() { rollupCats.Set(!rollupCats.Get()) })
	yoyMode := ui.UseState(cfg.YoY)
	_ = yoyMode // annual review always compares to the prior year; state kept for config compat
	scopeOpen := ui.UseState(!sc.IsAll())
	onToggleScope := ui.UseEvent(Prevent(func() { scopeOpen.Set(!scopeOpen.Get()) }))
	showFormulas := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))
	exportOpen := ui.UseState(false)
	onToggleExport := ui.UseEvent(Prevent(func() { exportOpen.Set(!exportOpen.Get()) }))
	onCloseExport := ui.UseEvent(Prevent(func() { exportOpen.Set(false) }))
	uiw.DismissPopover(exportOpen.Get(), "rpt-export", func() { exportOpen.Set(false) })
	uiw.AnchorPopover(exportOpen.Get(), "rpt-export")
	// Custom-field grouper for the appendix.
	txnDefs := app.CustomFieldDefsFor("transaction")
	var cfDefs []customfields.Def
	cfDefs = append(cfDefs, txnDefs...)
	firstCFKey := ""
	if len(cfDefs) > 0 {
		firstCFKey = cfDefs[0].Key
	}
	selectedCFKey := ui.UseState(firstCFKey)
	onCFKeyChange := OnChange(func(v string) { selectedCFKey.Set(v) })

	persistKey := fmt.Sprintf("annual|%t|%t", yoyMode.Get(), rollupCats.Get())
	ui.UseEffect(func() func() {
		uistate.SetReportsConfig(uistate.ReportsConfig{View: "annual", YoY: yoyMode.Get(), Rollup: rollupCats.Get()})
		return nil
	}, persistKey)

	// ── Year computations (all pure-core calls over [as, ae)). ────────────────
	flow, _ := reports.IncomeVsExpense(scopedTxns, as, ae, rates)
	rows, _ := reports.SpendingByCategory(scopedTxns, as, ae, true, ps, pe, rates)
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
	decimals := currency.Decimals(base)

	// Monthly bounds → the per-month flows, savings-rate + category trend series.
	bounds := make([]time.Time, 0, 13)
	for k := 0; k <= 12; k++ {
		bounds = append(bounds, dateutil.AddMonths(as, k))
	}
	monthFlows, _ := reports.IncomeExpenseSeries(scopedTxns, bounds, rates)
	srInts, _ := reports.SavingsRateSeries(scopedTxns, bounds, rates)
	catTrends, _ := reports.CategoryTrends(scopedTxns, bounds, rates)
	trendByCat := make(map[string][]int64, len(catTrends))
	for _, tr := range catTrends {
		trendByCat[tr.CategoryID] = tr.Spend
	}
	monthLabels := make([]string, 0, 12)
	for k := 0; k < 12; k++ {
		monthLabels = append(monthLabels, bounds[k].Format("Jan"))
	}

	// Net worth: full-household (unscoped) monthly series across the window.
	nwBounds := append(append([]time.Time{}, bounds...))
	nwSeries, _ := ledger.NetWorthSeries(accounts, txns, nwBounds, rates)
	nwNet, _, _, _ := ledger.NetWorth(accounts, txns, rates)
	var nwChange int64
	if n := len(nwSeries); n >= 2 {
		nwChange = nwSeries[n-1].Amount - nwSeries[0].Amount
	}

	// Health: the deterministic score + factors + prioritized steps.
	health := healthscore.Evaluate(liveHealthInputs(app, time.Now()))

	// Runway (liquid ÷ 6-month burn) for the strengths/problems split.
	liquid, _ := ledger.LiquidBalance(accounts, scopedTxns, rates)
	burn := reports.AverageMonthlyExpense(lastN(monthFlows, 6))
	runway := reports.EstimateRunway(liquid.Amount, burn)

	// Year lists.
	payees, _ := reports.TopPayees(scopedTxns, as, ae, rates, 10)
	largest, _ := reports.LargestExpenses(scopedTxns, as, ae, rates, 10)
	bigIncome, _ := reports.LargestIncome(scopedTxns, as, ae, rates, 8)
	incomeRows, _ := reports.IncomeByCategory(scopedTxns, as, ae, rates)
	memberSpend, _ := reports.SpendingByMember(scopedTxns, as, ae, rates)
	spendStats, _ := reports.SpendingStats(scopedTxns, as, ae, rates)
	noSpendDays := reports.NoSpendDays(scopedTxns, as, ae, time.Now())
	weekday, _ := reports.SpendingByWeekday(scopedTxns, as, ae, rates)
	subs, _ := subscriptions.Detect(scopedTxns, rates, 3)
	liveSubs := subs[:0:0]
	for _, s := range subs {
		if !s.Lapsed(time.Now()) {
			liveSubs = append(liveSubs, s)
		}
	}
	priceRises, _ := subscriptions.DetectPriceChanges(scopedTxns, rates, 3)
	rises := priceRises[:0:0]
	for _, pc := range priceRises {
		if pc.Delta > 0 {
			rises = append(rises, pc)
		}
	}
	monthsRed := reports.MonthsNegative(monthFlows)
	hiIdx, loIdx, seasonalOK := reports.SeasonalExtremes(monthFlows)
	trims := reports.TrimTargets(catTrends, 2500, 3) // ≥$25/mo recent average
	per100 := reports.Per100(rows, flow.Income, 6)

	// Goals over the year (household-wide; classify is cheap).
	gc := goalsvc.CountByState(app.Goals(), app.Tasks(), time.Now(), true)

	// Uncategorized share of spend (data hygiene).
	var uncatMinor int64
	for _, r := range rows {
		if r.CategoryID == "" {
			uncatMinor = absMinor(r.Amount)
			break
		}
	}
	uncatPct := int64(0)
	if flow.Expense > 0 {
		uncatPct = uncatMinor * 100 / flow.Expense
	}

	// Debt drag: interest-bearing liabilities with estimated annual interest.
	type debtRow struct {
		name         string
		balance      money.Money
		apr          float64
		estYearMinor int64
		minimum      money.Money
	}
	var debts []debtRow
	var debtInterestTotal int64
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability || a.InterestRateAPR <= 0 {
			continue
		}
		bal, _ := ledger.Balance(a, txns)
		ab := absMinor(bal.Amount)
		if ab == 0 {
			continue
		}
		est := int64(float64(ab) * a.InterestRateAPR / 100)
		debts = append(debts, debtRow{name: a.Name, balance: bal, apr: a.InterestRateAPR, estYearMinor: est, minimum: a.MinPayment})
		debtInterestTotal += est
	}
	sort.SliceStable(debts, func(i, j int) bool { return debts[i].apr > debts[j].apr })

	// CountUp on the masthead figures.
	heroSig := fmt.Sprintf("%d|%d|%d", flow.Net(), flow.Income, flow.Expense)
	ui.UseEffect(func() func() {
		if fn := js.Global().Get("cashfluxCountUpScan"); fn.Type() == js.TypeFunction {
			fn.Invoke()
		}
		return nil
	}, heroSig)

	// Empty year (and no scope filter to blame): a single CTA, not a page of zeros.
	if flow.Income == 0 && flow.Expense == 0 && sc.IsAll() {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("reports.empty"), CTALabel: uistate.T("reports.addFirst"), Href: "/transactions"})
	}

	windowLine := uistate.T("rpta.window", bounds[0].Format("Jan 2006"), lastMonth.Format("Jan 2006"))

	// ── Masthead: the verdict + four anchor figures. ──────────────────────────
	verdict, verdictTone := rptaVerdict(health)
	kept := money.New(flow.Net(), base)
	mastFigs := []ui.Node{
		rptaFig(uistate.T("dashboard.income"), fmtMoney(money.New(flow.Income, base)), "", ""),
		rptaFig(uistate.T("dashboard.spending"), fmtMoney(money.New(flow.Expense, base)), "", ""),
		rptaFig(uistate.T("rpta.kept"), fmtMoney(kept), rptaToneFor(kept.Amount), uistate.T("rpta.keptRate", flow.SavingsRate())),
	}
	if len(accounts) > 0 {
		sub := ""
		tone := ""
		if nwChange != 0 {
			arrow := "▲"
			tone = "up"
			if nwChange < 0 {
				arrow, tone = "▼", "down"
			}
			sub = arrow + " " + fmtMinor(absMinor(nwChange)) + " " + uistate.T("rpta.overYear")
		}
		mastFigs = append(mastFigs, Div(css.Class("rpta-fig"), Attr("data-testid", "reports-hero-networth"),
			Span(css.Class("rpta-fig-k"), uistate.T("dashboard.netWorth")),
			Span(css.Class("rpta-fig-v", tw.FontDisplay), fmtMoney(nwNet)),
			If(sub != "", Span(ClassStr("rpta-fig-sub rpta-tone-"+tone), sub)),
		))
	}
	masthead := Div(css.Class("rpta-masthead"), Attr("data-testid", "rpt-hero"), Attr("id", "rpta-top"),
		P(css.Class("rpta-eyebrow"), uistate.T("rpta.eyebrow")),
		H1(css.Class("rpta-title", tw.FontDisplay), windowLine),
		Div(ClassStr("rpta-verdict rpta-tone-"+verdictTone), Attr("data-testid", "rpta-verdict"),
			Span(css.Class("rpta-verdict-score", tw.FontDisplay), rptaScoreText(health)),
			Span(css.Class("rpta-verdict-line"), verdict),
		),
		Div(css.Class("rpta-figs"), Attr("data-countup", ""), mastFigs),
	)

	// ── Toolbar (tabless): scope, metrics, export. ───────────────────────────
	toolbar := rptaToolbar(app, sc, scopeOpen.Get(), onToggleScope, showFormulas.Get(), toggleFormulas,
		exportOpen.Get(), onToggleExport, onCloseExport, scopedTxns, rows, incomeRows, payees, largest,
		memberSpend, nameOf, base, w.Res, as, rates)

	// ── The sticky section index (jump links, zone-dotted). ──────────────────
	index := rptaIndex()

	// ── 01 · What's strong. ───────────────────────────────────────────────────
	var strongFacts, weakFacts []ui.Node
	for _, f := range health.Factors {
		if f.Weight <= 0 {
			continue
		}
		node := rptaFactorRow(f)
		if f.Score >= 70 {
			strongFacts = append(strongFacts, node)
		} else {
			weakFacts = append(weakFacts, node)
		}
	}
	var wins []ui.Node
	if noSpendDays > 0 {
		wins = append(wins, rptaWin(uistate.T("rpta.winNoSpend", noSpendDays)))
	}
	if bestIdx := bestSavingsMonth(monthFlows); bestIdx >= 0 {
		wins = append(wins, rptaWin(uistate.T("rpta.winBestMonth", bounds[bestIdx].Format("January"), fmtMinor(monthFlows[bestIdx].Net()))))
	}
	for i, r := range topCuts(rows, 3) {
		_ = i
		wins = append(wins, rptaWin(uistate.T("rpta.winCut", nameOf(r.CategoryID), fmtMinor(r.Prior-r.Amount))))
	}
	if gc.Completed > 0 {
		wins = append(wins, rptaWin(uistate.T("rpta.winGoals", gc.Completed)))
	}
	if runway.Months >= 3 && burn > 0 {
		wins = append(wins, rptaWin(uistate.T("rpta.winRunway", runway.Months)))
	}
	strengths := rptaSection("rpta-01", "01", uistate.T("rpta.secStrong"), "up", uistate.T("rpta.secStrongSub"), Fragment(
		If(len(strongFacts) == 0, P(css.Class("rpta-muted"), uistate.T("rpta.noStrong"))),
		Div(css.Class("rpta-facts"), strongFacts),
		If(len(wins) > 0, Div(css.Class("rpta-wins"), Attr("data-testid", "rpta-wins"), wins)),
	))

	// ── 02 · The flow of money (enhanced Sankey + per-$100). ─────────────────
	sankeyFactor := int64(1)
	for i := 0; i < decimals; i++ {
		sankeyFactor *= 10
	}
	toMajor := func(minor int64) int64 { return (minor + sankeyFactor/2) / sankeyFactor }
	var moneyFlows []mermaid.SankeyFlow
	// Sources → Income (the enhancement: where the money comes FROM).
	incomeLabel := uistate.T("rpta.nodeIncome")
	srcCount := 0
	var srcRest int64
	for _, r := range incomeRows {
		if r.Amount <= 0 {
			continue
		}
		if srcCount < 5 {
			moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: nameOf(r.CategoryID), To: incomeLabel, Value: toMajor(r.Amount)})
			srcCount++
		} else {
			srcRest += r.Amount
		}
	}
	if srcRest > 0 {
		moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: uistate.T("rpta.nodeOtherIncome"), To: incomeLabel, Value: toMajor(srcRest)})
	}
	// Income → categories (top 10 + rest) + Savings.
	catCount := 0
	var catRest int64
	for _, r := range rows {
		v := absMinor(r.Amount)
		if v == 0 {
			continue
		}
		if catCount < 10 {
			moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: incomeLabel, To: nameOf(r.CategoryID), Value: toMajor(v)})
			catCount++
		} else {
			catRest += v
		}
	}
	if catRest > 0 {
		moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: incomeLabel, To: uistate.T("rpta.nodeEverythingElse"), Value: toMajor(catRest)})
	}
	if sav := flow.Net(); sav > 0 {
		moneyFlows = append(moneyFlows, mermaid.SankeyFlow{From: incomeLabel, To: uistate.T("rpta.nodeSavings"), Value: toMajor(sav)})
	}
	var per100Rows []ui.Node
	for _, p := range per100 {
		label := nameOf(p.CategoryID)
		if p.CategoryID == "" {
			label = uistate.T("rpta.nodeEverythingElse")
		}
		per100Rows = append(per100Rows, Tr(
			Td(css.Class("rpta-td-name"), label),
			Td(css.Class("rpta-td-num"), fmt.Sprintf("$%d.%d0", p.Per100, p.Tenths)),
			Td(css.Class("rpta-td-num", "rpta-muted"), fmtMinor(p.AmountMinor)),
		))
	}
	if kv := flow.Net(); kv > 0 && flow.Income > 0 {
		scaled := kv * 1000 / flow.Income
		per100Rows = append(per100Rows, Tr(css.Class("rpta-tr-kept"),
			Td(css.Class("rpta-td-name"), uistate.T("rpta.nodeSavings")),
			Td(css.Class("rpta-td-num"), fmt.Sprintf("$%d.%d0", scaled/10, scaled%10)),
			Td(css.Class("rpta-td-num", "rpta-muted"), fmtMinor(kv)),
		))
	}
	flowSec := rptaSection("rpta-02", "02", uistate.T("reports.moneyFlow"), "up", uistate.T("rpta.secFlowSub"), Fragment(
		If(len(moneyFlows) > 1, Div(css.Class("rpta-sankey"),
			uiw.Mermaid(uiw.MermaidProps{Source: mermaid.Sankey(moneyFlows), Label: "Income sources through income to spending categories and savings", ValuePrefix: currency.Symbol(base)}))),
		Div(css.Class("rpta-flow-side"),
			Div(css.Class("rpta-subhead"), uistate.T("rpta.per100Head")),
			Table(css.Class("rpta-table", "rpta-per100"), Attr("data-testid", "rpta-per100"),
				Thead(Tr(Th(uistate.T("rpta.per100Where")), Th("/$100"), Th(uistate.T("rpta.per100Year")))),
				Tbody(per100Rows),
			),
			A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "moneyflow-drill"), uistate.T("reports.viewTransactions")),
		),
	))

	// ── 03 · The year in motion (monthly review table + trends). ─────────────
	accent := chartLineColor(uistate.CurrentAccent())
	var monthRows []ui.Node
	for i := 0; i < 12 && i < len(monthFlows); i++ {
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
		monthRows = append(monthRows, Tr(ClassStr(rowCls),
			Td(css.Class("rpta-td-name"), bounds[i].Format("January 2006")),
			Td(css.Class("rpta-td-num"), fmtMinor(f.Income)),
			Td(css.Class("rpta-td-num"), fmtMinor(f.Expense)),
			Td(ClassStr("rpta-td-num rpta-td-strong"+If2(f.Net() < 0, " rpta-tone-down", "")), fmtMinor(f.Net())),
			Td(css.Class("rpta-td-num"), rate),
		))
	}
	netSeries := make([]float64, 0, 12)
	for i := 0; i < 12 && i < len(monthFlows); i++ {
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
	motion := rptaSection("rpta-03", "03", uistate.T("rpta.secMotion"), "neutral", uistate.T("rpta.secMotionSub"), Fragment(
		Table(css.Class("rpta-table", "rpta-months"), Attr("data-testid", "rpta-months"),
			Thead(Tr(Th(uistate.T("rpta.colMonth")), Th(uistate.T("dashboard.income")), Th(uistate.T("dashboard.spending")), Th(uistate.T("reports.net")), Th(uistate.T("rpta.colKeptPct")))),
			Tbody(monthRows),
		),
		If(seasonLine != "", P(css.Class("rpta-muted"), Attr("data-testid", "rpta-seasonal"), seasonLine)),
		If(statsLine != "", P(css.Class("rpta-muted"), statsLine)),
		Div(css.Class("rpta-charts3"),
			If(len(netSeries) >= 2, Div(css.Class("rpta-chart"),
				Div(css.Class("rpta-subhead"), uistate.T("dashboard.cashFlow")),
				uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, Stroke: accent, GradientID: "rpta-net", Label: uistate.T("dashboard.cashFlow"), Labels: monthLabels, ValueLabels: moneyVL(netSeries)}))),
			If(len(srSeries) >= 2, Div(css.Class("rpta-chart"),
				Div(css.Class("rpta-subhead"), uistate.T("reports.savingsTrend")),
				uiw.AreaChart(uiw.AreaChartProps{Values: srSeries, Stroke: accent, GradientID: "rpta-sr", Label: uistate.T("reports.savingsTrend"), Labels: monthLabels, ValueLabels: pctVL(srSeries)}))),
			If(len(nwFloat) >= 2, Div(css.Class("rpta-chart"),
				Div(css.Class("rpta-subhead"), uistate.T("dashboard.netWorth")),
				uiw.AreaChart(uiw.AreaChartProps{Values: nwFloat, Stroke: accent, GradientID: "rpta-nw", Label: uistate.T("dashboard.netWorth"), Labels: monthLabels, ValueLabels: moneyVL(nwFloat)}))),
		),
	))

	// ── 04 · Categories reviewed (the full-year table with sparklines). ──────
	narrative := reports.SpendingNarrative(rows, true, fmtMinor, func(id string) string { return catName[id] })
	var maxCat int64
	for _, r := range rows {
		if a := absMinor(r.Amount); a > maxCat {
			maxCat = a
		}
	}
	var catRows, zeroCatRows []ui.Node
	for _, r := range rows {
		if r.Amount == 0 && r.Prior == 0 {
			continue
		}
		node := ui.CreateElement(rptaCatRow, rptaCatRowProps{
			CategoryID: r.CategoryID, Name: nameOf(r.CategoryID),
			Amount: r.Amount, Prior: r.Prior, HasDelta: r.HasDelta, DeltaPct: r.DeltaPct, PriorZero: r.PriorZero,
			TotalSpend: flow.Expense, MaxCat: maxCat, Spark: trendByCat[r.CategoryID],
			FmtMinor: fmtMinor, OnDrill: drillCategory,
		})
		if r.Amount == 0 {
			zeroCatRows = append(zeroCatRows, node)
		} else {
			catRows = append(catRows, node)
		}
	}
	catActions := Div(css.Class(tw.Flex, tw.Gap2),
		Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "reports-rollup-toggle"),
			Attr("aria-pressed", boolStr(rollupCats.Get())), Title(uistate.T("reports.rollupTitle")),
			OnClick(onToggleRollup), uistate.T(rollupLabelKey(rollupCats.Get()))),
	)
	categories := rptaSectionWithAction("rpta-04", "04", uistate.T("rpta.secCats"), "neutral", uistate.T("rpta.secCatsSub"), catActions, Fragment(
		P(css.Class("rpta-narrative", tw.FontDisplay), narrative),
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
	))

	// ── 05 · Where it actually goes (payees, biggest, deposits, sources, members).
	shareBar := func(amount, max int64) ui.Node {
		if max <= 0 {
			return Fragment()
		}
		pct := absMinor(amount) * 100 / max
		if pct > 100 {
			pct = 100
		}
		return Div(css.Class("share-bar"), Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
	}
	listRows := func(items []ui.Node) ui.Node { return Div(css.Class("rows"), items) }
	var payeeNodes []ui.Node
	var maxPayee int64
	for _, p := range payees {
		if a := absMinor(p.Amount); a > maxPayee {
			maxPayee = a
		}
	}
	for _, p := range payees {
		nm := p.Name
		if nm == "" {
			nm = uistate.T("reports.noPayee")
		}
		payeeNodes = append(payeeNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), nm), shareBar(p.Amount, maxPayee)),
			Span(css.Class("budget-amount"), fmtMinor(p.Amount))))
	}
	var largestNodes []ui.Node
	var maxExp int64
	for _, e := range largest {
		if a := absMinor(e.Amount); a > maxExp {
			maxExp = a
		}
	}
	for _, e := range largest {
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		largestNodes = append(largestNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), desc), Span(css.Class("row-meta"), pr.FormatDate(e.Date)), shareBar(e.Amount, maxExp)),
			Span(css.Class("budget-amount"), fmtMinor(e.Amount))))
	}
	var depositNodes []ui.Node
	var maxDep int64
	for _, e := range bigIncome {
		if a := absMinor(e.Amount); a > maxDep {
			maxDep = a
		}
	}
	for _, e := range bigIncome {
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		depositNodes = append(depositNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), desc), Span(css.Class("row-meta"), pr.FormatDate(e.Date)), shareBar(e.Amount, maxDep)),
			Span(css.Class("budget-amount"), fmtMinor(e.Amount))))
	}
	var srcNodes []ui.Node
	for _, r := range incomeRows {
		if r.Amount == 0 {
			continue
		}
		srcNodes = append(srcNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), nameOf(r.CategoryID))),
			Span(css.Class("budget-amount"), fmtMinor(r.Amount))))
	}
	memberName := make(map[string]string, len(app.Members()))
	for _, m := range app.Members() {
		memberName[m.ID] = m.Name
	}
	var memberNodes []ui.Node
	var maxMember int64
	for _, ms := range memberSpend {
		if a := absMinor(ms.Amount); a > maxMember {
			maxMember = a
		}
	}
	for _, ms := range memberSpend {
		nm := memberName[ms.MemberID]
		if nm == "" {
			nm = uistate.T("reports.noMember")
		}
		memberNodes = append(memberNodes, Div(css.Class("row"),
			Div(css.Class("row-main"), Span(css.Class("row-desc"), nm), shareBar(ms.Amount, maxMember)),
			Span(css.Class("budget-amount"), fmtMinor(ms.Amount))))
	}
	whereGoes := rptaSection("rpta-05", "05", uistate.T("rpta.secWhere"), "neutral", uistate.T("rpta.secWhereSub"), Div(css.Class("rpta-cols2"),
		Div(css.Class("rpta-col"),
			rptaSub(uistate.T("reports.topPayees"), A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "payees-drill"), uistate.T("reports.viewTransactions"))),
			listRows(payeeNodes),
			rptaSub(uistate.T("reports.biggestDeposits"), nil),
			listRows(depositNodes),
		),
		Div(css.Class("rpta-col"),
			rptaSub(uistate.T("reports.biggestExpenses"), A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "expenses-drill"), uistate.T("reports.viewTransactions"))),
			listRows(largestNodes),
			rptaSub(uistate.T("reports.incomeBySource"), A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "income-drill"), uistate.T("reports.viewTransactions"))),
			listRows(srcNodes),
			If(len(app.Members()) >= 2 && len(memberNodes) > 0, Fragment(
				rptaSub(uistate.T("reports.byMember"), nil),
				listRows(memberNodes))),
		),
	))

	// ── 06 · Watch list (rising, subscriptions, price creep). ────────────────
	var risingNodes []ui.Node
	for _, tr := range catTrends {
		if !tr.HasDelta || tr.DeltaPct < 25 || tr.Total < 10000 {
			continue
		}
		risingNodes = append(risingNodes, Div(css.Class("rpta-watch-row"),
			Span(css.Class("rpta-watch-name"), nameOf(tr.CategoryID)),
			sparklineSVG(tr.Spend),
			Span(css.Class("rpta-watch-delta", "rpta-tone-warn"), fmt.Sprintf("▲ %d%%", tr.DeltaPct)),
			Span(css.Class("rpta-watch-amt"), fmtMinor(tr.Total)),
		))
		if len(risingNodes) >= 6 {
			break
		}
	}
	var subsAnnual int64
	for _, s := range liveSubs {
		subsAnnual += annualizeSub(s)
	}
	var subNodes []ui.Node
	sort.SliceStable(liveSubs, func(i, j int) bool { return annualizeSub(liveSubs[i]) > annualizeSub(liveSubs[j]) })
	for i, s := range liveSubs {
		if i >= 8 {
			break
		}
		subNodes = append(subNodes, Div(css.Class("rpta-watch-row"),
			Span(css.Class("rpta-watch-name"), s.Name),
			Span(css.Class("rpta-muted"), string(s.Cadence)),
			Span(css.Class("rpta-watch-amt"), uistate.T("rpta.perYear", fmtMinor(annualizeSub(s)))),
		))
	}
	var riseNodes []ui.Node
	for i, pc := range rises {
		if i >= 5 {
			break
		}
		riseNodes = append(riseNodes, Div(css.Class("rpta-watch-row"),
			Span(css.Class("rpta-watch-name"), pc.Name),
			Span(css.Class("rpta-watch-delta", "rpta-tone-warn"), fmt.Sprintf("▲ %d%%", pc.PercentChange)),
			Span(css.Class("rpta-watch-amt"), fmtMinor(pc.OldAmount)+" → "+fmtMinor(pc.NewAmount)),
		))
	}
	watch := rptaSection("rpta-06", "06", uistate.T("rpta.secWatch"), "warn", uistate.T("rpta.secWatchSub"), Fragment(
		If(len(risingNodes) == 0 && len(subNodes) == 0 && len(riseNodes) == 0, P(css.Class("rpta-muted"), uistate.T("rpta.watchClear"))),
		If(len(risingNodes) > 0, Fragment(rptaSub(uistate.T("rpta.watchRising"), nil), Div(Attr("data-testid", "rpta-rising"), risingNodes))),
		If(len(subNodes) > 0, Fragment(
			rptaSub(uistate.T("rpta.watchSubs", len(liveSubs), fmtMinor(subsAnnual)), nil),
			Div(Attr("data-testid", "rpta-subs"), subNodes))),
		If(len(riseNodes) > 0, Fragment(rptaSub(uistate.T("rpta.watchRises"), nil), Div(riseNodes))),
	))

	// ── 07 · Problem spots. ───────────────────────────────────────────────────
	var debtRowNodes []ui.Node
	for _, d := range debts {
		minStr := "—"
		if d.minimum.Amount > 0 {
			minStr = fmtMoney(d.minimum)
		}
		debtRowNodes = append(debtRowNodes, Tr(
			Td(css.Class("rpta-td-name"), d.name),
			Td(css.Class("rpta-td-num"), fmtMoney(d.balance.Abs())),
			Td(css.Class("rpta-td-num"), fmt.Sprintf("%.1f%%", d.apr)),
			Td(css.Class("rpta-td-num", "rpta-tone-down"), fmtMinor(d.estYearMinor)),
			Td(css.Class("rpta-td-num"), minStr),
		))
	}
	var problemBits []ui.Node
	if len(weakFacts) > 0 {
		problemBits = append(problemBits, rptaSub(uistate.T("rpta.probFactors"), nil), Div(css.Class("rpta-facts"), weakFacts))
	}
	if monthsRed > 0 {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line"), Attr("data-testid", "rpta-monthsred"),
			Span(css.Class("rpta-tone-down"), fmt.Sprintf("%d", monthsRed)+" "), uistate.T("rpta.monthsRed")))
	}
	if health.NegativeCashFlow {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line", "rpta-tone-down"), uistate.T("rpta.negCashFlow")))
	}
	if len(debtRowNodes) > 0 {
		problemBits = append(problemBits,
			rptaSub(uistate.T("rpta.probDebt", fmtMinor(debtInterestTotal)), nil),
			Table(css.Class("rpta-table"), Attr("data-testid", "rpta-debt"),
				Thead(Tr(Th(uistate.T("rpta.colDebt")), Th(uistate.T("rpta.colBalance")), Th("APR"), Th(uistate.T("rpta.colYearInterest")), Th(uistate.T("rpta.colMinimum")))),
				Tbody(debtRowNodes)))
	}
	if uncatPct >= 5 {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line"), Attr("data-testid", "rpta-uncat"),
			uistate.T("rpta.uncategorized", uncatPct, fmtMinor(uncatMinor))))
	}
	if gc.Missed > 0 {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line"), uistate.T("rpta.missedGoals", gc.Missed)))
	}
	if len(problemBits) == 0 {
		problemBits = append(problemBits, P(css.Class("rpta-muted"), uistate.T("rpta.noProblems")))
	}
	problems := rptaSection("rpta-07", "07", uistate.T("rpta.secProblems"), "down", uistate.T("rpta.secProblemsSub"), Fragment(anyify(problemBits)...))

	// ── 08 · The plan (numbered, dollar-quantified). ──────────────────────────
	var planItems []ui.Node
	planN := 0
	addPlan := func(action, detail, href, linkLabel string) {
		planN++
		var link ui.Node = Fragment()
		if href != "" {
			link = A(css.Class("rpta-plan-link"), Href(uistate.RoutePath(href)), linkLabel)
		}
		planItems = append(planItems, Div(css.Class("rpta-plan-item"),
			Span(css.Class("rpta-plan-n", tw.FontDisplay), fmt.Sprintf("%02d", planN)),
			Div(css.Class("rpta-plan-body"),
				Span(css.Class("rpta-plan-action"), action),
				If(detail != "", Span(css.Class("rpta-plan-detail"), detail)),
				link,
			)))
	}
	for i, st := range health.Steps {
		if i >= 3 {
			break
		}
		detail := st.Target
		if st.TimeFraming != "" {
			detail += " · " + st.TimeFraming
		}
		addPlan(st.Action, detail, planRouteFor(st.Key), uistate.T("rpta.planOpen"))
	}
	for _, tr := range trims {
		addPlan(
			uistate.T("rpta.planTrim", nameOf(tr.CategoryID), fmtMinor(tr.MedianMinor)),
			uistate.T("rpta.planTrimDetail", fmtMinor(tr.RecentAvgMinor), fmtMinor(tr.MonthlySaveMinor*12)),
			"/budgets", uistate.T("nav.budgets"))
	}
	if len(debts) > 0 {
		d := debts[0]
		addPlan(
			uistate.T("rpta.planDebt", d.name, fmt.Sprintf("%.1f%%", d.apr)),
			uistate.T("rpta.planDebtDetail", fmtMinor(int64(d.apr*1000))),
			"/debt", uistate.T("nav.debt"))
	}
	if len(liveSubs) >= 3 {
		addPlan(
			uistate.T("rpta.planSubs", len(liveSubs), fmtMinor(subsAnnual)),
			If2(len(rises) > 0, uistate.T("rpta.planSubsRises", len(rises)), ""),
			"/subscriptions", uistate.T("nav.subscriptions"))
	}
	plan := rptaSection("rpta-08", "08", uistate.T("rpta.secPlan"), "plan", uistate.T("rpta.secPlanSub"),
		Div(css.Class("rpta-plan"), Attr("data-testid", "rpta-plan"), planItems))

	// ── 09 · Appendix (tax, custom fields, metrics). ──────────────────────────
	winForExports := period.Window{Res: period.Year, From: as}
	deductible := deductibleSection(scopedTxns, cats, as, ae, rates, base, fmtMinor, winForExports)
	var appendixBits []ui.Node
	if deductible != nil {
		appendixBits = append(appendixBits, deductible)
	}
	if len(cfDefs) > 0 {
		appendixBits = append(appendixBits, customFieldSpendSection(scopedTxns, cfDefs, selectedCFKey.Get(), onCFKeyChange, as, ae, rates, base, fmtMinor, winForExports))
	}
	if showFormulas.Get() {
		appendixBits = append(appendixBits, Fragment(
			P(css.Class("rpta-muted"), uistate.T("reports.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("reports.metricsShow"), ShowSaved: true})))
	}
	var appendix ui.Node = Fragment()
	if len(appendixBits) > 0 {
		appendix = rptaSection("rpta-09", "09", uistate.T("rpta.secAppendix"), "dim", uistate.T("rpta.secAppendixSub"), Fragment(anyify(appendixBits)...))
	}

	return Div(css.Class("rpta"),
		masthead,
		toolbar,
		index,
		strengths,
		flowSec,
		motion,
		categories,
		whereGoes,
		watch,
		problems,
		plan,
		appendix,
	)
}

// If2 is a tiny string ternary for class/detail composition.
func If2(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// lastN returns the trailing n elements of flows (fewer when shorter).
func lastN(flows []reports.PeriodFlow, n int) []reports.PeriodFlow {
	if len(flows) <= n {
		return flows
	}
	return flows[len(flows)-n:]
}

// annualizeSub converts a detected subscription's typical charge to a yearly cost.
func annualizeSub(s subscriptions.Subscription) int64 {
	switch s.Cadence {
	case subscriptions.CadenceWeekly:
		return s.Amount * 52
	case subscriptions.CadenceYearly:
		return s.Amount
	default:
		return s.Amount * 12
	}
}

// bestSavingsMonth returns the index of the month with the highest positive net,
// or -1 when no month saved anything.
func bestSavingsMonth(flows []reports.PeriodFlow) int {
	best, idx := int64(0), -1
	for i, f := range flows {
		if n := f.Net(); n > best {
			best, idx = n, i
		}
	}
	return idx
}

// topCuts returns up to n categories with the largest year-over-year spending
// DECREASE (Prior − Amount), the wins the strengths section celebrates.
func topCuts(rows []reports.CategorySpend, n int) []reports.CategorySpend {
	var cuts []reports.CategorySpend
	for _, r := range rows {
		if r.HasDelta && r.Prior > r.Amount && r.Prior-r.Amount > 5000 {
			cuts = append(cuts, r)
		}
	}
	sort.SliceStable(cuts, func(i, j int) bool { return cuts[i].Prior-cuts[i].Amount > cuts[j].Prior-cuts[j].Amount })
	if len(cuts) > n {
		cuts = cuts[:n]
	}
	return cuts
}

// rptaToneFor maps a signed amount to a tone suffix.
func rptaToneFor(v int64) string {
	if v < 0 {
		return "down"
	}
	if v > 0 {
		return "up"
	}
	return ""
}

// rptaVerdict turns the health result into the masthead's one-line verdict + tone.
func rptaVerdict(h healthscore.Result) (string, string) {
	switch h.Band {
	case healthscore.BandExcellent:
		return uistate.T("rpta.verdictExcellent"), "up"
	case healthscore.BandGood:
		return uistate.T("rpta.verdictGood"), "up"
	case healthscore.BandFair:
		return uistate.T("rpta.verdictFair"), "warn"
	case healthscore.BandNeedsWork:
		return uistate.T("rpta.verdictNeedsWork"), "warn"
	case healthscore.BandCritical:
		return uistate.T("rpta.verdictCritical"), "down"
	default:
		return uistate.T("rpta.verdictNoData"), ""
	}
}

// rptaScoreText renders "82 · Good" (or just the band when no score applies).
func rptaScoreText(h healthscore.Result) string {
	if h.Band == healthscore.BandNoData {
		return string(h.Band)
	}
	return fmt.Sprintf("%d · %s", h.Score, h.Band)
}

// planRouteFor maps a health-step key to the screen where the user acts on it.
func planRouteFor(key string) string {
	switch key {
	case "debt", "utilization":
		return "/debt"
	case "budget":
		return "/budgets"
	case "savings":
		return "/goals"
	case "emergency":
		return "/goals"
	case "nw-trend":
		return "/networth"
	default:
		return "/allocate"
	}
}

// rptaFig is one masthead anchor figure: small caps label over a serif value.
func rptaFig(label, value, tone, sub string) ui.Node {
	vCls := "rpta-fig-v " + tw.Fold(tw.FontDisplay)
	if tone != "" {
		vCls += " rpta-tone-" + tone
	}
	return Div(css.Class("rpta-fig"),
		Span(css.Class("rpta-fig-k"), label),
		Span(ClassStr(vCls), value),
		If(sub != "", Span(css.Class("rpta-fig-sub", "rpta-muted"), sub)),
	)
}

// rptaWin is one strengths-strip win chip.
func rptaWin(text string) ui.Node {
	return Span(css.Class("rpta-win"), text)
}

// rptaSub is a small in-section subheading with an optional right-aligned action.
func rptaSub(title string, action ui.Node) ui.Node {
	if action == nil {
		action = Fragment()
	}
	return Div(css.Class("rpta-subrow"),
		Span(css.Class("rpta-subhead"), title),
		action,
	)
}

// rptaFactorRow renders one health factor: label, live value, a 0-100 score bar.
func rptaFactorRow(f healthscore.Factor) ui.Node {
	tone := "up"
	if f.Score < 40 {
		tone = "down"
	} else if f.Score < 70 {
		tone = "warn"
	}
	return Div(css.Class("rpta-fact"), Attr("data-testid", "rpta-fact-"+f.Key),
		Span(css.Class("rpta-fact-name"), f.Label),
		Span(css.Class("rpta-fact-val", tw.FontDisplay), f.Value),
		Div(css.Class("rpta-fact-bar"),
			Div(ClassStr("rpta-fact-fill rpta-fill-"+tone), Style(map[string]string{"width": fmt.Sprintf("%d%%", f.Score)}))),
		Span(ClassStr("rpta-fact-score rpta-tone-"+tone), fmt.Sprintf("%d", f.Score)),
	)
}

// rptaSection wraps one numbered zone-toned document section.
func rptaSection(id, num, title, zone, sub string, body ui.Node) ui.Node {
	return rptaSectionWithAction(id, num, title, zone, sub, nil, body)
}

func rptaSectionWithAction(id, num, title, zone, sub string, action, body ui.Node) ui.Node {
	if action == nil {
		action = Fragment()
	}
	return Section(ClassStr("rpta-sec rpta-z-"+zone), Attr("id", id), Attr("data-testid", id),
		Div(css.Class("rpta-sec-head"),
			Div(css.Class("rpta-sec-title-wrap"),
				Span(css.Class("rpta-sec-num", tw.FontDisplay), num),
				Div(
					H2(css.Class("rpta-sec-title", tw.FontDisplay), title),
					If(sub != "", P(css.Class("rpta-sec-sub"), sub)),
				),
			),
			action,
		),
		Div(css.Class("rpta-sec-body"), body),
	)
}

// rptaIndex is the sticky jump index: 01-09 with zone dots.
func rptaIndex() ui.Node {
	item := func(href, num, key, zone string) ui.Node {
		return A(css.Class("rpta-idx-item"), Href(href),
			Span(ClassStr("rpta-idx-dot rpta-dot-"+zone)),
			Span(css.Class("rpta-idx-num"), num),
			Span(css.Class("rpta-idx-label"), uistate.T(key)),
		)
	}
	return Nav(css.Class("rpta-index"), Attr("data-testid", "rpta-index"), Attr("aria-label", uistate.T("rpta.indexLabel")),
		item("#rpta-01", "01", "rpta.idxStrong", "up"),
		item("#rpta-02", "02", "rpta.idxFlow", "up"),
		item("#rpta-03", "03", "rpta.idxMotion", "neutral"),
		item("#rpta-04", "04", "rpta.idxCats", "neutral"),
		item("#rpta-05", "05", "rpta.idxWhere", "neutral"),
		item("#rpta-06", "06", "rpta.idxWatch", "warn"),
		item("#rpta-07", "07", "rpta.idxProblems", "down"),
		item("#rpta-08", "08", "rpta.idxPlan", "plan"),
	)
}

// rptaToolbar renders the tabless control strip: scope, metrics, export.
func rptaToolbar(app *appstate.App, sc scope.ReportScope, scopeOpenV bool, onToggleScope ui.Handler,
	formulasOn bool, toggleFormulas ui.Handler, exportOpenV bool, onToggleExport, onCloseExport ui.Handler,
	scopedTxns []domain.Transaction, rows []reports.CategorySpend, incomeRows []reports.CategorySpend,
	payees []reports.PayeeTotal, largest []reports.ExpenseItem, memberSpend []reports.MemberSpend,
	nameOf func(string) string, base string, res period.Resolution, from time.Time, rates currency.Rates) ui.Node {

	scopeCount := len(sc.Institutions) + len(sc.Owners) + len(sc.Types) + len(sc.AccountIDs)
	scopeLabel := uistate.T("reports.scope")
	if scopeCount > 0 {
		scopeLabel = uistate.T("reports.scopeCount", scopeCount)
	}
	scopeCls := "strip-toggle"
	if scopeOpenV || scopeCount > 0 {
		scopeCls += " is-on"
	}
	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("reports.metricsShow")
	if formulasOn {
		metricsCls += " is-on"
		metricsLabel = uistate.T("reports.metricsHide")
	}
	csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
	memberNmMap := map[string]string{}
	for _, m := range app.Members() {
		memberNmMap[m.ID] = m.Name
	}
	memberNm := func(id string) string {
		if n := memberNmMap[id]; n != "" {
			return n
		}
		return uistate.T("reports.noMember")
	}
	taxYear := from.Year()
	ys := time.Date(taxYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	ye := time.Date(taxYear+1, time.January, 1, 0, 0, 0, 0, time.UTC)
	exportItem := func(testID, label string, on func()) ui.Node {
		return Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", testID), OnClick(on), label)
	}
	exportHidden := ""
	if !exportOpenV {
		exportHidden = " hidden-menu"
	}
	exportMenu := Div(css.Class("add-wrap"), Attr("id", "rpt-export"),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "reports-export-toggle"),
			Attr("aria-haspopup", "menu"), Attr("aria-expanded", boolStr(exportOpenV)),
			Title(uistate.T("reports.exportTitle")), OnClick(onToggleExport),
			uistate.T("reports.exportCsv")),
		Div(ClassStr("add-menu"+exportHidden), Attr("role", "menu"), OnClick(onCloseExport),
			exportItem("reports-export-category", uistate.T("reports.byCategory"), func() {
				downloadBytes(reports.ExportFilename("spending-by-category", res, from), "text/csv", reports.CategoryCSV(rows, nameOf, csvAmount))
			}),
			exportItem("reports-export-income", uistate.T("reports.incomeBySource"), func() {
				downloadBytes(reports.ExportFilename("income-by-source", res, from), "text/csv", reports.CategoryCSV(incomeRows, nameOf, csvAmount))
			}),
			exportItem("reports-export-payees", uistate.T("reports.topPayees"), func() {
				downloadBytes(reports.ExportFilename("top-payees", res, from), "text/csv", reports.PayeeCSV(payees, csvAmount))
			}),
			exportItem("reports-export-largest", uistate.T("reports.biggestExpenses"), func() {
				downloadBytes(reports.ExportFilename("largest-expenses", res, from), "text/csv", reports.LargestExpensesCSV(largest, nameOf, csvAmount))
			}),
			exportItem("reports-export-member", uistate.T("reports.byMember"), func() {
				downloadBytes(reports.ExportFilename("spending-by-member", res, from), "text/csv", reports.MemberCSV(memberSpend, memberNm, csvAmount))
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
	return Div(css.Class("rpta-toolbar"),
		Div(css.Class("rpta-toolbar-row"),
			Button(ClassStr(scopeCls), Type("button"), Attr("aria-pressed", boolStr(scopeOpenV)),
				Attr("data-testid", "reports-scope-toggle"), Title(uistate.T("reports.scopeHint")),
				OnClick(onToggleScope), Text(scopeLabel)),
			Button(ClassStr(metricsCls), Type("button"), Attr("aria-pressed", boolStr(formulasOn)),
				Attr("data-testid", "reports-toggle-formulas"), Title(uistate.T("reports.metricsTitle")),
				OnClick(toggleFormulas), Text(metricsLabel)),
			exportMenu,
		),
		If(scopeOpenV, ui.CreateElement(ScopeSelector)),
	)
}

// rptaCatRowProps drives one row of the full-year category review table.
type rptaCatRowProps struct {
	CategoryID, Name   string
	Amount, Prior      int64
	HasDelta           bool
	DeltaPct           int64
	PriorZero          bool
	TotalSpend, MaxCat int64
	Spark              []int64
	FmtMinor           func(int64) string
	OnDrill            func(string)
}

// rptaCatRow is one category line: name (drillable), year total, monthly average,
// a 12-month sparkline, the vs-prior-year delta, and the share of all spending.
// Its own component so the drill hook sits at a stable call-site.
func rptaCatRow(props rptaCatRowProps) ui.Node {
	drill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(props.CategoryID)
		}
	}))
	amt := props.Amount
	if amt < 0 {
		amt = -amt
	}
	share := int64(0)
	if props.TotalSpend > 0 {
		share = amt * 100 / props.TotalSpend
	}
	delta := "—"
	deltaCls := "rpta-cat-delta rpta-muted"
	if props.PriorZero {
		delta = uistate.T("rpta.newCat")
	} else if props.HasDelta {
		if props.DeltaPct > 0 {
			delta = fmt.Sprintf("▲ %d%%", props.DeltaPct)
			deltaCls = "rpta-cat-delta rpta-tone-down" // spending UP is bad
		} else if props.DeltaPct < 0 {
			delta = fmt.Sprintf("▼ %d%%", -props.DeltaPct)
			deltaCls = "rpta-cat-delta rpta-tone-up"
		} else {
			delta = "0%"
		}
	}
	widthPct := int64(0)
	if props.MaxCat > 0 {
		widthPct = amt * 100 / props.MaxCat
	}
	return Div(css.Class("rpta-cat-row"), Attr("data-testid", "reports-cat-row"), Attr("data-category-id", props.CategoryID),
		Button(css.Class("rpta-cat-name"), Type("button"), Attr("data-testid", "reports-cat-drill"),
			Title(uistate.T("reports.drillTitleCat")), OnClick(drill),
			Span(props.Name),
			Div(css.Class("share-bar"), Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", widthPct)}))),
		),
		Span(css.Class("rpta-cat-amt", tw.FontDisplay), props.FmtMinor(amt)),
		Span(css.Class("rpta-cat-avg", "rpta-muted"), props.FmtMinor(amt/12)),
		Span(css.Class("rpta-cat-spark"), sparklineSVG(props.Spark)),
		Span(ClassStr(deltaCls), delta),
		Span(css.Class("rpta-cat-share", "rpta-muted"), fmt.Sprintf("%d%%", share)),
	)
}

// anyify converts a node slice to the []any Fragment expects.
func anyify(nodes []ui.Node) []any {
	out := make([]any, len(nodes))
	for i, n := range nodes {
		out[i] = n
	}
	return out
}
