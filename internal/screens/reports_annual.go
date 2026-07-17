// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/safespend"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/vitals"
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
	// The report reads the /reports-local scope merged INSIDE the app-wide
	// "Viewing as" lens — a filter chosen here never rewrites what other pages
	// show (the parity scan's "report scope leaks globally" defect).
	scopeAtom := uistate.UseReportScope()
	lensScope := uistate.UseActiveScope().Get()

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

	sc := scope.Merge(lensScope, scopeAtom.Get())
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
	// The panel auto-opens only for a REPORT-local scope — an app-wide lens
	// ("Viewing as …") is announced by the top-bar banner, not this panel.
	scopeOpen := ui.UseState(!scopeAtom.Get().IsAll())
	onToggleScope := ui.UseEvent(Prevent(func() { scopeOpen.Set(!scopeOpen.Get()) }))
	// QA CF-01/UX-03: a persisted report scope must stay conspicuous. One click
	// clears the REPORT-local scope (never the app-wide lens).
	onResetScope := ui.UseEvent(Prevent(func() {
		uistate.SetReportScope(scope.ReportScope{})
		scopeOpen.Set(false)
	}))
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

	// Net worth follows the report's scope like every other figure (QA CF-07:
	// scoped income/spending beside an unscoped $152k net worth read as a broken
	// filter). The scope resolves to account IDs, so filter the accounts and use
	// the already-scoped transactions for the series and the level.
	inScope := make(map[string]bool, len(scopeIDs))
	for _, id := range scopeIDs {
		inScope[id] = true
	}
	scopedAccounts := make([]domain.Account, 0, len(accounts))
	for _, a := range accounts {
		if inScope[a.ID] {
			scopedAccounts = append(scopedAccounts, a)
		}
	}
	nwBounds := append([]time.Time{}, bounds...)
	nwSeries, _ := ledger.NetWorthSeries(scopedAccounts, scopedTxns, nwBounds, rates)
	nwNet, _, _, _ := ledger.NetWorth(scopedAccounts, scopedTxns, rates)
	var nwChange int64
	if n := len(nwSeries); n >= 2 {
		nwChange = nwSeries[n-1].Amount - nwSeries[0].Amount
	}

	// Health: the deterministic score + factors + prioritized steps.
	health := healthscore.Evaluate(liveHealthInputs(app, time.Now()))

	// Credit proxy score at each month end (bounds[1..12] are the month-end
	// cutoffs; transactions strictly before each cutoff count, mirroring
	// NetWorthSeries). Scoped like net worth (QA CF-01): a report narrowed to
	// accounts with no cards simply omits the credit section.
	var creditSeries []int
	hasCards := false
	for _, a := range scopedAccounts {
		if a.Type == domain.TypeCreditCard && !a.Archived {
			hasCards = true
			break
		}
	}
	if hasCards {
		for k := 1; k < len(bounds); k++ {
			cutoff := bounds[k]
			upto := make([]domain.Transaction, 0, len(scopedTxns))
			for _, t := range scopedTxns {
				if t.Date.Before(cutoff) {
					upto = append(upto, t)
				}
			}
			balances := make(map[string]int64, 4)
			for _, a := range scopedAccounts {
				if a.Type != domain.TypeCreditCard || a.Archived {
					continue
				}
				if bal, err := ledger.Balance(a, upto); err == nil {
					balances[a.ID] = bal.Amount
				}
			}
			cr := credithealth.Evaluate(credithealth.Inputs{Accounts: scopedAccounts, Balances: balances, Transactions: upto, Now: cutoff})
			creditSeries = append(creditSeries, cr.ProxyScore)
		}
	}

	// The year's fee + interest charges ("money that bought nothing", §07).
	costs, _ := reports.CostOfMoney(scopedTxns, cats, as, ae, rates)

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
	tagSpend, _ := reports.SpendingByTag(scopedTxns, as, ae, true, ps, pe, rates)
	spendStats, _ := reports.SpendingStats(scopedTxns, as, ae, rates)
	noSpendDays := reports.NoSpendDays(scopedTxns, as, ae, time.Now())
	weekday, _ := reports.SpendingByWeekday(scopedTxns, as, ae, rates)
	// QA R3 CF-03: the review's subscription section counted every detected
	// recurring pattern — utilities, pharmacy runs, liability payments — into
	// "N recurring charges ≈ $X/yr". It now applies the same classification the
	// Subscriptions surface uses (essentials, liability payments, and planned
	// recurring flows excluded), so the headline total only carries charges the
	// user could actually cancel.
	subsRecurringNames := make(map[string]bool)
	for _, r := range app.Recurring() {
		if n := strings.ToLower(strings.TrimSpace(r.Label)); n != "" {
			subsRecurringNames[n] = true
		}
	}
	subsCatNameOf := func(id string) string { return catName[id] }
	subsAccounts := app.Accounts()
	subs, _ := subscriptions.Detect(scopedTxns, rates, 3)
	liveSubs := subs[:0:0]
	for _, s := range subs {
		if s.Lapsed(time.Now()) {
			continue
		}
		if !subscriptions.IsRealSubscriptionName(s.Name, scopedTxns, subsAccounts, subsCatNameOf, subsRecurringNames) {
			continue
		}
		liveSubs = append(liveSubs, s)
	}
	priceRises, _ := subscriptions.DetectPriceChanges(scopedTxns, rates, 3)
	rises := priceRises[:0:0]
	for _, pc := range priceRises {
		if pc.Delta > 0 && subscriptions.IsRealSubscriptionName(pc.Name, scopedTxns, subsAccounts, subsCatNameOf, subsRecurringNames) {
			rises = append(rises, pc)
		}
	}
	monthsRed := reports.MonthsNegative(monthFlows)
	// QA CF-23: the in-progress month (17 days of July) must not rank against
	// eleven complete months as the year's "lightest" — find the partial month
	// in the window (the one containing today) and exclude it from the extremes.
	partialIdx := -1
	nowSeasonal := time.Now()
	for k := 0; k+1 < len(bounds); k++ {
		if !nowSeasonal.Before(bounds[k]) && nowSeasonal.Before(bounds[k+1]) {
			partialIdx = k
			break
		}
	}
	hiIdx, loIdx, seasonalOK := reports.SeasonalExtremesSkipping(monthFlows, partialIdx)
	trims := reports.TrimTargets(catTrends, 2500, 3) // ≥$25/mo recent average
	// Top-10 to match the money-flow diagram, so the table's "everything else"
	// and the diagram's are the same number.
	per100 := reports.Per100(rows, flow.Income, 10)

	// Goals over the year (household-wide; classify is cheap).
	gc := goalsvc.CountByState(app.Goals(), app.Tasks(), time.Now(), true)

	// ── 00 · Where you stand: today's position, household-wide like net worth
	// (a balance sheet has no scope). Trailing 6-month averages feed the
	// capacity figures; the essential-month basis and debt list feed the rest.
	// All judgment lives in the pure internal/vitals core.
	vitFlows := lastN(monthFlows, 6)
	essBasis := buildSmartInput(app, pr.WeekStartWeekday()).EssentialBasis()
	toBaseVit := safespend.ToBaseFunc(rates)
	liquidAll, _ := ledger.LiquidBalance(accounts, txns, rates)
	var vitDebts []vitals.Debt
	vitCards := vitals.Cards{}
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		bal, err := ledger.Balance(a, txns)
		if err != nil {
			continue
		}
		mag := absMinor(toBaseVit(bal.Amount, bal.Currency))
		if a.Type == domain.TypeCreditCard {
			vitCards.HasCards = true
			vitCards.BalanceMinor += mag
			vitCards.LimitMinor += toBaseVit(a.CreditLimit.Amount, a.CreditLimit.Currency)
		}
		if mag == 0 && a.MinPayment.Amount == 0 {
			continue
		}
		vitDebts = append(vitDebts, vitals.Debt{
			Name:            a.Name,
			BalanceMinor:    mag,
			AprPercent:      a.InterestRateAPR,
			MinPaymentMinor: toBaseVit(a.MinPayment.Amount, a.MinPayment.Currency),
			IsMortgage:      a.Type == domain.TypeMortgage,
			InPayoff:        a.IncludedInPayoff(),
		})
	}
	vt := vitals.Evaluate(vitals.Inputs{
		IncomeMonthlyMinor:    reports.AverageMonthlyIncome(vitFlows),
		ExpenseMonthlyMinor:   reports.AverageMonthlyExpense(vitFlows),
		MonthsAveraged:        reports.ActiveMonths(vitFlows),
		EssentialMonthlyMinor: essBasis.EssentialMonthlyMinor(),
		LiquidMinor:           liquidAll.Amount,
		Debts:                 vitDebts,
		Cards:                 vitCards,
	})
	standSec := rptaVitalsSection(vt, reports.ActiveMonths(vitFlows), essBasis.FixedMonthlyMinor, essBasis.EssentialSpendMonthlyMinor, fmtMinor)

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
		nwInts := make([]int64, 0, len(nwSeries))
		for _, m := range nwSeries {
			nwInts = append(nwInts, m.Amount)
		}
		mastFigs = append(mastFigs, Div(css.Class("rpta-fig"), Attr("data-testid", "reports-hero-networth"),
			Span(css.Class("rpta-fig-k"), uistate.T("dashboard.netWorth")),
			Span(css.Class("rpta-fig-v", tw.FontDisplay), fmtMoney(nwNet)),
			If(sub != "", Span(ClassStr("rpta-fig-sub rpta-tone-"+tone), sub)),
			If(len(nwInts) >= 2, Div(css.Class("rpta-fig-spark"), Title(uistate.T("rpta.nwSparkTitle")),
				sparklineSVG(nwInts, uistate.T("rpta.nwSparkAlt")),
				Span(css.Class("rpta-fig-spark-cap"), uistate.T("rpta.nwSparkCap"), " · ", rptaSrcLink("nav.netWorth", "/networth")))),
		))
	}
	// Partial-period honesty (parity scan): when the window's newest month is
	// still in progress, the masthead says so — a 17-day July beside eleven
	// complete months must never read as a spending collapse.
	var partialChip ui.Node = Fragment()
	if nowM := time.Now(); !ae.After(dateutil.AddMonths(dateutil.MonthStart(nowM), 1)) && ae.After(nowM) {
		daysIn := nowM.Day()
		daysTotal := dateutil.AddMonths(dateutil.MonthStart(nowM), 1).AddDate(0, 0, -1).Day()
		partialChip = Span(css.Class("rpta-partial-chip"), Attr("data-testid", "rpta-partial-chip"),
			uistate.T("rpta.partialMonth", nowM.Format("January"), daysIn, daysTotal))
	}
	// QA CF-01/UX-03: when a REPORT-local scope is active, a plain-language
	// sentence stays visible in the masthead even with the Scope panel closed,
	// with a one-click reset. Household-wide figures (health score, Where you
	// stand) are named so a scoped report never implies they follow the filter.
	var scopeLine ui.Node = Fragment()
	if local := scopeAtom.Get(); !local.IsAll() {
		scopeLine = Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FlexWrap), Attr("data-testid", "rpta-scope-line"),
			Span(css.Class("rpta-partial-chip"), uistate.T("rpta.scopeShowing", rptaScopeSummary(local, accounts, app.Members()), windowLine)),
			Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "rpta-scope-reset"),
				OnClick(onResetScope), uistate.T("rpta.scopeReset")),
		)
	}
	masthead := Div(css.Class("rpta-masthead"), Attr("data-testid", "rpt-hero"), Attr("id", "rpta-top"),
		P(css.Class("rpta-eyebrow"), uistate.T("rpta.eyebrow")),
		H1(css.Class("rpta-title", tw.FontDisplay), windowLine),
		partialChip,
		scopeLine,
		Div(ClassStr("rpta-verdict rpta-tone-"+verdictTone), Attr("data-testid", "rpta-verdict"),
			Span(css.Class("rpta-verdict-score", tw.FontDisplay), rptaScoreText(health)),
			Span(css.Class("rpta-verdict-line"), verdict),
			rptaSrcLink("nav.health", "/health"),
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
	askStrong := fmt.Sprintf("health score %s; ", rptaScoreText(health))
	for _, f := range health.Factors {
		if f.Weight > 0 && f.Score >= 70 {
			askStrong += fmt.Sprintf("%s = %s (score %d); ", f.Label, f.Value, f.Score)
		}
	}
	strengths := rptaSection("rpta-01", "01", uistate.T("rpta.secStrong"), "up", uistate.T("rpta.secStrongSub"), askStrong, Fragment(
		If(len(strongFacts) == 0, P(css.Class("rpta-muted"), uistate.T("rpta.noStrong"))),
		Div(css.Class("rpta-facts"), strongFacts),
		If(len(wins) > 0, Div(css.Class("rpta-wins"), Attr("data-testid", "rpta-wins"), wins)),
		Div(css.Class("rpta-srcrow"), rptaSrcLink("nav.health", "/health")),
	))

	// ── 02 · The flow of money (in-house smooth-ribbon sankey + per-$100). ───
	sankeyFactor := int64(1)
	for i := 0; i < decimals; i++ {
		sankeyFactor *= 10
	}
	accent := chartLineColor(uistate.CurrentAccent())
	flowColors := newRptaFlowColors(accent)
	var moneyFlows []reports.Flow
	// Sources → Income (the enhancement: where the money comes FROM).
	incomeLabel := uistate.T("rpta.nodeIncome")
	flowColors.hub(incomeLabel)
	srcCount := 0
	var srcRest int64
	for _, r := range incomeRows {
		if r.Amount <= 0 {
			continue
		}
		if srcCount < 5 {
			flowColors.source(nameOf(r.CategoryID))
			moneyFlows = append(moneyFlows, reports.Flow{From: nameOf(r.CategoryID), To: incomeLabel, Value: r.Amount})
			srcCount++
		} else {
			srcRest += r.Amount
		}
	}
	if srcRest > 0 {
		flowColors.rest(uistate.T("rpta.nodeOtherIncome"))
		moneyFlows = append(moneyFlows, reports.Flow{From: uistate.T("rpta.nodeOtherIncome"), To: incomeLabel, Value: srcRest})
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
			flowColors.category(nameOf(r.CategoryID))
			moneyFlows = append(moneyFlows, reports.Flow{From: incomeLabel, To: nameOf(r.CategoryID), Value: v})
			catCount++
		} else {
			catRest += v
		}
	}
	if catRest > 0 {
		flowColors.rest(uistate.T("rpta.nodeEverythingElse"))
		moneyFlows = append(moneyFlows, reports.Flow{From: incomeLabel, To: uistate.T("rpta.nodeEverythingElse"), Value: catRest})
	}
	if sav := flow.Net(); sav > 0 {
		flowColors.savings(uistate.T("rpta.nodeSavings"))
		moneyFlows = append(moneyFlows, reports.Flow{From: incomeLabel, To: uistate.T("rpta.nodeSavings"), Value: sav})
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
	if kv := flow.Net(); kv > 0 && flow.Income > 0 {
		scaled := kv * 1000 / flow.Income
		per100Rows = append(per100Rows, Tr(css.Class("rpta-tr-kept"),
			Td(css.Class("rpta-td-name"), per100Dot("#4ea777"), uistate.T("rpta.nodeSavings")),
			Td(css.Class("rpta-td-num"), fmt.Sprintf("$%d.%d0", scaled/10, scaled%10)),
			Td(css.Class("rpta-td-num", "rpta-muted"), fmtMinor(kv)),
		))
	}
	askFlow := fmt.Sprintf("income %s from %d sources; spending %s; savings %s (%d%% of income)",
		fmtMinor(flow.Income), srcCount, fmtMinor(flow.Expense), fmtMinor(flow.Net()), flow.SavingsRate())
	if len(rows) > 0 {
		askFlow += fmt.Sprintf("; biggest category %s at %s", nameOf(rows[0].CategoryID), fmtMinor(absMinor(rows[0].Amount)))
	}
	flowSec := rptaSection("rpta-02", "02", uistate.T("reports.moneyFlow"), "up", uistate.T("rpta.secFlowSub"), askFlow, Fragment(
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
			Table(css.Class("rpta-table", "rpta-per100"), Attr("data-testid", "rpta-per100"),
				Thead(Tr(Th(uistate.T("rpta.per100Where")), Th("/$100"), Th(uistate.T("rpta.per100Year")))),
				Tbody(per100Rows),
			),
			A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "moneyflow-drill"), uistate.T("reports.viewTransactions")),
		),
	))

	// ── 03 · The year in motion (monthly review table + trends). ─────────────
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
			InProgress: !bounds[i+1].Before(time.Now()) && bounds[i].Before(time.Now()),
		})
		monthRows = append(monthRows, Tr(ClassStr(rowCls),
			Td(css.Class("rpta-td-name"), nameCell),
			Td(css.Class("rpta-td-num"), fmtMinor(f.Income)),
			Td(css.Class("rpta-td-num"), fmtMinor(f.Expense)),
			Td(ClassStr("rpta-td-num rpta-td-strong"+If2(f.Net() < 0, " rpta-tone-down", "")), fmtMinor(f.Net())),
			Td(css.Class("rpta-td-num", "rpta-td-kept"), rate, keptMeter),
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
	motion := rptaSection("rpta-03", "03", uistate.T("rpta.secMotion"), "neutral", uistate.T("rpta.secMotionSub"), askMotion, Fragment(
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
			If(len(creditSeries) >= 2, Div(Attr("data-testid", "rpta-credit-chart"), rptaTrendChart(uistate.T("rpta.creditHead"), creditStroke, uistate.T("rpta.legendCredit"),
				rptaSrcLink("nav.credit", "/credit"),
				fmt.Sprintf("%d", int(seriesMax(creditFloat))), fmt.Sprintf("%d", int(seriesMid(creditFloat))), fmt.Sprintf("%d", int(seriesMin(creditFloat))),
				uiw.AreaChart(uiw.AreaChartProps{Values: creditFloat, Stroke: creditStroke, GradientID: "rpta-credit", Label: uistate.T("rpta.creditHead"), Labels: monthLabels, ValueLabels: pctlessVL(creditFloat)})))),
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
			Dot:    flowColors.of(nameOf(r.CategoryID)),
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
	// The year histogram: one colored magnitude bar per category (sankey-palette
	// hues for the diagram's top ten), replacing a page of airy rows. The full
	// analytic table (run rates, sparklines) folds beneath it.
	// Top 12 bars + one aggregated "everything else" row — the long tail lives
	// in the folded table, not as fifteen more sliver bars.
	var catHist []ui.Node
	var histRest int64
	histRestN := 0
	for _, r := range rows {
		if r.Amount == 0 {
			continue
		}
		amt := absMinor(r.Amount)
		if len(catHist) >= 12 {
			histRest += amt
			histRestN++
			continue
		}
		share := int64(0)
		if flow.Expense > 0 {
			share = amt * 100 / flow.Expense
		}
		id := r.CategoryID
		catHist = append(catHist, ui.CreateElement(rptaHistRow, rptaHistRowProps{
			Label: nameOf(r.CategoryID), Color: flowColors.of(nameOf(r.CategoryID)),
			Amount: amt, Max: maxCat, Meta: fmt.Sprintf("%d%%", share),
			HasDelta: r.HasDelta, DeltaPct: r.DeltaPct, PriorZero: r.PriorZero,
			FmtMinor: fmtMinor, OnSelect: func() { drillCategory(id) },
		}))
	}
	if histRest > 0 {
		share := int64(0)
		if flow.Expense > 0 {
			share = histRest * 100 / flow.Expense
		}
		catHist = append(catHist, ui.CreateElement(rptaHistRow, rptaHistRowProps{
			Label: uistate.T("rpta.histRest", histRestN), Color: "#8a8f98",
			Amount: histRest, Max: maxCat, Meta: fmt.Sprintf("%d%%", share),
			FmtMinor: fmtMinor,
		}))
	}
	// Tags render beside the categories on the SAME spending scale (maxCat), so
	// a tag bar and a category bar are directly comparable.
	var tagNodes []ui.Node
	for i, ts := range tagSpend {
		if i >= 10 {
			break
		}
		hasDelta := ts.Prior > 0
		var deltaPct int64
		if hasDelta {
			deltaPct = (ts.Amount - ts.Prior) * 100 / ts.Prior
		}
		tagNodes = append(tagNodes, Div(Attr("data-testid", "rpta-tag-row"), ui.CreateElement(rptaHistRow, rptaHistRowProps{
			Label: "#" + ts.Tag, Chip: true,
			Color:  rptaCatPalette[i%len(rptaCatPalette)],
			Amount: ts.Amount, Max: maxCat, Meta: uistate.T("rpta.tagCharges", ts.Count),
			HasDelta: hasDelta, DeltaPct: deltaPct, PriorZero: ts.Prior == 0 && ts.Amount > 0,
			FmtMinor: fmtMinor,
		})))
	}
	// The roll-up control is a state TOGGLE (aria-pressed), so it wears the
	// page's toggle component — strip-toggle with is-on chrome — not a plain btn
	// whose pressed state would be invisible.
	rollupCls := "strip-toggle"
	if rollupCats.Get() {
		rollupCls += " is-on"
	}
	catActions := Div(css.Class(tw.Flex, tw.Gap2),
		Button(ClassStr(rollupCls+" "+tw.Fold(tw.Gap2)), Type("button"), Attr("data-testid", "reports-rollup-toggle"),
			Attr("aria-pressed", boolStr(rollupCats.Get())), Title(uistate.T("reports.rollupTitle")),
			OnClick(onToggleRollup),
			uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T(rollupLabelKey(rollupCats.Get())))),
	)
	categories := rptaSectionWithAction("rpta-04", "04", uistate.T("rpta.secCats"), "neutral", uistate.T("rpta.secCatsSub"), narrative, catActions, Fragment(
		P(css.Class("rpta-narrative", tw.FontDisplay), narrative),
		Div(css.Class("rpta-hist-scale"), uistate.T("rpta.histScale", fmtMinor(maxCat))),
		Div(css.Class("rpta-hist"), Attr("data-testid", "rpta-cat-hist"), catHist),
		If(len(tagNodes) > 0, Fragment(
			rptaSubG("↘", "down", uistate.T("rpta.byTag"), A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "tags-drill"), uistate.T("reports.viewTransactions"))),
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
		payeeNodes = append(payeeNodes, quietRow(nm, "", p.Amount))
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
	type depGroup struct {
		desc   string
		amount int64
		count  int
		last   time.Time
	}
	var depGroups []depGroup
	for _, e := range bigIncome {
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		folded := false
		for gi := range depGroups {
			if depGroups[gi].desc == desc && depGroups[gi].amount == e.Amount {
				depGroups[gi].count++
				if e.Date.After(depGroups[gi].last) {
					depGroups[gi].last = e.Date
				}
				folded = true
				break
			}
		}
		if !folded {
			depGroups = append(depGroups, depGroup{desc: desc, amount: e.Amount, count: 1, last: e.Date})
		}
	}
	var depositNodes []ui.Node
	for i, g := range depGroups {
		if i >= 4 {
			break
		}
		meta := pr.FormatDate(g.last)
		if g.count > 1 {
			meta = uistate.T("rpta.depTimes", g.count)
		}
		depositNodes = append(depositNodes, quietRow(g.desc, meta, g.amount))
	}
	var srcNodes []ui.Node
	for _, r := range incomeRows {
		if r.Amount == 0 {
			continue
		}
		srcNodes = append(srcNodes, quietRow(nameOf(r.CategoryID), "", r.Amount))
	}
	memberName := make(map[string]string, len(app.Members()))
	for _, m := range app.Members() {
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
	whereGoes := rptaSection("rpta-05", "05", uistate.T("rpta.secWhere"), "neutral", uistate.T("rpta.secWhereSub"), askWhere, Fragment(Div(css.Class("rpta-cols2"),
		Div(css.Class("rpta-col"),
			rptaSubG("↘", "down", uistate.T("reports.topPayees"), A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "payees-drill"), uistate.T("reports.viewTransactions"))),
			listRows(payeeNodes),
			rptaSubG("↗", "up", uistate.T("reports.biggestDeposits"), nil),
			listRows(depositNodes),
			If(len(app.Members()) >= 2 && len(memberNodes) > 0, Fragment(
				rptaSubG("↘", "down", uistate.T("reports.byMember"), nil),
				listRows(memberNodes))),
		),
		Div(css.Class("rpta-col"),
			rptaSubG("↘", "down", uistate.T("reports.biggestExpenses"), A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "expenses-drill"), uistate.T("reports.viewTransactions"))),
			listRows(largestNodes),
			rptaSubG("↗", "up", uistate.T("reports.incomeBySource"), A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "income-drill"), uistate.T("reports.viewTransactions"))),
			listRows(srcNodes),
		),
	)))

	// ── 06 · Goal progress: every financial goal's year, coverage-first. ─────
	tasks := app.Tasks()
	nowT := time.Now()
	var goalRows []ui.Node
	archivedReached := 0
	for _, g := range app.Goals() {
		if g.TargetAmount.Amount <= 0 {
			continue
		}
		if g.Archived {
			archivedReached++
			continue
		}
		state := goalsvc.Classify(g, tasks, nowT)
		covPct := goalsvc.CoveragePercent(g)
		savedPct := 0
		if g.TargetAmount.Amount > 0 {
			savedPct = int(g.CurrentAmount.Amount * 100 / g.TargetAmount.Amount)
			if savedPct > 100 {
				savedPct = 100
			}
			if savedPct < 0 {
				savedPct = 0
			}
		}
		tone, chipKey := "", "rpta.goalStateCurrent"
		barColor := rptaToneUp
		switch state {
		case goalsvc.StateCompleted:
			tone, chipKey = "up", "rpta.goalStateReached"
		case goalsvc.StateMissed:
			tone, chipKey, barColor = "down", "rpta.goalStateMissed", rptaToneDown
		default:
			tone = "dim"
			if !g.TargetDate.IsZero() && g.TargetDate.Before(nowT.AddDate(0, 3, 0)) && covPct < 80 {
				// Due within a quarter and materially short: worth amber.
				tone, barColor = "warn", rptaToneWarn
			}
		}
		deadline := ""
		if !g.TargetDate.IsZero() {
			deadline = uistate.T("rpta.goalBy", g.TargetDate.Format("Jan 2006"))
		}
		goalRows = append(goalRows, Div(css.Class("rpta-goal-row"), Attr("data-testid", "rpta-goal-row"),
			Div(css.Class("rpta-goal-top"),
				Span(css.Class("rpta-goal-name"), g.Name),
				Span(ClassStr("rpta-goal-chip rpta-chip-"+tone), uistate.T(chipKey)),
				Span(css.Class("rpta-goal-when", "rpta-muted"), deadline),
				Span(css.Class("rpta-goal-fig", tw.FontDisplay), uistate.T("rpta.goalBacked", fmtMoney(money.New(goalsvc.CoverageMinor(g), base)), fmtMoney(g.TargetAmount), covPct)),
			),
			Div(css.Class("rpta-goal-track"), Attr("aria-hidden", "true"),
				Div(css.Class("rpta-goal-cov"), Style(map[string]string{"width": fmt.Sprintf("%d%%", covPct), "background": barColor})),
				Div(css.Class("rpta-goal-saved"), Style(map[string]string{"width": fmt.Sprintf("%d%%", savedPct), "background": barColor})),
			),
		))
	}
	askGoals := uistate.T("rpta.goalsSummary", gc.Completed, gc.Current, gc.Missed)
	goalsSec := Fragment()
	if len(goalRows) > 0 || gc.Completed+gc.Missed > 0 {
		goalsSec = rptaSection("rpta-06", "06", uistate.T("rpta.secGoals"), "neutral", uistate.T("rpta.secGoalsSub"), askGoals, Fragment(
			P(css.Class("rpta-muted"), Attr("data-testid", "rpta-goals-summary"), askGoals+If2(archivedReached > 0, " "+uistate.T("rpta.goalsArchived", archivedReached), "")),
			Div(css.Class("rpta-goal-rows"), goalRows),
			A(css.Class("rpta-drill"), Href(uistate.RoutePath("/goals")), Attr("data-testid", "rpta-goals-drill"), uistate.T("rpta.openGoals")),
		))
	}

	// ── 07 · Budget adherence: month-by-month cells per budget. ──────────────
	periodsPerYear := func(p domain.Period) int64 {
		switch p {
		case domain.PeriodWeekly:
			return 52
		case domain.PeriodBiweekly:
			return 26
		case domain.PeriodSemimonthly:
			return 24
		case domain.PeriodQuarterly:
			return 4
		case domain.PeriodYearly:
			return 1
		default:
			return 12
		}
	}
	budgets := app.Budgets()
	var budgetRows []ui.Node
	budgetsClean := 0
	budgetsCounted := 0
	for _, bg := range budgets {
		limConv, err := rates.Convert(bg.Limit, rates.Base)
		if err != nil || limConv.Amount <= 0 {
			continue
		}
		limit := limConv.Amount
		name := bg.Name
		if name == "" {
			name = catName[bg.CategoryID]
		}
		budgetsCounted++
		if bg.Period == domain.PeriodMonthly || bg.Period == "" {
			var cells []ui.Node
			over := 0
			for k := 0; k+1 < len(bounds) && k < 12; k++ {
				sp, err := budgeting.Spent(bg, scopedTxns, bounds[k], bounds[k+1], rates)
				if err != nil {
					continue
				}
				spent := sp.Amount
				cls := "rpta-bud-cell "
				switch {
				case spent == 0:
					cls += "rpta-bud-quiet"
				case spent <= limit:
					cls += "rpta-bud-under"
				case spent <= limit+limit/10:
					cls += "rpta-bud-near"
				default:
					cls += "rpta-bud-over"
					over++
				}
				cells = append(cells, Span(ClassStr(cls), Title(bounds[k].Format("Jan")+": "+fmtMinor(spent)+" / "+fmtMinor(limit))))
			}
			if over == 0 {
				budgetsClean++
			}
			verdictCls, verdictTxt := "rpta-tone-up", uistate.T("rpta.budWithin")
			if over > 0 {
				verdictCls, verdictTxt = "rpta-tone-down", uistate.T("rpta.budMonthsOver", over)
			}
			budgetRows = append(budgetRows, Div(css.Class("rpta-bud-row"), Attr("data-testid", "rpta-bud-row"),
				Span(css.Class("rpta-bud-name"), name),
				Div(css.Class("rpta-bud-cells"), cells),
				Span(ClassStr("rpta-bud-verdict "+verdictCls), verdictTxt),
			))
		} else {
			yearCap := limit * periodsPerYear(bg.Period)
			sp, err := budgeting.Spent(bg, scopedTxns, as, ae, rates)
			if err != nil {
				continue
			}
			spent := sp.Amount
			tone := rptaToneUp
			verdictCls, verdictTxt := "rpta-tone-up", uistate.T("rpta.budWithin")
			switch {
			case spent > yearCap+yearCap/10:
				tone, verdictCls, verdictTxt = rptaToneDown, "rpta-tone-down", uistate.T("rpta.budOverYear", fmtMinor(spent-yearCap))
			case spent > yearCap:
				tone, verdictCls, verdictTxt = rptaToneWarn, "rpta-tone-warn", uistate.T("rpta.budOverYear", fmtMinor(spent-yearCap))
			default:
				budgetsClean++
			}
			pct := int64(0)
			if yearCap > 0 {
				pct = spent * 100 / yearCap
				if pct > 100 {
					pct = 100
				}
			}
			budgetRows = append(budgetRows, Div(css.Class("rpta-bud-row"), Attr("data-testid", "rpta-bud-row"),
				Span(css.Class("rpta-bud-name"), name+" "+uistate.T("rpta.budAnnualized", string(bg.Period))),
				Div(css.Class("rpta-hist-track"), Div(css.Class("rpta-hist-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct), "background": tone}))),
				Span(ClassStr("rpta-bud-verdict "+verdictCls), verdictTxt),
			))
		}
	}
	// Month-initial header so the twelve adherence cells have a labeled x-axis.
	var budMonthCells []ui.Node
	for k := 0; k+1 < len(bounds) && k < 12; k++ {
		budMonthCells = append(budMonthCells, Span(css.Class("rpta-bud-cell", "rpta-bud-mon"), bounds[k].Format("Jan")[:1]))
	}
	budHeader := Div(css.Class("rpta-bud-row", "rpta-bud-header"), Attr("aria-hidden", "true"),
		Span(css.Class("rpta-bud-name")),
		Div(css.Class("rpta-bud-cells"), budMonthCells),
		Span(css.Class("rpta-bud-verdict")),
	)
	askBudgets := uistate.T("rpta.budgetsSummary", budgetsClean, budgetsCounted)
	budgetsSec := Fragment()
	if len(budgetRows) > 0 {
		budgetsSec = rptaSection("rpta-07", "07", uistate.T("rpta.secBudgets"), "neutral", uistate.T("rpta.secBudgetsSub"), askBudgets, Fragment(
			P(css.Class("rpta-muted"), Attr("data-testid", "rpta-budgets-summary"), askBudgets),
			Div(css.Class("rpta-bud-rows"), budHeader, budgetRows),
			Div(css.Class("rpta-flow-key"),
				Span(css.Class("rpta-flow-key-item"), Span(css.Class("rpta-flow-dot"), Style(map[string]string{"background": rptaToneUp})), uistate.T("rpta.budKeyUnder")),
				Span(css.Class("rpta-flow-key-item"), Span(css.Class("rpta-flow-dot"), Style(map[string]string{"background": rptaToneWarn})), uistate.T("rpta.budKeyNear")),
				Span(css.Class("rpta-flow-key-item"), Span(css.Class("rpta-flow-dot"), Style(map[string]string{"background": rptaToneDown})), uistate.T("rpta.budKeyOver")),
			),
			A(css.Class("rpta-drill"), Href(uistate.RoutePath("/budgets")), Attr("data-testid", "rpta-budgets-drill"), uistate.T("rpta.openBudgets")),
		))
	}

	// ── 08 · Watch list (rising, subscriptions, price creep). ────────────────
	var risingNodes []ui.Node
	for _, tr := range catTrends {
		if !tr.HasDelta || tr.DeltaPct < 25 || tr.Total < 10000 {
			continue
		}
		risingNodes = append(risingNodes, Div(css.Class("rpta-watch-row"),
			Span(css.Class("rpta-watch-name"), nameOf(tr.CategoryID)),
			sparklineSVG(tr.Spend, uistate.T("rpta.catSparkAlt")),
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
	askWatch := fmt.Sprintf("%d recurring charges ≈ %s a year; %d categories rising ≥25%%; %d price increases caught",
		len(liveSubs), fmtMinor(subsAnnual), len(risingNodes), len(rises))
	watch := rptaSection("rpta-08", "08", uistate.T("rpta.secWatch"), "warn", uistate.T("rpta.secWatchSub"), askWatch, Fragment(
		If(len(risingNodes) == 0 && len(subNodes) == 0 && len(riseNodes) == 0, P(css.Class("rpta-muted"), uistate.T("rpta.watchClear"))),
		If(len(risingNodes) > 0, Fragment(rptaSubG("⚠", "warn", uistate.T("rpta.watchRising"), nil), Div(Attr("data-testid", "rpta-rising"), risingNodes))),
		If(len(subNodes) > 0, Fragment(
			rptaSubG("⚠", "warn", uistate.T("rpta.watchSubs", len(liveSubs), fmtMinor(subsAnnual)), rptaSrcLink("nav.subscriptions", "/subscriptions")),
			Div(Attr("data-testid", "rpta-subs"), subNodes))),
		If(len(riseNodes) > 0, Fragment(rptaSubG("⚠", "warn", uistate.T("rpta.watchRises"), rptaSrcLink("nav.subscriptions", "/subscriptions")), Div(riseNodes))),
	))

	// ── 07 · Problem spots. ───────────────────────────────────────────────────
	var maxDebtInterest int64
	for _, d := range debts {
		if d.estYearMinor > maxDebtInterest {
			maxDebtInterest = d.estYearMinor
		}
	}
	var debtRowNodes []ui.Node
	for _, d := range debts {
		minStr := "—"
		if d.minimum.Amount > 0 {
			minStr = fmtMoney(d.minimum)
		}
		var dragBar ui.Node = Fragment()
		if maxDebtInterest > 0 {
			dragBar = Div(css.Class("rpta-bar-red"), Attr("aria-hidden", "true"),
				Div(css.Class("rpta-bar-red-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", d.estYearMinor*100/maxDebtInterest)})))
		}
		debtRowNodes = append(debtRowNodes, Tr(
			Td(css.Class("rpta-td-name"), d.name),
			Td(css.Class("rpta-td-num"), fmtMoney(d.balance.Abs())),
			Td(css.Class("rpta-td-num"), fmt.Sprintf("%.1f%%", d.apr)),
			Td(css.Class("rpta-td-num", "rpta-tone-down"), fmtMinor(d.estYearMinor), dragBar),
			Td(css.Class("rpta-td-num"), minStr),
		))
	}
	var problemBits []ui.Node
	if len(weakFacts) > 0 {
		problemBits = append(problemBits, rptaSub(uistate.T("rpta.probFactors"), rptaSrcLink("nav.health", "/health")), Div(css.Class("rpta-facts"), weakFacts))
	}
	if monthsRed > 0 {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line"), Attr("data-testid", "rpta-monthsred"),
			Span(css.Class("rpta-tone-down"), fmt.Sprintf("%d", monthsRed)+" "), uistate.T("rpta.monthsRed")))
	}
	if health.NegativeCashFlow {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line", "rpta-tone-down"), uistate.T("rpta.negCashFlow")))
	}
	// Money that bought nothing: the year's fee + interest charges, itemized.
	if costs.FeeCount+costs.InterestCount > 0 {
		head := ""
		switch {
		case costs.FeeCount > 0 && costs.InterestCount > 0:
			head = uistate.T("rpta.probCosts", fmtMinor(costs.FeeTotal), costs.FeeCount, fmtMinor(costs.InterestTotal), costs.InterestCount)
		case costs.FeeCount == 1:
			head = uistate.T("rpta.probFeesOnlyOne", fmtMinor(costs.FeeTotal))
		case costs.FeeCount > 1:
			head = uistate.T("rpta.probFeesOnly", fmtMinor(costs.FeeTotal), costs.FeeCount)
		case costs.InterestCount == 1:
			head = uistate.T("rpta.probInterestOnlyOne", fmtMinor(costs.InterestTotal))
		default:
			head = uistate.T("rpta.probInterestOnly", fmtMinor(costs.InterestTotal), costs.InterestCount)
		}
		var costRows []ui.Node
		for i, it := range costs.Items {
			if i >= 8 {
				costRows = append(costRows, Div(css.Class("row"), Span(css.Class("rpta-muted"), uistate.T("rpta.costMore", len(costs.Items)-8))))
				break
			}
			kind := uistate.T("rpta.costFee")
			if it.Interest {
				kind = uistate.T("rpta.costInterest")
			}
			costRows = append(costRows, Div(css.Class("row"), Attr("data-testid", "rpta-cost-row"),
				Div(css.Class("row-main"),
					Span(css.Class("rpta-cost-kind"), kind),
					Span(css.Class("row-desc"), it.Desc),
					Span(css.Class("row-meta"), pr.FormatDate(it.Date))),
				Span(css.Class("budget-amount", "rpta-tone-down"), fmtMinor(it.Amount))))
		}
		problemBits = append(problemBits,
			rptaSub(head, A(css.Class("rpta-drill"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "costs-drill"), uistate.T("reports.viewTransactions"))),
			Div(css.Class("rows"), Attr("data-testid", "rpta-costs"), costRows))
	}
	if len(debtRowNodes) > 0 {
		problemBits = append(problemBits,
			rptaSub(uistate.T("rpta.probDebt", fmtMinor(debtInterestTotal)), rptaSrcLink("nav.debt", "/debt")),
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
	askProblems := fmt.Sprintf("%d months in the red; %s estimated yearly debt interest; %s in fees; %s in interest charges",
		monthsRed, fmtMinor(debtInterestTotal), fmtMinor(costs.FeeTotal), fmtMinor(costs.InterestTotal))
	for _, f := range health.Factors {
		if f.Weight > 0 && f.Score < 70 {
			askProblems += fmt.Sprintf("; weak factor %s = %s (score %d)", f.Label, f.Value, f.Score)
		}
	}
	problems := rptaSection("rpta-09", "09", uistate.T("rpta.secProblems"), "down", uistate.T("rpta.secProblemsSub"), askProblems, Fragment(anyify(problemBits)...))

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
	if costs.FeeTotal >= 2500 { // $25+/yr in fees is worth a line in the plan
		feeDetail := ""
		if len(costs.Items) > 0 && !costs.Items[0].Interest {
			feeDetail = uistate.T("rpta.planFeesDetail", costs.Items[0].Desc, fmtMinor(costs.Items[0].Amount))
		}
		feeAction := uistate.T("rpta.planFees", fmtMinor(costs.FeeTotal), costs.FeeCount)
		if costs.FeeCount == 1 {
			feeAction = uistate.T("rpta.planFeesOne", fmtMinor(costs.FeeTotal))
		}
		addPlan(feeAction, feeDetail, "/transactions", uistate.T("nav.transactions"))
	}
	if len(liveSubs) >= 3 {
		addPlan(
			uistate.T("rpta.planSubs", len(liveSubs), fmtMinor(subsAnnual)),
			If2(len(rises) > 0, uistate.T("rpta.planSubsRises", len(rises)), ""),
			"/subscriptions", uistate.T("nav.subscriptions"))
	}
	askPlan := ""
	for i, st := range health.Steps {
		if i >= 3 {
			break
		}
		askPlan += st.Action + "; "
	}
	plan := rptaSection("rpta-10", "10", uistate.T("rpta.secPlan"), "plan", uistate.T("rpta.secPlanSub"), askPlan,
		Div(css.Class("rpta-plan"), Attr("data-testid", "rpta-plan"), planItems))

	// ── 09 · Appendix (tax, custom fields, metrics). ──────────────────────────
	winForExports := period.Window{Res: period.Year, From: as}
	deductible := deductibleSection(scopedTxns, cats, as, ae, rates, base, fmtMinor, winForExports)
	var appendixBits []ui.Node
	if deductible != nil {
		appendixBits = append(appendixBits, deductible)
	}
	if invperf := investmentPerformanceSection(scopedAccounts, scopedTxns, rates, base, fmtMinor, winForExports); invperf != nil {
		appendixBits = append(appendixBits, invperf)
	}
	if len(cfDefs) > 0 {
		appendixBits = append(appendixBits, customFieldSpendSection(scopedTxns, cfDefs, selectedCFKey.Get(), onCFKeyChange, as, ae, rates, base, fmtMinor, winForExports))
	}
	// #46: the Report-metrics builder opens in the app-standard flip modal
	// instead of expanding a very large builder inline in the appendix, far
	// from its toolbar trigger. Constructed unconditionally (FlipPanel carries
	// a hook), rendered only while open.
	metricsModal := uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("reports.metricsShow"),
		Width:     uiw.FlipLargeW,
		Height:    "min(90vh, 720px)",
		CloseOnly: true,
		OnClose:   func() { showFormulas.Set(false) },
		// Title "" → the builder's own "Formula calculator" heading, so the
		// modal header and the section heading never read as duplicates.
		Back: Div(Attr("data-testid", "reports-metrics-modal"),
			P(css.Class("rpta-muted"), uistate.T("reports.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{ShowSaved: true})),
	})
	var appendix ui.Node = Fragment()
	if len(appendixBits) > 0 {
		appendix = rptaSection("rpta-11", "11", uistate.T("rpta.secAppendix"), "dim", uistate.T("rpta.secAppendixSub"), "", Fragment(anyify(appendixBits)...))
	}

	return Div(css.Class("rpta"),
		masthead,
		toolbar,
		index,
		standSec,
		strengths,
		flowSec,
		motion,
		categories,
		whereGoes,
		goalsSec,
		budgetsSec,
		watch,
		problems,
		plan,
		appendix,
		If(showFormulas.Get(), metricsModal),
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

// rptaWin is one strengths-strip win chip, led by a check glyph.
func rptaWin(text string) ui.Node {
	return Span(css.Class("rpta-win"),
		Span(css.Class("rpta-win-check"), Attr("aria-hidden", "true"), "✓"),
		text)
}

// rptaSub is a small in-section subheading with an optional right-aligned action.
func rptaSub(title string, action ui.Node) ui.Node {
	return rptaSubG("", "", title, action)
}

// rptaSubG is rptaSub with a leading toned glyph (↗ money in, ↘ money out,
// ⚠ watch) so a list's direction/nature reads before its words do.
func rptaSubG(glyph, tone, title string, action ui.Node) ui.Node {
	if action == nil {
		action = Fragment()
	}
	var g ui.Node = Fragment()
	if glyph != "" {
		g = Span(ClassStr("rpta-sub-glyph rpta-tone-"+tone), Attr("aria-hidden", "true"), glyph+" ")
	}
	return Div(css.Class("rpta-subrow"),
		Span(css.Class("rpta-subhead"), g, title),
		action,
	)
}

// rptaSrcLink is the subtle "this number lives over there" affordance: a
// small muted page-name link with a trailing arrow, placed beside a metric's
// subhead or legend so drilling to the owning surface is one quiet click.
func rptaSrcLink(labelKey, route string) ui.Node {
	return A(css.Class("rpta-src"), Href(uistate.RoutePath(route)), Attr("data-testid", "rpta-src"),
		uistate.T(labelKey), " →")
}

// rptaTrendChart wraps one §03 trend chart with its full context: subhead
// (plus a quiet source-page link), a y-axis column (series max / mid / min,
// aligned to the 120px plot), the chart itself with its month x-labels, and
// the color-keyed legend beneath. No chart floats context-free.
func rptaTrendChart(head, stroke, legend string, src ui.Node, maxL, midL, minL string, chart ui.Node) ui.Node {
	if src == nil {
		src = Fragment()
	}
	return Div(css.Class("rpta-chart"),
		Div(css.Class("rpta-subrow"), Span(css.Class("rpta-subhead"), head), src),
		Div(css.Class("rpta-chart-body"),
			Div(css.Class("rpta-yaxis"), Attr("aria-hidden", "true"), Span(maxL), Span(midL), Span(minL)),
			Div(css.Class("rpta-chart-plot"), chart),
		),
		rptaChartLegend(stroke, legend),
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
func rptaSection(id, num, title, zone, sub, ask string, body ui.Node) ui.Node {
	return rptaSectionWithAction(id, num, title, zone, sub, ask, nil, body)
}

func rptaSectionWithAction(id, num, title, zone, sub, ask string, action, body ui.Node) ui.Node {
	if action == nil {
		action = Fragment()
	}
	var askBtn ui.Node = Fragment()
	if ask != "" {
		askBtn = ui.CreateElement(rptaAskBtn, rptaAskProps{Title: title, Observations: ask})
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
			Div(css.Class("rpta-sec-actions"), action, askBtn),
		),
		Div(css.Class("rpta-sec-body"), body),
	)
}

// rptaAskProps configures a section's "ask the assistant" affordance.
type rptaAskProps struct {
	Title        string
	Observations string
}

// rptaAskBtn opens the assistant pre-seeded with this section's observations,
// so the follow-up conversation starts already grounded in the report's
// numbers (the same SeedExplain seam the Explain chips use). Its own component
// so the click hook sits at a stable position per section.
func rptaAskBtn(p rptaAskProps) ui.Node {
	click := ui.UseEvent(Prevent(func() {
		uistate.SeedExplain(uistate.T("rpta.askSeed", p.Title, p.Observations))
		router.Navigate(uistate.RoutePath("/assistant"))
	}))
	return Button(css.Class("rpta-ask"), Type("button"), Attr("data-testid", "rpta-ask"),
		Title(uistate.T("rpta.askTitle", p.Title)), OnClick(click),
		Span(Attr("aria-hidden", "true"), "✦ "), uistate.T("rpta.ask"))
}

// rptaHistRowProps drives one histogram bar in the year-spend charts (§04
// categories, §05 tags): a colored magnitude bar with the label, amount, and a
// judgment-toned delta.
type rptaHistRowProps struct {
	Label     string
	Chip      bool // render the label as a tag chip
	Color     string
	Amount    int64
	Max       int64
	Meta      string // right-edge secondary figure (share %, charge count)
	HasDelta  bool
	DeltaPct  int64
	PriorZero bool
	FmtMinor  func(int64) string
	OnSelect  func() // nil = static row (no drill)
}

// rptaHistRow is one histogram bar. Its own component so a drill hook never
// registers inside the caller's loop.
func rptaHistRow(p rptaHistRowProps) ui.Node {
	click := ui.UseEvent(Prevent(func() {
		if p.OnSelect != nil {
			p.OnSelect()
		}
	}))
	pct := int64(0)
	if p.Max > 0 {
		pct = p.Amount * 100 / p.Max
	}
	if pct < 1 {
		pct = 1
	}
	delta := ""
	deltaCls := "rpta-hist-delta rpta-muted"
	switch {
	case p.PriorZero:
		delta = uistate.T("rpta.newCat")
	case p.HasDelta && p.DeltaPct > 0:
		delta = fmt.Sprintf("▲ %d%%", p.DeltaPct)
		deltaCls = "rpta-hist-delta rpta-tone-down" // spending UP is bad
	case p.HasDelta && p.DeltaPct < 0:
		delta = fmt.Sprintf("▼ %d%%", -p.DeltaPct)
		deltaCls = "rpta-hist-delta rpta-tone-up"
	case p.HasDelta:
		delta = "0%"
	}
	labelCls := "rpta-hist-label"
	if p.Chip {
		labelCls += " rpta-tag-chip"
	}
	inner := []any{
		Span(ClassStr(labelCls), p.Label),
		Div(css.Class("rpta-hist-track"),
			Div(css.Class("rpta-hist-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct), "background": p.Color}))),
		Span(css.Class("rpta-hist-amt", tw.FontDisplay), p.FmtMinor(p.Amount)),
		Span(ClassStr(deltaCls), delta),
		Span(css.Class("rpta-hist-meta", "rpta-muted"), p.Meta),
	}
	if p.OnSelect == nil {
		return Div(append([]any{css.Class("rpta-hist-row")}, inner...)...)
	}
	return Button(append([]any{css.Class("rpta-hist-row", "rpta-hist-btn"), Type("button"),
		Title(uistate.T("reports.drillTitleCat")), OnClick(click)}, inner...)...)
}

// rptaIndex is the sticky jump index: 01-09 with zone dots. Items are buttons
// that scroll their section into view — not raw #hash anchors, which would
// push a fragment URL through the SPA router.
func rptaIndex() ui.Node {
	scrollTo := func(id string) func() {
		return func() {
			doc := js.Global().Get("document")
			if el := doc.Call("getElementById", id); el.Truthy() {
				el.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "start"})
			}
		}
	}
	item := func(id, num, key, zone string) ui.Node {
		// Title carries the section name even when the CSS hides the text label at
		// narrow widths (2026-07-17 audit: the index must stay one compact row).
		return Button(css.Class("rpta-idx-item"), Type("button"), Attr("data-testid", "rpta-idx-item"),
			Title(uistate.T(key)), OnClick(scrollTo(id)),
			Span(ClassStr("rpta-idx-dot rpta-dot-"+zone)),
			Span(css.Class("rpta-idx-num"), num),
			Span(css.Class("rpta-idx-label"), uistate.T(key)),
		)
	}
	return Nav(css.Class("rpta-index"), Attr("data-testid", "rpta-index"), Attr("aria-label", uistate.T("rpta.indexLabel")),
		item("rpta-00", "00", "rpta.idxStand", "neutral"),
		item("rpta-01", "01", "rpta.idxStrong", "up"),
		item("rpta-02", "02", "rpta.idxFlow", "up"),
		item("rpta-03", "03", "rpta.idxMotion", "neutral"),
		item("rpta-04", "04", "rpta.idxCats", "neutral"),
		item("rpta-05", "05", "rpta.idxWhere", "neutral"),
		item("rpta-06", "06", "rpta.idxGoals", "neutral"),
		item("rpta-07", "07", "rpta.idxBudgets", "neutral"),
		item("rpta-08", "08", "rpta.idxWatch", "warn"),
		item("rpta-09", "09", "rpta.idxProblems", "down"),
		item("rpta-10", "10", "rpta.idxPlan", "plan"),
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
	exportCls := "strip-toggle"
	if exportOpenV {
		exportCls += " is-on"
	}
	exportMenu := Div(css.Class("add-wrap"), Attr("id", "rpt-export"),
		Button(ClassStr(exportCls+" "+tw.Fold(tw.Gap2)), Type("button"), Attr("data-testid", "reports-export-toggle"),
			Attr("aria-haspopup", "menu"), Attr("aria-expanded", boolStr(exportOpenV)),
			Title(uistate.T("reports.exportTitle")), OnClick(onToggleExport),
			uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("reports.exportCsv"))),
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
			Button(ClassStr(scopeCls+" "+tw.Fold(tw.Gap2)), Type("button"), Attr("aria-pressed", boolStr(scopeOpenV)),
				Attr("data-testid", "reports-scope-toggle"), Title(uistate.T("reports.scopeHint")),
				OnClick(onToggleScope),
				uiw.Icon(icon.Filter, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(scopeLabel)),
			Button(ClassStr(metricsCls+" "+tw.Fold(tw.Gap2)), Type("button"), Attr("aria-pressed", boolStr(formulasOn)),
				Attr("data-testid", "reports-toggle-formulas"), Title(uistate.T("reports.metricsTitle")),
				OnClick(toggleFormulas),
				uiw.Icon(icon.Calculator, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(metricsLabel)),
			// Saved report views: name the current period + scope, reopen in one click.
			ui.CreateElement(savedReportsControl, struct{}{}),
			// Report-to-action: the report ends in next steps, not just charts.
			ui.CreateElement(reportActionsMenu, reportActionsProps{PeriodLabel: from.Format("Jan 2006")}),
			exportMenu,
			// Month-end snapshots: freeze the current aggregates; reopen them
			// read-only in a flip modal. Inline with its sibling controls (#46).
			ui.CreateElement(reportSnapshotControl, reportSnapshotProps{
				Rows: rows, IncomeRows: incomeRows, Payees: payees, NameOf: nameOf,
				Base: base, PeriodLabel: from.Format("Jan 2006"),
			}),
		),
		If(scopeOpenV, ui.CreateElement(ScopeSelector)),
		// Life-event annotations: the events intersecting this report window.
		ui.CreateElement(reportEventChips, struct{}{}),
	)
}

// rptaMonthDrillProps drives one month-name drill in the year-in-motion table.
type rptaMonthDrillProps struct {
	Label      string
	From, To   string // inclusive ledger-filter dates for the month
	InProgress bool   // the month contains today — label it, don't let it read as a collapse
}

// rptaMonthDrill renders a month name as a drill into the ledger filtered to
// exactly that month's transactions (parity scan: every chart/table element
// routes to its contributing transactions), with an "in progress" tag on the
// current month. Own component so the hooks sit at a stable call-site.
func rptaMonthDrill(props rptaMonthDrillProps) ui.Node {
	nav := router.UseNavigate()
	filterAtom := uistate.UseTxFilter()
	drill := ui.UseEvent(Prevent(func() {
		f := uistate.TxFilter{From: props.From, To: props.To}.Normalize()
		filterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}))
	return Fragment(
		Button(css.Class("rpta-month-drill"), Type("button"),
			Attr("title", uistate.T("rpta.monthDrillTitle", props.Label)),
			OnClick(drill),
			props.Label),
		If(props.InProgress, Span(css.Class("rpta-inprogress"), Attr("data-testid", "rpta-inprogress"),
			uistate.T("rpta.inProgress"))),
	)
}

// rptaCatRowProps drives one row of the full-year category review table.
type rptaCatRowProps struct {
	CategoryID, Name   string
	Dot                string // the category's money-flow color (grey when it isn't a diagram node)
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
			Span(css.Class("rpta-cat-title"),
				If(props.Dot != "", Span(css.Class("rpta-flow-dot", "rpta-cat-dot"), Attr("aria-hidden", "true"), Style(map[string]string{"background": props.Dot}))),
				Span(props.Name)),
			Div(css.Class("share-bar"), Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", widthPct)}))),
		),
		Span(css.Class("rpta-cat-amt", tw.FontDisplay), props.FmtMinor(amt)),
		Span(css.Class("rpta-cat-avg", "rpta-muted"), props.FmtMinor(amt/12)),
		Span(css.Class("rpta-cat-spark"), sparklineSVG(props.Spark, uistate.T("rpta.catSparkAlt"))),
		Span(ClassStr(deltaCls), delta),
		Span(css.Class("rpta-cat-share", "rpta-muted"), fmt.Sprintf("%d%%", share)),
	)
}

// seriesMax/seriesMin/seriesMid give a plotted series' y-axis anchor values —
// the chart normalizes to its own data range, so these ARE the axis. An empty
// series anchors at 0: chart axes are built as eager If(...) arguments, so
// these run even when the guard hides the chart (a card-less report scope
// empties creditSeries — QA CF-01).
func seriesMax(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	m := vs[0]
	for _, v := range vs {
		if v > m {
			m = v
		}
	}
	return m
}

func seriesMin(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	m := vs[0]
	for _, v := range vs {
		if v < m {
			m = v
		}
	}
	return m
}

func seriesMid(vs []float64) float64 { return (seriesMax(vs) + seriesMin(vs)) / 2 }

// anyify converts a node slice to the []any Fragment expects.
func anyify(nodes []ui.Node) []any {
	out := make([]any, len(nodes))
	for i, n := range nodes {
		out[i] = n
	}
	return out
}

// rptaScopeSummary names a report-local scope in plain English for the masthead
// sentence (QA CF-01/UX-03): the selected account names, member names, types,
// and institutions, comma-joined. Unknown IDs fall back to the raw value so a
// stale scope is still visible rather than silently blank.
func rptaScopeSummary(s scope.ReportScope, accounts []domain.Account, members []domain.Member) string {
	var parts []string
	acctName := make(map[string]string, len(accounts))
	for _, a := range accounts {
		acctName[a.ID] = a.Name
	}
	for _, id := range s.AccountIDs {
		if n := acctName[id]; n != "" {
			parts = append(parts, n)
		} else {
			parts = append(parts, id)
		}
	}
	memberName := make(map[string]string, len(members))
	for _, m := range members {
		memberName[m.ID] = m.Name
	}
	for _, id := range s.Owners {
		if n := memberName[id]; n != "" {
			parts = append(parts, n)
		} else {
			parts = append(parts, id)
		}
	}
	for _, t := range s.Types {
		parts = append(parts, humanizeType(string(t)))
	}
	parts = append(parts, s.Institutions...)
	return strings.Join(parts, ", ")
}
