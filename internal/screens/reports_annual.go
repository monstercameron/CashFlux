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
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dashboardview"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/doublecount"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/provenance"
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

// Reports is the Annual Review: one long, dense document that reviews and trends
// a full year of transactions, reading from strengths to problem spots to a
// dollar-quantified plan. Its structure IS its message — the "verdict spine"
// tones each section's left edge from healthy green through watch amber to
// problem red, ending on the accent-toned plan. All figures come from the pure
// internal/reports + internal/healthscore cores, so they match the rest of the
// app and remain available as report_* engine variables.
// rptaHashPollMS / rptaHashWindowMS bound the wait for a fragment's target to
// appear: check this often, for at most this long.
const (
	rptaHashPollMS   = 80
	rptaHashWindowMS = 3000
)

// reportNow keeps the report clock explicit for calendar-boundary host tests.
var reportNow = time.Now

// reportModelBuildObserver observes actual memoized model builds in focused tests.
var reportModelBuildObserver func(string)

func reportModelBuilt(model string) {
	if reportModelBuildObserver != nil {
		reportModelBuildObserver(model)
	}
}

func Reports() ui.Node {
	revision := uistate.UseDataRevision().Get()
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	return ui.CreateElement(reportController, reportControllerProps{App: app, Revision: revision})
}

type reportControllerProps struct {
	App      *appstate.App
	Revision int
}

func reportController(p reportControllerProps) ui.Node {
	app := p.App
	// The report reads the /reports-local scope merged INSIDE the app-wide
	// "Viewing as" lens — a filter chosen here never rewrites what other pages
	// show (the parity scan's "report scope leaks globally" defect).
	scopeAtom := uistate.UseReportScope()
	lensScope := uistate.UseActiveScope().Get()

	pr := uistate.UsePrefs().Get()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()
	accounts := app.Accounts()

	sc := scope.Merge(lensScope, scopeAtom.Get())

	// When any scope narrows the report, every section that deliberately stays
	// household-wide (a balance sheet, health factors, goals, budget limits)
	// carries a visible chip saying so — a scoped report must never leave the
	// reader guessing which conclusions followed the filter (2026-07-18
	// nine-page assessment, High: inconsistent scope inside one report).
	scopeActive := !sc.IsAll()
	hhChip := func(titleKey string) ui.Node {
		if !scopeActive {
			return Fragment()
		}
		return Span(css.Class("rpta-hh-chip"), Attr("data-testid", "rpta-hh-chip"),
			Attr("title", uistate.T(titleKey)), uistate.T("rpta.hhChip"))
	}

	// ── The review window (C383). It was a fixed twelve trailing months with
	// automatic year-over-year, which answers one question well and every other
	// question not at all. Both are now settings: the window is a preset resolved
	// against the month the top-bar period lands in, and the comparison is the
	// reader's choice of same-months-last-year, the equally-long window before
	// this one, or nothing. Windows are always WHOLE months — every figure below
	// is bucketed by month, so a partial last bucket would silently understate it.
	w := uistate.UsePeriod().Get()
	_, wEnd := w.Range()
	lastMonth := dateutil.MonthStart(wEnd.AddDate(0, 0, -1))
	rangeCfg := uistate.LoadReportRange()
	primarySpan, cmpSpan, hasCompare := rangeCfg.Windows(lastMonth)
	as, ae := primarySpan.Start, primarySpan.End // annual start (incl) / end (excl)
	months := primarySpan.Months()
	// With the comparison off, the prior window collapses to an empty range. Every
	// prior-period figure downstream then reads zero, which is what "nothing to
	// compare against" should look like — not last year's numbers left on screen.
	ps, pe := as, as
	if hasCompare {
		ps, pe = cmpSpan.Start, cmpSpan.End
	}
	lastMonth = primarySpan.LastMonth()

	// One explicit scope/window model feeds the charts and headline. Its
	// separate household flows keep household position figures consistent with
	// their cash, debt and essentials, even while the activity report is scoped.
	bounds := make([]time.Time, 0, months+1)
	for k := 0; k <= months; k++ {
		bounds = append(bounds, dateutil.AddMonths(as, k))
	}
	review := ui.UseMemo(func() reportview.Series {
		reportModelBuilt("series")
		return reportview.BuildSeries(accounts, txns, sc, bounds, rates)
	}, revKey(app), as.Unix(), ae.Unix(), fmt.Sprintf("%#v", sc))
	scopedTxns := review.Transactions
	scopedAccounts := review.Accounts

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
	// Summary vs. Full mode (progressive disclosure). Summary is the default
	// executive review; Full reveals the long-form document. Local UI state — the
	// report opens on the short summary each visit; the reader steps into the full
	// evidence only when they want it.
	summaryMode := ui.UseState(true)
	onModeSummary := ui.UseEvent(Prevent(func() { summaryMode.Set(true) }))
	onModeFull := ui.UseEvent(Prevent(func() { summaryMode.Set(false) }))
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
	onCFKeyChange := ui.UseEvent(func(v string) { selectedCFKey.Set(v) })
	// UseEvent keeps these wrappers alive and updates their callback cells each
	// render. A stable holder lets section prop equality reuse them even when a
	// renderer's handler payload is itself a non-comparable function value.
	sectionActions := ui.UseMemo(func() *reportSectionActions {
		return &reportSectionActions{ToggleRollup: onToggleRollup, ChangeCFKey: onCFKeyChange}
	}, app)

	persistKey := fmt.Sprintf("annual|%t|%t", yoyMode.Get(), rollupCats.Get())
	ui.UseEffect(func() func() {
		current := uistate.ReportsConfigGet()
		if current.YoY != yoyMode.Get() || current.Rollup != rollupCats.Get() {
			current.YoY, current.Rollup = yoyMode.Get(), rollupCats.Get()
			uistate.SetReportsConfig(current)
		}
		return nil
	}, persistKey)

	// C672: rows that look like unlinked transfers are counted in every figure
	// below. The notice covers the SAME population and the SAME window those
	// figures do — a leak quoted from the account's whole history beside one
	// year's numbers would name money the reader cannot find in them.
	leakNotice := ui.CreateElement(transferLeakNotice, transferLeakProps{
		Txns: scopedTxns, From: as, To: ae, Where: uistate.T("txnleak.whereReport"),
	})

	// ── Year computations (all pure-core calls over [as, ae)). ────────────────
	flow := review.Flow
	cats := app.Categories()
	nowActivity := reportNow()
	confirmedSubs := loadConfirmedSubs()
	activity := ui.UseMemo(func() reportview.Activity {
		reportModelBuilt("activity")
		model, _ := reportview.BuildActivity(reportview.ActivityInput{
			Series: review, Bounds: bounds, PriorStart: ps, PriorEnd: pe, Rates: rates,
			Categories: cats, HouseholdAccounts: accounts, Recurring: app.Recurring(),
			ConfirmedSubs: confirmedSubs, Now: nowActivity,
		})
		return model
	}, revKey(app), as.Unix(), ae.Unix(), ps.Unix(), pe.Unix(), fmt.Sprintf("%#v", sc), nowActivity.Format(dateutil.Layout), fmt.Sprintf("%#v", confirmedSubs))
	categoryReview := ui.UseMemo(func() reportview.CategoryReview {
		reportModelBuilt("categories")
		model, err := reportview.BuildCategoryReview(activity.Spending, activity.CategoryTrends, cats, rollupCats.Get(), months, flow.Expense)
		model.Error = err
		return model
	}, revKey(app), as.Unix(), ae.Unix(), ps.Unix(), pe.Unix(), fmt.Sprintf("%#v", sc), rollupCats.Get())
	adherence := ui.UseMemo(func() reportview.BudgetAdherence {
		reportModelBuilt("adherence")
		model, _ := reportview.BuildBudgetAdherence(app.Budgets(), budgetLedgerForApp(app), bounds, rates)
		return model
	}, revKey(app), as.Unix(), ae.Unix())
	rows := categoryReview.Spending
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

	// Savings reuses the exact cash-flow buckets; it never scans the ledger again.
	monthFlows := review.MonthFlows
	catTrends := activity.CategoryTrends

	// Net worth follows the report's scope like every other figure (QA CF-07:
	// scoped income/spending beside an unscoped $152k net worth read as a broken
	// filter). The scope resolves to account IDs, so filter the accounts and use
	// the already-scoped transactions for the series and the level.
	nwSeries, nwNet, nwChange := activity.NetWorthSeries, activity.NetWorth, activity.NetWorthChange

	// Health: the deterministic score + factors + prioritized steps.
	health := healthscore.Evaluate(liveHealthInputs(app, nowActivity))

	// The year's fee + interest charges ("money that bought nothing", §07).
	costs := activity.Costs

	// Runway (liquid ÷ 6-month burn) for the strengths/problems split. Scoped
	// like net worth — an account-scoped report must not count cash the scope
	// excluded (the vitals section keeps its own household-wide position model).
	runway := activity.Runway

	// Year lists.
	payees, largest, incomeRows := activity.Payees, activity.LargestExpenses, activity.Income
	memberSpend, tagSpend := activity.Members, activity.Tags
	noSpendDays := activity.NoSpendDays
	// QA R3 CF-03: the review's subscription section counted every detected
	// recurring pattern — utilities, pharmacy runs, liability payments — into
	// "N recurring charges ≈ $X/yr". It now applies the same classification the
	// Subscriptions surface uses (essentials, liability payments, and planned
	// recurring flows excluded), so the headline total only carries charges the
	// user could actually cancel.
	liveSubs, rises := activity.Subscriptions, activity.PriceRises
	monthsRed := activity.MonthsNegative
	trims := activity.Trims

	// Goals over the year (household-wide; classify is cheap).
	goalFacts := ui.UseMemo(func() dashboardview.GoalFacts {
		reportModelBuilt("goals")
		return dashboardview.DeriveGoals(app.Goals(), app.Tasks(), rates, nowActivity)
	}, revKey(app), nowActivity.Format(dateutil.Layout))
	gc := goalFacts.Counts

	// ── 00 · Where you stand: today's position, household-wide like net worth
	// (a balance sheet has no scope). Trailing 6-month averages feed the
	// capacity figures; the essential-month basis and debt list feed the rest.
	// All judgment lives in the pure internal/vitals core.
	essBasis := buildSmartInput(app, pr.WeekStartWeekday()).EssentialBasis()
	position := ui.UseMemo(func() reportview.Position {
		reportModelBuilt("position")
		return reportview.BuildPosition(accounts, txns, review.HouseholdFlows, rates, essBasis.EssentialMonthlyMinor())
	}, revKey(app), as.Unix(), ae.Unix(), essBasis.EssentialMonthlyMinor())
	vt := position.Vitals
	var standSec ui.Node
	if position.Error != nil {
		standSec = rptaSectionWithAction("rpta-00", "00", uistate.T("rpta.secStand"), "neutral", uistate.T("rpta.secStandSub"), "", hhChip("rpta.hhChipTitle"),
			P(css.Class("rpta-muted"), Attr("data-testid", "rpta-position-unavailable"), uistate.T("rpta.positionUnavailable")))
	} else {
		standSec = rptaVitalsSection(vt, position.MonthsAveraged, essBasis.FixedMonthlyMinor, essBasis.EssentialSpendMonthlyMinor, fmtMinor, hhChip("rpta.hhChipTitle"))
	}

	// Uncategorized share of spend (data hygiene).
	uncatMinor, uncatPct := categoryReview.UncategorizedMinor, categoryReview.UncategorizedPercent

	// Native balances retain their own currency; the cost column, bars and total
	// all use base-currency interest computed in the shared position model.
	debts, debtInterestTotal := position.Debts, position.AnnualInterestMinor

	// CountUp on the masthead figures.
	heroSig := fmt.Sprintf("%d|%d|%d", flow.Net(), flow.Income, flow.Expense)
	ui.UseEffect(func() func() {
		if fn := js.Global().Get("cashfluxCountUpScan"); fn.Type() == js.TypeFunction {
			fn.Invoke()
		}
		return nil
	}, heroSig)

	// Scroll-spy: as the reader moves through the long full-mode document, mark the
	// section they're in .is-current in the sticky index so orientation never gets
	// lost (fine-detail review: the index helps navigate but weakly orients). An
	// IntersectionObserver over the section anchors fires on band crossings; each
	// fire rescans the anchors' positions and toggles the class directly on the DOM
	// (no state → a scroll never re-renders the document). Full mode only; the
	// summary view has no index.
	// C524: land a deep link on the section it names.
	//
	// The report's numbered sections are not in the DOM when the browser
	// processes a fragment — the page mounts, then fills in — so "#rpta-04"
	// scrolled nowhere and read as a broken link. That is why the "See where it
	// went" links from /budgets and /categories originally shipped pointing at
	// the top of a long document instead of at the categories breakdown.
	//
	// This polls briefly for the target rather than scrolling once: a single
	// timeout has to guess how long the page takes, and guessing short is
	// indistinguishable from the bug. It gives up after rptaHashWindowMS so a
	// stale or misspelled fragment costs nothing, and clears the hash on success
	// so a later in-page navigation to the same section fires again.
	ui.UseEffect(func() func() {
		loc := js.Global().Get("location")
		doc := js.Global().Get("document")
		if !loc.Truthy() || !doc.Truthy() {
			return nil
		}
		id := strings.TrimPrefix(loc.Get("hash").String(), "#")
		if id == "" || !strings.HasPrefix(id, "rpta-") {
			return nil
		}
		if summaryMode.Get() {
			summaryMode.Set(false)
			return nil
		}
		var tick js.Func
		var handle js.Value
		elapsed := 0
		released := false
		stop := func() {
			if !released {
				js.Global().Call("clearInterval", handle)
				tick.Release()
				released = true
			}
		}
		tick = js.FuncOf(func(js.Value, []js.Value) any {
			el := doc.Call("getElementById", id)
			if el.Truthy() {
				el.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "start"})
				js.Global().Get("history").Call("replaceState", js.Null(), "",
					loc.Get("pathname").String()+loc.Get("search").String())
				stop()
				return nil
			}
			if elapsed += rptaHashPollMS; elapsed >= rptaHashWindowMS {
				stop()
			}
			return nil
		})
		handle = js.Global().Call("setInterval", tick, rptaHashPollMS)
		return stop
	}, "rpta-hash|"+boolStr(summaryMode.Get()))

	ui.UseEffect(func() func() {
		if summaryMode.Get() {
			return nil
		}
		doc := js.Global().Get("document")
		ioCtor := js.Global().Get("IntersectionObserver")
		if !doc.Truthy() || !ioCtor.Truthy() {
			return nil
		}
		secIDs := []string{"rpta-00", "rpta-01", "rpta-02", "rpta-03", "rpta-04", "rpta-05", "rpta-06", "rpta-07", "rpta-08", "rpta-09", "rpta-10"}
		var secs []js.Value
		for _, id := range secIDs {
			if el := doc.Call("getElementById", id); el.Truthy() {
				secs = append(secs, el)
			}
		}
		if len(secs) == 0 {
			return nil
		}
		apply := func() {
			// Current = the last section whose heading has scrolled up to (or past)
			// the band just under the sticky index — the greatest top still ≤ band.
			const band = 170.0
			cur := secs[0].Get("id").String()
			best := -1e18
			for _, el := range secs {
				top := el.Call("getBoundingClientRect").Get("top").Float()
				if top <= band && top > best {
					best = top
					cur = el.Get("id").String()
				}
			}
			items := doc.Call("querySelectorAll", `[data-testid="rpta-idx-item"]`)
			for i := 0; i < items.Get("length").Int(); i++ {
				it := items.Call("item", i)
				on := it.Call("getAttribute", "data-section").String() == cur
				it.Get("classList").Call("toggle", "is-current", on)
			}
		}
		cb := js.FuncOf(func(js.Value, []js.Value) any { apply(); return nil })
		io := ioCtor.New(cb, map[string]any{"rootMargin": "-150px 0px -55% 0px", "threshold": 0})
		for _, el := range secs {
			io.Call("observe", el)
		}
		apply()
		return func() {
			io.Call("disconnect")
			cb.Release()
		}
	}, "rpta-scrollspy|"+boolStr(summaryMode.Get()))

	// Empty year (and no scope filter to blame): a single CTA, not a page of zeros.

	windowLine := uistate.T("rpta.window", bounds[0].Format("Jan 2006"), lastMonth.Format("Jan 2006"))

	// ── Masthead: the verdict + four anchor figures. ──────────────────────────
	// #56: each figure is clickable and opens a provenance popover naming the
	// transactions counted, accounts in scope, window, and what was left out —
	// derived by the pure provenance package with the SAME counting rules as
	// the figures themselves.
	verdict, verdictTone := rptaVerdict(health)
	kept := money.New(flow.Net(), base)
	provFlow := provenance.DescribeFlow(scopedTxns, scopedAccounts, as, ae)
	provNW := provenance.DescribeBalance(scopedTxns, scopedAccounts, ae)
	provLines := func(counted string) []string {
		ls := []string{counted, windowLine}
		if provFlow.TransferCount > 0 {
			ls = append(ls, uistate.T("rpta.provTransfers", plural(provFlow.TransferCount, "transfer")))
		}
		if provFlow.ExcludedCount > 0 {
			ls = append(ls, uistate.T("rpta.provExcluded", plural(provFlow.ExcludedCount, "transaction")))
		}
		return ls
	}
	mastFigs := []ui.Node{
		ui.CreateElement(rptaProvFig, rptaProvFigProps{
			ID: "rpta-prov-income", Label: uistate.T("dashboard.income"),
			Value: fmtMoney(money.New(flow.Income, base)),
			Title: uistate.T("dashboard.income"),
			Lines: provLines(uistate.T("rpta.provCounted", plural(provFlow.IncomeCount, "income transaction"), plural(provFlow.AccountCount, "account"))),
		}),
		ui.CreateElement(rptaProvFig, rptaProvFigProps{
			ID: "rpta-prov-spending", Label: uistate.T("dashboard.spending"),
			Value: fmtMoney(money.New(flow.Expense, base)),
			Title: uistate.T("dashboard.spending"),
			Lines: provLines(uistate.T("rpta.provCounted", plural(provFlow.ExpenseCount, "spending transaction"), plural(provFlow.AccountCount, "account"))),
		}),
		ui.CreateElement(rptaProvFig, rptaProvFigProps{
			ID: "rpta-prov-kept", Label: uistate.T("rpta.kept"),
			Value: fmtMoney(kept), Tone: rptaToneFor(kept.Amount),
			Sub:   uistate.T("rpta.keptRate", flow.SavingsRate()),
			Title: uistate.T("rpta.kept"),
			Lines: provLines(uistate.T("rpta.provKept", plural(provFlow.Counted(), "transaction"), plural(provFlow.AccountCount, "account"))),
		}),
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
		mastFigs = append(mastFigs, ui.CreateElement(rptaProvFig, rptaProvFigProps{
			ID: "reports-hero-networth", Label: uistate.T("dashboard.netWorth"),
			Value: fmtMoney(nwNet), Sub: sub, SubTone: tone,
			Title: uistate.T("dashboard.netWorth"),
			Lines: []string{
				uistate.T("rpta.provNWAccounts", plural(provNW.AccountCount, "account")),
				uistate.T("rpta.provNWTxns", plural(provNW.TxnCount, "transaction"), lastMonth.Format("Jan 2006")),
			},
			Extra: If(len(nwInts) >= 2, Div(css.Class("rpta-fig-spark"), Title(uistate.T("rpta.nwSparkTitle")),
				sparklineSVG(nwInts, uistate.T("rpta.nwSparkAlt")),
				Span(css.Class("rpta-fig-spark-cap"), uistate.T("rpta.nwSparkCap"), " · ", rptaSrcLink("nav.netWorth", "/networth")))),
		}))
	}
	// Partial-period honesty (parity scan): when the window's newest month is
	// still in progress, the masthead says so — a 17-day July beside eleven
	// complete months must never read as a spending collapse.
	var partialChip ui.Node = Fragment()
	if nowM := nowActivity; !ae.After(dateutil.AddMonths(dateutil.MonthStart(nowM), 1)) && ae.After(nowM) {
		daysIn := nowM.Day()
		daysTotal := dateutil.AddMonths(dateutil.MonthStart(nowM), 1).AddDate(0, 0, -1).Day()
		partialChip = Span(css.Class("rpta-partial-chip"), Attr("data-testid", "rpta-partial-chip"),
			uistate.T("rpta.partialMonth", nowM.Format("January"), daysIn, daysTotal))
	}
	// QA CF-01/UX-03: when a REPORT-local scope is active, a plain-language
	// sentence stays visible in the masthead even with the Scope panel closed,
	// with a one-click reset. Household-wide figures (health score, Where you
	// stand) are named so a scoped report never implies they follow the filter.
	scopeResetButton := Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "rpta-scope-reset"), OnClick(onResetScope), uistate.T("rpta.scopeReset"))
	var scopeLine ui.Node = Fragment()
	if local := scopeAtom.Get(); !local.IsAll() {
		scopeLine = Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FlexWrap), Attr("data-testid", "rpta-scope-line"),
			Span(css.Class("rpta-partial-chip"), uistate.T("rpta.scopeShowing", rptaScopeSummary(local, accounts, app.Members()), windowLine)),
			scopeResetButton,
		)
	}
	// C383: the window and its comparison are the report's two most consequential
	// parameters, so they sit in the masthead under the title rather than behind a
	// toolbar popover — a reader must never have to hunt for what period they are
	// looking at.
	cmpLabel := ""
	if hasCompare {
		cmpLabel = spanLabel(cmpSpan)
	}
	rangePicker := ui.CreateElement(reportRangePicker, reportRangePickerProps{
		Settings:     rangeCfg,
		PrimaryLabel: spanLabel(primarySpan),
		CompareLabel: cmpLabel,
	})
	masthead := Div(css.Class("rpta-masthead"), Attr("data-testid", "rpt-hero"), Attr("id", "rpta-top"),
		P(css.Class("rpta-eyebrow"), uistate.T("rpta.eyebrow")),
		H1(css.Class("rpta-title", tw.FontDisplay), windowLine),
		rangePicker,
		partialChip,
		scopeLine,
		Div(ClassStr("rpta-verdict rpta-tone-"+verdictTone), Attr("data-testid", "rpta-verdict"),
			Span(css.Class("rpta-verdict-score", tw.FontDisplay), rptaScoreText(health)),
			Span(css.Class("rpta-verdict-line"), verdict),
			rptaSrcLink("nav.health", "/health"),
		),
		Div(css.Class("rpta-figs"), Attr("data-countup", ""), mastFigs),
	)

	sectionContext := reportSectionContext{Revision: revKey(app), ScopeKey: fmt.Sprintf("%#v", sc), Base: base,
		Language: string(uistate.ActiveLanguage()), Accent: chartLineColor(uistate.CurrentAccent()), From: as, To: ae, Prefs: pr}

	// ── Toolbar (tabless): scope, metrics, export. ───────────────────────────
	toolbar := ui.CreateElement(reportToolbar, reportToolbarProps{
		Context: sectionContext, App: app, Scope: sc,
		ScopeOpen: scopeOpen.Get(), FormulasOn: showFormulas.Get(), ExportOpen: exportOpen.Get(),
		OnToggleScope: onToggleScope, ToggleFormulas: toggleFormulas, OnToggleExport: onToggleExport, OnCloseExport: onCloseExport,
		Transactions: scopedTxns, Spending: rows, Income: incomeRows, Payees: payees, Largest: largest,
		Members: memberSpend, Names: catName, Resolution: w.Res, Rates: rates, Monthly: monthFlows,
	})

	// ── The sticky section index (jump links, zone-dotted). ──────────────────
	index := ui.CreateElement(rptaIndex)

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
	if bestIdx := reports.BestSavingsMonth(monthFlows); bestIdx >= 0 {
		wins = append(wins, rptaWin(uistate.T("rpta.winBestMonth", bounds[bestIdx].Format("January"), fmtMinor(monthFlows[bestIdx].Net()))))
	}
	for i, r := range reports.TopSpendingCuts(rows, 5000, 3) {
		_ = i
		wins = append(wins, rptaWin(uistate.T("rpta.winCut", nameOf(r.CategoryID), fmtMinor(r.Prior-r.Amount))))
	}
	if gc.Completed > 0 {
		wins = append(wins, rptaWin(uistate.T("rpta.winGoals", gc.Completed)))
	}
	if runway.Months >= 3 && !runway.Sustainable {
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
		If(len(strongFacts) > 0, P(css.Class("rpta-muted"),
			uistate.T("rpta.factorBasisNote")+If2(scopeActive, " "+uistate.T("rpta.factorScopeNote"), ""))),
		If(len(wins) > 0, Div(css.Class("rpta-wins"), Attr("data-testid", "rpta-wins"), wins)),
		Div(css.Class("rpta-srcrow"), rptaSrcLink("nav.health", "/health")),
	))

	flowSec := ui.CreateElement(reportMoneyFlowSection, reportMoneyFlowProps{Context: sectionContext, Flow: flow, Income: incomeRows, Spending: rows, Names: catName})

	motion := ui.CreateElement(reportMotionSection, reportMotionProps{Context: sectionContext, Series: review, Activity: activity, Bounds: bounds, InProgress: reportview.InProgressMonth(bounds, nowActivity)})

	categories := ui.CreateElement(reportCategoriesSection, reportCategoriesProps{Context: sectionContext, Review: categoryReview, Flow: flow, Income: incomeRows, Tags: tagSpend, Names: catName, Rollup: rollupCats.Get(), Actions: sectionActions})

	whereGoes := ui.CreateElement(reportWhereSection, reportWhereProps{Context: sectionContext, Activity: activity, Names: catName, Members: app.Members()})

	// ── 06 · Goal progress: every financial goal's year, coverage-first. ─────
	nowT := nowActivity
	var goalRows []ui.Node
	archivedReached := 0
	for _, goal := range app.Goals() {
		if goal.Archived && goal.TargetAmount.Amount > 0 {
			archivedReached++
		}
	}
	for _, row := range goalFacts.Rows {
		g, evaluation := row.Goal, row.Evaluation
		if g.TargetAmount.Amount <= 0 {
			continue
		}
		state := evaluation.State
		covPct, savedPct := evaluation.Coverage.CoveragePercent, evaluation.Coverage.SavedPercent
		tone, chipKey := "", "rpta.goalStateCurrent"
		barColor := rptaToneUp
		switch state {
		case goalsvc.StateCompleted:
			tone, chipKey = "up", "rpta.goalStateReached"
		case goalsvc.StateMissed:
			tone, chipKey, barColor = "down", "rpta.goalStateMissed", rptaToneDown
		default:
			tone = "dim"
			if evaluation.Err == nil && !g.TargetDate.IsZero() && g.TargetDate.Before(nowT.AddDate(0, 3, 0)) && covPct < 80 {
				// Due within a quarter and materially short: worth amber.
				tone, barColor = "warn", rptaToneWarn
			}
		}
		deadline := ""
		if !g.TargetDate.IsZero() {
			deadline = uistate.T("rpta.goalBy", g.TargetDate.Format("Jan 2006"))
		}
		goalFigure := uistate.T("goals.coverageUnavailableShort")
		var track ui.Node
		if evaluation.Err == nil {
			goalFigure = uistate.T("rpta.goalBacked", fmtMoney(evaluation.Coverage.Covered), fmtMoney(g.TargetAmount), covPct)
			track = Div(css.Class("rpta-goal-track"), Attr("aria-hidden", "true"),
				Div(css.Class("rpta-goal-cov"), Style(map[string]string{"width": fmt.Sprintf("%d%%", covPct), "background": barColor})),
				Div(css.Class("rpta-goal-saved"), Style(map[string]string{"width": fmt.Sprintf("%d%%", savedPct), "background": barColor})))
		}
		goalRows = append(goalRows, Div(css.Class("rpta-goal-row"), Attr("data-testid", "rpta-goal-row"),
			Div(css.Class("rpta-goal-top"),
				Span(css.Class("rpta-goal-name"), g.Name),
				Span(ClassStr("rpta-goal-chip rpta-chip-"+tone), uistate.T(chipKey)),
				Span(css.Class("rpta-goal-when", "rpta-muted"), deadline),
				Span(css.Class("rpta-goal-fig", tw.FontDisplay), goalFigure),
			),
			track,
		))
	}
	askGoals := uistate.T("rpta.goalsSummary", gc.Completed, gc.Current, gc.Missed)
	goalsSec := Fragment()
	if len(goalRows) > 0 || gc.Completed+gc.Missed > 0 {
		goalsSec = rptaSectionWithAction("rpta-06", "06", uistate.T("rpta.secGoals"), "neutral", uistate.T("rpta.secGoalsSub"), askGoals, hhChip("rpta.hhChipTitle"), Fragment(
			P(css.Class("rpta-muted"), Attr("data-testid", "rpta-goals-summary"), askGoals+If2(archivedReached > 0, " "+uistate.T("rpta.goalsArchived", archivedReached), "")),
			Div(css.Class("rpta-goal-rows"), goalRows),
			A(css.Class("rpta-drill"), Href(uistate.RoutePath("/goals")), Attr("data-testid", "rpta-goals-drill"), uistate.T("rpta.openGoals")),
		))
	}

	// ── 07 · Budget adherence: month-by-month cells per budget. ──────────────

	var budgetRows []ui.Node
	for _, row := range adherence.Rows {
		bg := row.Budget
		name := bg.Name
		if name == "" {
			name = catName[bg.CategoryID]
		}
		if len(row.Months) > 0 {
			var cells []ui.Node
			for _, month := range row.Months {
				state := string(month.State)
				if month.State == reportview.AdherenceWithin {
					state = "under"
				}
				cells = append(cells, Span(ClassStr("rpta-bud-cell rpta-bud-"+state), Title(month.Start.Format("Jan")+": "+fmtMinor(month.Spent)+" / "+fmtMinor(row.Limit))))
			}
			verdictCls, verdictTxt := "rpta-tone-up", uistate.T("rpta.budWithin")
			if row.MonthsOver > 0 {
				verdictCls, verdictTxt = "rpta-tone-down", uistate.T("rpta.budMonthsOver", row.MonthsOver)
			}
			budgetRows = append(budgetRows, Div(css.Class("rpta-bud-row"), Attr("data-testid", "rpta-bud-row"),
				Span(css.Class("rpta-bud-name"), name),
				Div(css.Class("rpta-bud-cells"), cells),
				Span(ClassStr("rpta-bud-verdict "+verdictCls), verdictTxt),
			))
		} else {
			tone := rptaToneUp
			verdictCls, verdictTxt := "rpta-tone-up", uistate.T("rpta.budWithin")
			switch row.AnnualState {
			case reportview.AdherenceOver:
				tone, verdictCls, verdictTxt = rptaToneDown, "rpta-tone-down", uistate.T("rpta.budOverYear", fmtMinor(row.AnnualOver))
			case reportview.AdherenceNear:
				tone, verdictCls, verdictTxt = rptaToneWarn, "rpta-tone-warn", uistate.T("rpta.budOverYear", fmtMinor(row.AnnualOver))
			}
			budgetRows = append(budgetRows, Div(css.Class("rpta-bud-row"), Attr("data-testid", "rpta-bud-row"),
				Span(css.Class("rpta-bud-name"), name+" "+uistate.T("rpta.budAnnualized", string(bg.Period))),
				Div(css.Class("rpta-hist-track"), Div(css.Class("rpta-hist-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", row.AnnualPercent), "background": tone}))),
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
	askBudgets := uistate.T("rpta.budgetsSummary", adherence.Clean, adherence.Counted)
	budgetsSec := Fragment()
	if len(budgetRows) > 0 {
		budgetsSec = rptaSectionWithAction("rpta-07", "07", uistate.T("rpta.secBudgets"), "neutral", uistate.T("rpta.secBudgetsSub"), askBudgets, hhChip("rpta.hhChipBudgetsTitle"), Fragment(
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
	subsAnnual := activity.SubscriptionAnnualMinor
	var subNodes []ui.Node
	for i, s := range liveSubs {
		if i >= 8 {
			break
		}
		subNodes = append(subNodes, Div(css.Class("rpta-watch-row"),
			Span(css.Class("rpta-watch-name"), s.Name),
			Span(css.Class("rpta-muted"), string(s.Cadence)),
			Span(css.Class("rpta-watch-amt"), uistate.T("rpta.perYear", fmtMinor(s.AnnualAmount()))),
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
		if d.AnnualInterestMinor > maxDebtInterest {
			maxDebtInterest = d.AnnualInterestMinor
		}
	}
	var debtRowNodes []ui.Node
	for _, d := range debts {
		minStr := "—"
		if d.Minimum.Amount > 0 {
			minStr = fmtMoney(d.Minimum)
		}
		var dragBar ui.Node = Fragment()
		if maxDebtInterest > 0 {
			dragBar = Div(css.Class("rpta-bar-red"), Attr("aria-hidden", "true"),
				Div(css.Class("rpta-bar-red-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", d.AnnualInterestMinor*100/maxDebtInterest)})))
		}
		debtRowNodes = append(debtRowNodes, Tr(
			Td(css.Class("rpta-td-name"), d.Name),
			Td(css.Class("rpta-td-num"), fmtMoney(d.Balance.Abs())),
			Td(css.Class("rpta-td-num"), fmt.Sprintf("%.1f%%", d.APR)),
			Td(css.Class("rpta-td-num", "rpta-tone-down"), fmtMinor(d.AnnualInterestMinor), dragBar),
			Td(css.Class("rpta-td-num"), minStr),
		))
	}
	var problemBits []ui.Node
	if position.Error != nil {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line", "rpta-muted"), Attr("data-testid", "rpta-debt-unavailable"), uistate.T("rpta.positionUnavailable")))
	}
	if len(weakFacts) > 0 {
		problemBits = append(problemBits, rptaSub(uistate.T("rpta.probFactors"),
			Fragment(hhChip("rpta.hhChipTitle"), rptaSrcLink("nav.health", "/health"))), Div(css.Class("rpta-facts"), weakFacts))
	}
	if monthsRed > 0 {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line"), Attr("data-testid", "rpta-monthsred"),
			Span(css.Class("rpta-tone-down"), fmt.Sprintf("%d", monthsRed)+" "), uistate.T("rpta.monthsRed")))
	}
	if health.NegativeCashFlow {
		problemBits = append(problemBits, P(css.Class("rpta-prob-line", "rpta-tone-down"), uistate.T("rpta.negCashFlow")))
	}
	// WF11: money counted twice. A transfer entered as two separate transactions
	// shows up as both spending and income, so every figure in this report built
	// on either side is wrong — which makes the report itself the right place to
	// say so.
	if dc := doublecount.Find(scopedTxns); dc.Any() {
		// One gets its own wording. "1 movement(s)" is the third plural seam this
		// week, and it makes a reader trust a finding less than it deserves.
		line := uistate.T("rpta.doubleCounted", len(dc.Findings), fmtMinor(dc.MirrorMinor))
		if len(dc.Findings) == 1 {
			line = uistate.T("rpta.doubleCountedOne", fmtMinor(dc.MirrorMinor))
		}
		problemBits = append(problemBits, P(css.Class("rpta-prob-line", "rpta-tone-down"),
			Attr("data-testid", "rpta-doublecount"), line))
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
			rptaSub(head, ui.CreateElement(rptaDrill, rptaDrillProps{Label: uistate.T("reports.viewTransactions"), TestID: "costs-drill", From: as, To: ae, ExpenseOnly: true})),
			Div(css.Class("rows"), Attr("data-testid", "rpta-costs"), costRows))
	}
	if len(debtRowNodes) > 0 {
		problemBits = append(problemBits,
			rptaSub(uistate.T("rpta.probDebt", fmtMinor(debtInterestTotal)), rptaSrcLink("nav.debt", "/debt")),
			Table(css.Class("rpta-table"), Attr("data-testid", "rpta-debt"),
				Thead(Tr(Th(uistate.T("rpta.colDebt")), Th(uistate.T("rpta.colBalance")), Th(uistate.T("rpta.colAPR")), Th(uistate.T("rpta.colYearInterest")), Th(uistate.T("rpta.colMinimum")))),
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
			uistate.T("rpta.planDebt", d.Name, fmt.Sprintf("%.1f%%", d.APR)),
			uistate.T("rpta.planDebtDetail", fmtMinor(int64(d.APR*1000))),
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

	appendix := ui.CreateElement(reportAppendixSection, reportAppendixProps{Context: sectionContext, Accounts: scopedAccounts, Transactions: scopedTxns, Categories: cats, Definitions: cfDefs, SelectedCFKey: selectedCFKey.Get(), Actions: sectionActions, Rates: rates})
	metricsModal := ui.CreateElement(reportMetricsModal, reportMetricsProps{Context: sectionContext, OnClose: func() { showFormulas.Set(false) }})

	// ── Summary (executive) view ──────────────────────────────────────────────
	// The default. Everything here REUSES what the full report already computed —
	// no new analysis. Top 3 wins, top 3 risks, top 3 actions, and the core
	// trends, so the reader gets the verdict in one glance and steps into the full
	// document (the evidence) only when they want it.
	sumWins := wins
	if len(sumWins) > 3 {
		sumWins = sumWins[:3]
	}
	sumActions := planItems
	if len(sumActions) > 3 {
		sumActions = sumActions[:3]
	}
	// Top risks = the weakest health factors by the existing severity ranking
	// (lowest score first), rendered with the same factor-row component §01/§09 use.
	var weakSorted []healthscore.Factor
	for _, f := range health.Factors {
		if f.Weight > 0 && f.Score < 70 {
			weakSorted = append(weakSorted, f)
		}
	}
	sort.SliceStable(weakSorted, func(i, j int) bool { return weakSorted[i].Score < weakSorted[j].Score })
	var sumRisks []ui.Node
	for i, f := range weakSorted {
		if i >= 3 {
			break
		}
		sumRisks = append(sumRisks, rptaFactorRow(f))
	}

	// Core trends (compact): the five headline series the year turns on. The
	// monthly series are lifted straight from the per-month flows / net-worth
	// series the full report already built.
	sumIncSeries := make([]int64, 0, 12)
	sumExpSeries := make([]int64, 0, 12)
	sumNetSeries := make([]int64, 0, 12)
	for i := 0; i < 12 && i < len(monthFlows); i++ {
		sumIncSeries = append(sumIncSeries, monthFlows[i].Income)
		sumExpSeries = append(sumExpSeries, monthFlows[i].Expense)
		sumNetSeries = append(sumNetSeries, monthFlows[i].Net())
	}
	sumNWSeries := make([]int64, 0, len(nwSeries))
	for _, m := range nwSeries {
		sumNWSeries = append(sumNWSeries, m.Amount)
	}
	totalDebtValue := fmtMinor(vt.TotalDebtMinor)
	if position.Error != nil {
		totalDebtValue = uistate.T("calculation.unavailable")
	}
	trendItem := func(label, value, sub, tone string, series []int64) ui.Node {
		valCls := "rpta-sum-trend-val " + tw.Fold(tw.FontDisplay)
		if tone != "" {
			valCls += " rpta-tone-" + tone
		}
		return Div(css.Class("rpta-sum-trend"),
			Span(css.Class("rpta-sum-trend-k"), label),
			Span(ClassStr(valCls), value),
			If(sub != "", Span(css.Class("rpta-sum-trend-sub", "rpta-muted"), sub)),
			If(len(series) >= 2, Div(css.Class("rpta-sum-trend-spark"), sparklineSVG(series, label))),
		)
	}
	nwSub, nwTone := "", ""
	if nwChange != 0 {
		arrow := "▲"
		nwTone = "up"
		if nwChange < 0 {
			arrow, nwTone = "▼", "down"
		}
		// Stamp the window: this delta spans the whole report period, not "this
		// month" — an unlabelled arrow beside net worth reads as month-to-date
		// and disagrees with every surface that means it (C341/C343).
		nwSub = arrow + " " + fmtMinor(absMinor(nwChange)) + " " + uistate.T("rpta.overYear")
	}
	sumTrends := Div(css.Class("rpta-sum-trends"), Attr("data-testid", "rpta-sum-trends"),
		trendItem(uistate.T("dashboard.income"), fmtMinor(flow.Income), "", "", sumIncSeries),
		trendItem(uistate.T("dashboard.spending"), fmtMinor(flow.Expense), "", "", sumExpSeries),
		trendItem(uistate.T("rpta.kept"), fmtMinor(flow.Net()), uistate.T("rpta.keptRate", flow.SavingsRate()), rptaToneFor(flow.Net()), sumNetSeries),
		trendItem(uistate.T("rpta.sumDebt"), totalDebtValue, "", "", nil),
		trendItem(uistate.T("dashboard.netWorth"), fmtMoney(nwNet), nwSub, nwTone, sumNWSeries),
	)
	sumCol := func(title, tone string, items []ui.Node, emptyKey string) ui.Node {
		var body ui.Node = Div(css.Class("rpta-sum-list"), items)
		if len(items) == 0 {
			body = P(css.Class("rpta-muted"), uistate.T(emptyKey))
		}
		return Div(ClassStr("rpta-sum-col rpta-z-"+tone),
			H3(ClassStr("rpta-sum-col-title rpta-tone-"+tone+" "+tw.Fold(tw.FontDisplay)), title),
			body,
		)
	}
	summaryView := Div(css.Class("rpta-summary"), Attr("data-testid", "rpta-summary"),
		Div(css.Class("rpta-sum-grid"),
			// Left column: Strengths + Risks stacked, with the core-trends strip
			// filling the space beneath them so it rises into the first viewport.
			Div(css.Class("rpta-sum-main"),
				sumCol(uistate.T("rpta.sumStrengths"), "up", sumWins, "rpta.sumNoWins"),
				sumCol(uistate.T("rpta.sumRisks"), "down", sumRisks, "rpta.sumNoRisks"),
				Div(css.Class("rpta-sum-trends-wrap"),
					H3(ClassStr("rpta-sum-trends-title "+tw.Fold(tw.FontDisplay)), uistate.T("rpta.sumTrends")),
					sumTrends,
				),
			),
			// Right column: the taller Recommended-actions list.
			sumCol(uistate.T("rpta.sumActions"), "plan", sumActions, "rpta.sumNoActions"),
		),
		Button(css.Class("rpta-drill", "rpta-sum-fulllink"), Type("button"), Attr("data-testid", "rpta-sum-fulllink"),
			OnClick(onModeFull), uistate.T("rpta.sumFullLink")),
	)

	// ── The mode toggle (progressive disclosure): Summary is the default; Full
	// reveals the long-form document as the supporting evidence.
	modeToggle := Div(css.Class("rpta-modes"), Attr("data-testid", "rpta-modes"),
		Attr("role", "group"), Attr("aria-label", uistate.T("rpta.eyebrow")),
		Button(ClassStr("rpta-mode"+If2(summaryMode.Get(), " is-on", "")), Type("button"),
			Attr("data-testid", "rpta-mode-summary"), Attr("aria-pressed", boolStr(summaryMode.Get())),
			Title(uistate.T("rpta.modeSummaryTitle")), OnClick(onModeSummary),
			uistate.T("rpta.modeSummary")),
		Button(ClassStr("rpta-mode"+If2(!summaryMode.Get(), " is-on", "")), Type("button"),
			Attr("data-testid", "rpta-mode-full"), Attr("aria-pressed", boolStr(!summaryMode.Get())),
			Title(uistate.T("rpta.modeFullTitle")), OnClick(onModeFull),
			uistate.T("rpta.modeFull")),
	)

	if flow.Income == 0 && flow.Expense == 0 && sc.IsAll() {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("reports.empty"), CTALabel: uistate.T("reports.addFirst"), Href: "/transactions"})
	}

	return Div(css.Class("rpta"),
		masthead,
		// C672: stated above the report, not inside one section, because the leak
		// distorts every figure below it — spending, income, the flow diagram and
		// the category totals alike.
		// C693: say whether these figures are final before the reader compares
		// them against a finished month.
		provisionalNotice(app, as, ae),
		leakNotice,
		If(activity.Error != nil || categoryReview.Error != nil || review.FlowError != nil,
			P(css.Class("rpta-muted"), Attr("data-testid", "rpta-activity-incomplete"), uistate.T("rpta.activityIncomplete"))),
		If(position.Error != nil, P(css.Class("rpta-muted"), Attr("data-testid", "rpta-position-incomplete"), uistate.T("rpta.positionUnavailable"))),
		If(goalFacts.Unavailable > 0, P(css.Class("rpta-muted"), Attr("role", "status"), Attr("data-testid", "rpta-goals-unavailable"), uistate.T("dashboard.goalStatesUnavailable", goalFacts.Unavailable))),
		modeToggle,
		If(summaryMode.Get(), summaryView),
		If(!summaryMode.Get(), Fragment(
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
			// FP-T1e: the business-and-tax section, given the SAME window as the
			// report above it so the two can never describe different periods.
			// FP-T3b: how much of the period was a commitment rather than a choice,
			// and how far each budget landed from plan.
			ui.CreateElement(spendMixSection, spendMixProps{From: as, To: ae}),
			ui.CreateElement(taxDepthSection, taxSectionProps{From: as, To: ae, Year: as.Year()}),
			appendix,
		)),
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

// rptaScoreText renders "82/100 · Good" (or just the band when no score
// applies). The 0-100 scale travels with the number so the health score never
// reads as a bare, unlabelled figure.
func rptaScoreText(h healthscore.Result) string {
	if h.Band == healthscore.BandNoData {
		return string(h.Band)
	}
	return fmt.Sprintf("%d%s · %s", h.Score, uistate.T("rptaref.scaleOutOf100"), h.Band)
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
	// The savings factor is a trailing 3-month average — the one factor whose
	// name collides with the report's full-period kept rate, so its window
	// travels with the label.
	var window ui.Node = Fragment()
	if f.Key == "savings" {
		window = Span(css.Class("rpta-fact-win"), " · "+uistate.T("rpta.factWindow3mo"))
	}
	return Div(css.Class("rpta-fact"), Attr("data-testid", "rpta-fact-"+f.Key),
		Span(css.Class("rpta-fact-name"), f.Label, window),
		Span(css.Class("rpta-fact-val", tw.FontDisplay), f.Value),
		Div(css.Class("rpta-fact-bar"),
			Div(ClassStr("rpta-fact-fill rpta-fill-"+tone), Style(map[string]string{"width": fmt.Sprintf("%d%%", f.Score)}))),
		// The score carries its 0-100 scale visibly and names its factor for
		// assistive tech, so no bare number ever reads without its label.
		Span(ClassStr("rpta-fact-score rpta-tone-"+tone),
			Attr("aria-label", uistate.T("rptaref.factScoreAria", f.Label, f.Score)),
			Title(uistate.T("rptaref.factScoreAria", f.Label, f.Score)),
			fmt.Sprintf("%d", f.Score),
			Span(css.Class("rpta-fact-scale"), Attr("aria-hidden", "true"), uistate.T("rptaref.scaleOutOf100"))),
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
		// C385: every section that computes something says how, what it leaves
		// out, and — where it judges a figure as healthy or not — what standard it
		// is judging against and where that standard came from. Closed by default;
		// sections with nothing to explain render nothing.
		rptaMethodDrawer(id),
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
		// Every section renders one of these; the accessible name carries the
		// section so "Ask AI" ×11 reads as eleven distinct actions.
		Title(uistate.T("rpta.askTitle", p.Title)), Attr("aria-label", uistate.T("rpta.askTitle", p.Title)),
		OnClick(click),
		Span(Attr("aria-hidden", "true"), "✦ "), uistate.T("rpta.ask"))
}

// rptaHistRowProps drives one histogram bar in the year-spend charts (§04
// categories, §05 tags): a colored magnitude bar with the label, amount, and a
// judgment-toned delta.
type rptaHistRowProps struct {
	Context      reportSectionContext
	Label        string
	Chip         bool // render the label as a tag chip
	Color        string
	Amount       int64
	WidthPercent int64
	Meta         string // right-edge secondary figure (share %, charge count)
	HasDelta     bool
	DeltaPct     int64
	PriorZero    bool
	FmtMinor     func(int64) string
	OnSelect     func() // nil = static row (no drill)
}

// rptaHistRow is one histogram bar. Its own component so a drill hook never
// registers inside the caller's loop.
func rptaHistRow(p rptaHistRowProps) ui.Node {
	click := ui.UseEvent(Prevent(func() {
		if p.OnSelect != nil {
			p.OnSelect()
		}
	}))
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
			Div(css.Class("rpta-hist-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", p.WidthPercent), "background": p.Color}))),
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
			// data-section pairs the item with its section id so the scroll-spy
			// effect (in Reports) can mark the in-view section .is-current.
			Attr("data-section", id),
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
	nameOf func(string) string, base string, res period.Resolution, from time.Time, rates currency.Rates,
	monthFlows []reports.PeriodFlow) ui.Node {

	scopeCount := len(sc.Institutions) + len(sc.Owners) + len(sc.Types) + len(sc.AccountIDs)
	scopeChipText := rptaScopeChipText(sc, app.Accounts(), app.Members())
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
			// C384: the monthly grid is the report's raw shape and the table people
			// most want in a spreadsheet, and it was the largest one with no export —
			// missed because the exports were built per analysis, and this is not one.
			exportItem("reports-export-months", uistate.T("reports.monthlyFlow"), func() {
				downloadBytes(reports.ExportFilename("monthly-cash-flow", res, from), "text/csv", reports.MonthlyFlowCSV(monthFlows, csvAmount))
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
		uiw.ControlStrip(uiw.ControlStripProps{Label: uistate.T("reports.controls"), Class: "rpta-toolbar-row", Controls: []ui.Node{
			Button(ClassStr(scopeCls+" "+tw.Fold(tw.Gap2)), Type("button"), Attr("aria-pressed", boolStr(scopeOpenV)),
				Attr("data-testid", "reports-scope-toggle"), Title(uistate.T("reports.scopeHint")),
				OnClick(onToggleScope),
				uiw.Icon(icon.Filter, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(scopeLabel)),
			// At-a-glance summary of what the scope currently narrows to, so the
			// active filter reads without opening the panel (#8). Hidden when the
			// report is unscoped (the toggle already says "Scope").
			If(scopeChipText != "", Span(css.Class(tw.TextFaint, tw.Text12),
				Attr("data-testid", "reports-scope-active"),
				Attr("aria-label", uistate.T("reports.scopeActiveAria", scopeChipText)),
				scopeChipText)),
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
		}}),
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
	Context                             reportSectionContext
	CategoryID, Name                    string
	Dot                                 string // the category's money-flow color (grey when it isn't a diagram node)
	Amount, Prior                       int64
	HasDelta                            bool
	DeltaPct                            int64
	PriorZero                           bool
	Average, SharePercent, WidthPercent int64
	Spark                               []int64
	FmtMinor                            func(int64) string
	OnDrill                             func(string)
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
	return Div(css.Class("rpta-cat-row"), Attr("data-testid", "reports-cat-row"), Attr("data-category-id", props.CategoryID),
		Button(css.Class("rpta-cat-name"), Type("button"), Attr("data-testid", "reports-cat-drill"),
			Title(uistate.T("reports.drillTitleCat")), OnClick(drill),
			Span(css.Class("rpta-cat-title"),
				If(props.Dot != "", Span(css.Class("rpta-flow-dot", "rpta-cat-dot"), Attr("aria-hidden", "true"), Style(map[string]string{"background": props.Dot}))),
				Span(props.Name)),
			Div(css.Class("share-bar"), Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", props.WidthPercent)}))),
		),
		Span(css.Class("rpta-cat-amt", tw.FontDisplay), props.FmtMinor(amt)),
		Span(css.Class("rpta-cat-avg", "rpta-muted"), props.FmtMinor(props.Average)),
		Span(css.Class("rpta-cat-spark"), sparklineSVG(props.Spark, uistate.T("rpta.catSparkAlt"))),
		Span(ClassStr(deltaCls), delta),
		Span(css.Class("rpta-cat-share", "rpta-muted"), fmt.Sprintf("%d%%", props.SharePercent)),
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
// rptaScopeChipText renders a compact, dot-separated summary of the ACTIVE
// report scope for the toolbar chip (#8) — e.g. "Marcus · Checking · 3 accounts".
// Institutions, owners (by member name), and account types list by name; a long
// account list collapses to a "%d accounts" count so the chip stays one glance.
// Empty when the report is unscoped.
func rptaScopeChipText(s scope.ReportScope, accounts []domain.Account, members []domain.Member) string {
	var parts []string
	parts = append(parts, s.Institutions...)
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
	if n := len(s.AccountIDs); n > 2 {
		parts = append(parts, uistate.T("reports.scopeAccounts", n))
	} else if n > 0 {
		acctName := make(map[string]string, len(accounts))
		for _, a := range accounts {
			acctName[a.ID] = a.Name
		}
		for _, id := range s.AccountIDs {
			if nm := acctName[id]; nm != "" {
				parts = append(parts, nm)
			} else {
				parts = append(parts, id)
			}
		}
	}
	return strings.Join(parts, " · ")
}

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
