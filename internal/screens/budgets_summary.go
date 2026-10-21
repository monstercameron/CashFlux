// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/monthclose"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// budgetSummaryPlanResult carries the explicit results for budgetSummaryPlan.
type budgetSummaryPlanResult struct {
	normalizedBudgetTotal int64
	normalizationOK       bool
	planView              budgetView
	smartSettings         smart.Settings
	wStart                time.Time
	wEnd                  time.Time
	hist                  bool
	excluded              int
	issues                []budgeting.ViewIssue
}

// budgetSummaryPlan resolves monthly normalization and the period/member lens.
func budgetSummaryPlan(v budgetView, app *appstate.App, vw period.Window) budgetSummaryPlanResult {
	// BUD1 exposes checked, explainable cadence conversion. Income on this screen
	// is monthly, so every budget contributes its monthly equivalent before the
	// plan is compared with income. Fall back to the existing view total only if a
	// currency or cadence cannot be converted; the disclosure below says so.
	rates := currency.Rates{Base: v.Base, Rates: app.Settings().FXRates}
	planTotal, planExcluded, planIssues := budgetSummaryWindowPlan(v.Statuses, vw, v.LastMonthMode, rates)
	issues := append([]budgeting.ViewIssue(nil), planIssues...)
	excluded := 0
	for _, status := range v.Statuses {
		if status.Error != nil {
			excluded++
			issues = append(issues, status.Issues...)
			if len(status.Issues) == 0 {
				issues = append(issues, budgeting.ViewIssue{BudgetID: status.Budget.ID, Operation: "evaluate budget", Err: status.Error})
			}
			continue
		}
		issues = append(issues, status.Issues...)
	}
	normalizedBudgetTotal := planTotal
	excluded += planExcluded
	issues = budgeting.DeduplicateViewIssues(issues)
	normalizationOK := planExcluded == 0 && len(v.Incomplete) == 0
	planView := v
	planView.TotalLimit = normalizedBudgetTotal
	spending := budgetview.SummarySpending{Amount: money.New(v.SummaryActual, v.Base), Known: v.SummaryKnown, Complete: v.SummaryComplete, Issues: v.SummaryIssues}
	if !v.SummaryReady {
		spending = budgetSummarySpending(budgetLedgerForApp(app), app.Categories(), v.Statuses, vw, v.LastMonthMode, rates, v.Base)
	}
	issues = budgeting.DeduplicateViewIssues(append(issues, spending.Issues...))
	planView.TotalSpent = spending.Amount.Amount
	planView.SummaryActual, planView.SummaryIssues = spending.Amount.Amount, spending.Issues
	planView.SummaryKnown, planView.SummaryComplete, planView.SummaryReady = spending.Known, spending.Complete, true
	smartSettings := uistate.LoadSmartSettings()
	// UX-05: a wholly-past view window flips the summary into past-tense wording —
	// "Jun 2026 spending", "ended over budget" — because "so far this month" over a
	// closed month reads as a live figure that will still move. Range is half-open,
	// so the window is historical exactly when its end is not after now.
	wStart, wEnd := vw.Range()
	if v.LastMonthMode {
		wStart, wEnd = wStart.AddDate(0, -1, 0), wEnd.AddDate(0, -1, 0)
	}
	hist := !budgetNow().Before(wEnd)
	return budgetSummaryPlanResult{
		normalizedBudgetTotal: normalizedBudgetTotal, normalizationOK: normalizationOK, planView: planView,
		smartSettings: smartSettings, wStart: wStart, wEnd: wEnd, hist: hist, excluded: excluded, issues: issues,
	}
}

// budgetSummaryActualSpend totals posted spending for the selected calendar
// window. Each budget keeps its own category and tag scope, while its cadence
// affects only the plan figure. A future window has no posted spend yet.
func budgetSummaryActualSpend(app *appstate.App, statuses []budgeting.Status, vw period.Window, previous bool, rates currency.Rates, base string) (int64, []budgeting.ViewIssue) {
	if app == nil {
		return 0, []budgeting.ViewIssue{{Operation: "read budget spending", Err: fmt.Errorf("budget app is unavailable")}}
	}
	return budgetSummaryActualSpendNetted(budgetLedgerForApp(app), app.Categories(), statuses, vw, previous, rates, base)
}

func budgetSummaryActualSpendNetted(transactions budgeting.NettedLedger, categories []domain.Category, statuses []budgeting.Status, vw period.Window, previous bool, rates currency.Rates, base string) (int64, []budgeting.ViewIssue) {
	result := budgetSummarySpending(transactions, categories, statuses, vw, previous, rates, base)
	return result.Amount.Amount, result.Issues
}

func budgetSummarySpending(transactions budgeting.NettedLedger, categories []domain.Category, statuses []budgeting.Status, vw period.Window, previous bool, rates currency.Rates, base string) budgetview.SummarySpending {
	start, end := vw.Range()
	if previous {
		start, end = start.AddDate(0, -1, 0), end.AddDate(0, -1, 0)
	}
	return budgetview.PrepareSummarySpending(budgetview.SummarySpendingInput{
		Ledger: transactions, Categories: categories, Statuses: statuses,
		Start: start, End: end, Now: budgetNow(), Rates: rates, Base: base,
	})
}

// budgetSummaryWindowPlan totals each budget's history-aware monthly plan over
// the same calendar window used by the headline spend. Partial months are
// prorated by their covered calendar days.
func budgetSummaryWindowPlan(statuses []budgeting.Status, vw period.Window, previous bool, rates currency.Rates) (int64, int, []budgeting.ViewIssue) {
	start, end := vw.Range()
	if previous {
		start, end = start.AddDate(0, -1, 0), end.AddDate(0, -1, 0)
	}
	if !end.After(start) {
		return 0, 0, []budgeting.ViewIssue{{Operation: "resolve headline budget window", Err: fmt.Errorf("budget window end must follow start")}}
	}
	firstMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	var total int64
	excluded := 0
	var issues []budgeting.ViewIssue
	for monthStart := firstMonth; monthStart.Before(end); monthStart = monthStart.AddDate(0, 1, 0) {
		monthEnd := monthStart.AddDate(0, 1, 0)
		coveredStart := budgetLaterTime(start, monthStart)
		coveredEnd := budgetEarlierTime(end, monthEnd)
		if !coveredEnd.After(coveredStart) {
			continue
		}
		budgets := make([]domain.Budget, 0, len(statuses))
		for _, status := range statuses {
			if status.Error != nil || status.Budget.IsSaving() {
				continue
			}
			budget := status.Budget
			base, err := budgeting.BudgetBaseAt(budget, monthStart)
			if !status.Start.IsZero() && coveredStart.Before(status.End) && coveredEnd.After(status.Start) && status.Base.Currency != "" {
				base = status.Base
			}
			if err != nil {
				excluded++
				issues = append(issues, budgeting.ViewIssue{BudgetID: budget.ID, Operation: "resolve viewed-period budget limit", Err: err})
				continue
			}
			budget.Limit = base
			budgets = append(budgets, budget)
		}
		normalized, normalizeErr := budgeting.NormalizeBudgets(budgets, domain.PeriodMonthly, rates)
		excluded += normalized.Excluded
		issues = append(issues, normalized.Issues...)
		monthPlan := normalized.Total
		coveredDays := budgetCalendarDaysBetween(coveredStart, coveredEnd)
		monthDays := budgetCalendarDaysBetween(monthStart, monthEnd)
		if coveredDays < monthDays {
			monthPlan, normalizeErr = monthPlan.MulDiv(coveredDays, monthDays, money.RoundHalfAwayFromZero)
		}
		if normalizeErr != nil && normalized.Excluded == 0 {
			issues = append(issues, budgeting.ViewIssue{Operation: "scale headline plan to viewed window", Err: normalizeErr})
			excluded++
		}
		total, normalizeErr = money.Add(total, monthPlan.Amount)
		if normalizeErr != nil {
			issues = append(issues, budgeting.ViewIssue{Operation: "sum headline budget plan", Err: normalizeErr})
			return 0, excluded + 1, budgeting.DeduplicateViewIssues(issues)
		}
	}
	return total, excluded, budgeting.DeduplicateViewIssues(issues)
}

func budgetLaterTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func budgetEarlierTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func budgetCalendarDaysBetween(start, end time.Time) int64 {
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int64(endDate.Sub(startDate).Hours() / 24)
}

// budgetSummarySpendingBudgets returns evaluable spending budgets for the
// monthly plan denominator. Saving targets are accounted for separately in
// SavingsAssigned and must not be normalized here as well.
func budgetSummarySpendingBudgets(statuses []budgeting.Status) []domain.Budget {
	budgets := make([]domain.Budget, 0, len(statuses))
	for _, status := range statuses {
		if status.Error != nil || status.Budget.IsSaving() {
			continue
		}
		budgets = append(budgets, status.Budget)
	}
	return budgets
}

// budgetSummaryPoolResult carries the explicit results for budgetSummaryPool.
type budgetSummaryPoolResult struct {
	rangeHint    ui.Node
	lastMonthTag ui.Node
	spendCap     string
	fundFree     int64
	fundHasPool  bool
	zbbHero      bool
}

// budgetSummaryPool builds the summary pool section.
func budgetSummaryPool(vw period.Window, v budgetView, hist bool, normalizedBudgetTotal int64) budgetSummaryPoolResult {
	// The spend figure the graph shows: this month by default; last period's total when
	// the "Last month's spend" overlay is on, so the top graph matches the tiles.
	// The band's geometry (spent-of-limit fill, overage segment, tone thresholds)
	// went with the band. Everything it needed — barSpent, spendLimit, fillPct and
	// the loader classes — described spending against the SUM OF LIMITS, which is
	// the denominator this page stopped using.

	// C130: clarify a custom top-bar range only changes the view window — it doesn't
	// redefine each budget's own period.
	rangeHint := If(!vw.IsSinglePeriod(), P(css.Class("muted"), Attr("data-testid", "budgets-custom-range-hint"),
		uistate.T("budgets.customRangeHint")))

	// When the overlay is on, an accent "LAST MONTH" tag over the spend graph makes clear
	// the figures are last period's, matching the tiles.
	var lastMonthTag ui.Node = Fragment()
	if v.LastMonthMode {
		lastMonthTag = Div(css.Class("budget-lastmonth-tag"), Attr("data-testid", "budgets-summary-lastmonth"), uistate.T("budgets.lastMonthCap"))
	}

	// The caption over the spend graph: "Spending so far this month" while the period
	// is live, "<period> spending" (e.g. "Jun 2026 spending") once it has ended.
	spendCap := uistate.T("budgets.zbbSpendCap")
	if hist {
		spendCap = uistate.T("budgets.histSpendCap", vw.Label())
	}

	// The sinking-fund commitment checks against the method's still-unallocated
	// pool: To-Assign for zero-based, income − budgeted for simple. Envelope has no
	// such pool (money lives in the envelopes), so it never alerts there.
	var fundFree int64
	fundHasPool := false
	switch v.Method {
	case budgeting.MethodZeroBased:
		fundFree = budgeting.ToAssign(v.BannerIncome+v.RolledOver, normalizedBudgetTotal+v.SavingsAssigned)
		fundHasPool = true
	case budgeting.MethodSimple:
		fundFree = v.BannerIncome - normalizedBudgetTotal
		fundHasPool = true
	}
	// zbbHero (2026-08-31): in zero-based the hero figure is To Assign, not "Left".
	// Zero-based asks exactly one question — is every dollar assigned? — and the
	// band was answering a different one: "Left" here is income − spent, which is a
	// full month of income measured against a part-month of spending. It reads
	// large and green early in every month, and it stayed green while the plan was
	// over-assigned, directly above a red "$X more than you earn". Live views only:
	// a closed or last-month view is a spending record, where To Assign has no
	// present tense to report.
	zbbHero := v.Method == budgeting.MethodZeroBased && fundHasPool && !v.LastMonthMode && !hist
	return budgetSummaryPoolResult{
		rangeHint: rangeHint, lastMonthTag: lastMonthTag, spendCap: spendCap, fundFree: fundFree, fundHasPool: fundHasPool,
		zbbHero: zbbHero,
	}
}

// budgetSummaryChipsProps carries the explicit inputs for budgetSummaryChips.
type budgetSummaryChipsProps struct {
	v             budgetView
	hist          bool
	filterAttn    ui.Handler
	fundHasPool   bool
	fundFree      int64
	zbbHero       bool
	spendCap      string
	smartSettings smart.Settings
}

// budgetSummaryChips builds the summary chips section.
func budgetSummaryChips(p budgetSummaryChipsProps) (ui.Node, ui.Node, ui.Node) {
	attentionCount := len(budgetAttentionStatuses(p.v.Statuses))
	var attnChip ui.Node = Fragment()
	if n := attentionCount; n > 0 && !p.hist && !p.v.LastMonthMode {
		txt := uistate.T("bud2.attentionCount", n)
		attnChip = Button(css.Class("budget-hero-attn"), Type("button"),
			Attr("data-testid", "budgets-hero-attn"), Title(uistate.T("budgets.heroAttnTitle")),
			OnClick(p.filterAttn), txt)
	}
	// Zero-based only: the still-unassigned pool as one small chip. Redundant once
	// the hero reports To Assign itself (zbbHero), so it now survives only for the
	// views the hero does not cover — a closed month or the last-month overlay.
	var toAssignChip ui.Node = Fragment()
	if p.v.Method == budgeting.MethodZeroBased && p.fundHasPool && p.fundFree > 0 && !p.zbbHero {
		toAssignChip = Span(css.Class("budget-hero-toassign"), Attr("data-testid", "budgets-toassign-chip"),
			uistate.T("budgets.heroToAssign", fmtMoney(money.New(p.fundFree, p.v.Base))))
	}
	// The hero FIGURE is gone with the band. What it computed — "Left", or To
	// Assign under zero-based — is now stated by the allocation bar's own caption
	// ("155% of your $5,900.00 income · $3,260.20 more than you earn"), in one form
	// that works for a closed month and a live one alike.
	//
	// heroLabel survives only because the attention chip and the month-close gate
	// read the same period wording.
	_ = p.spendCap
	var ageChip ui.Node = Fragment()
	if am := p.v.AgeMoney; am.Ready {
		// A capped figure is the ceiling reporting itself. "365+ days" reads as a
		// measurement, is frozen there whatever you do, and says nothing about whether
		// high is good — so at the cap the chip states the span in words and hands the
		// tooltip the copy that names the direction ("an exceptional buffer").
		figure := strconv.Itoa(am.Days) + " " + uistate.T("budgets.ageMoneyUnit")
		why := uistate.T("budgets.ageMoneyWhy")
		switch {
		case am.Capped:
			figure = uistate.T("budgets.ageMoneyOverYear")
			why = uistate.T("budgets.ageMoneyCapped")
		case am.Days == 1:
			figure = "1 " + uistate.T("budgets.ageMoneyUnitOne")
		}
		ageChip = Span(css.Class("budget-hero-age"), Attr("data-testid", "budgets-agemoney"),
			uistate.T("budgets.ageMoneyLabel")+" · "+figure,
			Span(css.Class("bud3-age-explain"), uistate.T("budgets.ageMoneyOneLine")),
			smartTooltipFor(p.smartSettings, "budget-agemoney", uistate.T("budgets.ageMoneyLabel"), why))
	} else if am.Overflow {
		ageChip = Span(css.Class("budget-hero-age"), Attr("data-testid", "budgets-agemoney-unavailable"),
			uistate.T("budgets.ageMoneyUnavailable"),
			Span(css.Class("bud3-age-explain"), uistate.T("budgets.ageMoneyTotalsTooLarge")))
	}

	return attnChip, toAssignChip, ageChip
}

// budgetSummaryFundingProps carries the explicit inputs for budgetSummaryFunding.
type budgetSummaryFundingProps struct {
	app                   *appstate.App
	wStart                time.Time
	wEnd                  time.Time
	v                     budgetView
	normalizationOK       bool
	normalizedBudgetTotal int64
	hist                  bool
}

// budgetSummaryFunding builds the summary funding section.
type budgetSummaryFundingResult struct {
	incomeActual ui.Node
	prepared     budgetview.FundingPreparation
}

func budgetSummaryFunding(p budgetSummaryFundingProps) budgetSummaryFundingResult {
	// #64: the plan figure is EXPECTED income — say what actually arrived, so the
	// basis never masquerades as money already received. One quiet sub-line.
	var incomeActual ui.Node = Fragment()
	// C587: the funding read — what the plan was built on, what has arrived, and
	// what is assigned. A plan can be 100% assigned against an expected $10,709.16
	// while only $6,961.00 has come in, and "fully assigned" then looks exactly
	// like "fully funded". The difference is the part that decides whether a
	// payment clears.
	var lines []reports.CategorySpend
	var incomeErr error
	var settingsRates map[string]float64
	var transactions []domain.Transaction
	if p.app == nil || p.app.Store() == nil {
		incomeErr = fmt.Errorf("read budget income: app is unavailable")
	} else {
		if err := p.app.Store().CheckReady(); err != nil {
			incomeErr = fmt.Errorf("read budget income: %w", err)
		} else {
			transactions, incomeErr = p.app.TransactionsChecked()
			settings, settingsErr := p.app.SettingsChecked()
			incomeErr = errors.Join(incomeErr, settingsErr)
			settingsRates = settings.FXRates
			if incomeErr == nil {
				lines, incomeErr = reports.IncomeByCategoryChecked(transactions, p.wStart, p.wEnd,
					currency.Rates{Base: p.v.Base, Rates: settingsRates})
			}
		}
	}
	prepared := budgetview.PrepareFunding(budgetview.FundingInput{
		IncomeLines: lines, IncomeError: incomeErr, Method: p.v.Method, Expected: p.v.BannerIncome, RolledOver: p.v.RolledOver,
		Budgeted: p.normalizedBudgetTotal, Fixed: p.v.SavingsAssigned, PlanKnown: p.normalizationOK,
	})
	if prepared.FundingEligible {
		key := "budgets.incomeActualSoFar"
		if p.hist {
			key = "budgets.incomeActualEnded"
		}
		// Only speak while income is still outstanding. This line exists to stop the
		// basis masquerading as money already in hand — once everything expected has
		// arrived there is no gap to report, and "$5,900.00 of the $5,900.00 your plan
		// expects" is a tautology that reads as "fully funded" directly beneath a hero
		// saying the plan is over-assigned (Cam, 2026-08-31).
		if !prepared.IncomeKnown || !prepared.AssignmentsKnown {
			incomeActual = P(css.Class("budget-sub"), Attr("data-testid", "budgets-income-unavailable"),
				uistate.T("budgets.fundingUnavailable"))
		} else if prepared.Received < p.v.BannerIncome {
			incomeActual = P(css.Class("budget-sub"), Attr("data-testid", "budgets-income-actual"),
				uistate.T(key, fmtMoney(money.New(prepared.Received, p.v.Base)), fmtMoney(money.New(p.v.BannerIncome, p.v.Base))))
		}
	}

	return budgetSummaryFundingResult{incomeActual: incomeActual, prepared: prepared}
}

// budgetSummaryAllocationResult carries the explicit results for budgetSummaryAllocation.
type budgetSummaryAllocationResult struct {
	alloc        ui.Node
	hasAlloc     bool
	overAssigned int64
	closeOffer   ui.Node
}

type budgetSummaryAllocationProps struct {
	futurePeriod          bool
	planView              budgetView
	basisBtn              ui.Node
	hist                  bool
	v                     budgetView
	normalizationOK       bool
	normalizedBudgetTotal int64
	wEnd                  time.Time
	actualIncome          int64
	actualIncomeKnown     bool
	overAssigned          int64
	overAssignedKnown     bool
	pr                    prefs.Prefs
}

// budgetSummaryAllocation builds the summary allocation section.
func budgetSummaryAllocation(p budgetSummaryAllocationProps) budgetSummaryAllocationResult {
	// The summary band is gone. It was the shared cross-page "loader" (Spent /
	// Budgeted / Left) that Goals and To-do still use, and on this page it plotted
	// spending against the SUM OF LIMITS — a denominator that moves when you add a
	// budget without spending anything, and that reads "over budget" the moment you
	// spend in a category no budget watches. Two heroes existed because neither it
	// nor To Assign works in both tenses, so the page changed shape as you stepped
	// through months (Cam, 2026-08-31). The allocation bar answers the same question
	// in one form for every period, and carries the spent figures the band held.

	// C529/C530: the allocation read sits directly under the band on EVERY method,
	// so configuring "Budget income" always changes something you can see. It
	// renders nothing without an income figure, in which case basisBtn below stays
	// as the call to action.
	// The spend rail is on in EVERY period now, not only zero-based-live: it is the
	// only place the spent figure survives once the band above is gone.
	var alloc ui.Node = Fragment()
	if !p.futurePeriod {
		alloc = budgetIncomeAllocation(p.planView, p.basisBtn, p.hist, true)
	}
	hasAlloc := p.v.BannerIncome+p.v.RolledOver > 0

	// Zero-based drops the loader band entirely. The band is a spent/budgeted/left
	// reading on its own axis; beneath a bar that plots the same money against
	// income it read as the same chart twice, and the pair could disagree — a calm
	// 79% fill under a red "Over-assigned" (Cam, 2026-08-31). Spending moves onto
	// the allocation bar's axis as a rail (showSpend), so what remains to say here
	// is the one governing number. Every other method keeps the shared band, so the
	// cross-page loader system that Goals and To-do mirror is untouched.
	// ONE graph, every month: "Budgeted against income".
	//
	// There were two heroes and the period decided which you saw — a closed month
	// got the spent/budgeted/left band, the live month got To Assign — so the page
	// changed shape as you stepped through months and the version people preferred
	// was the one they saw least. From the outside that is indistinguishable from a
	// regression, which is exactly how it was reported (Cam, 2026-08-31).
	//
	// Income is the denominator that reads the same in both tenses. "Left" is wrong
	// live (a full month of income against a part month of spending) and "To
	// Assign" is meaningless on a closed month, but "how much of what I earned did
	// I commit" is answerable in either, so one bar serves every period and the
	// spend rail carries the figures the band used to hold.
	// How far the plan runs past the income basis. Feeds BOTH the issues rail below
	// and the month-close gate, so it is computed once, before either.
	// ONE definition, every method, and deliberately the SAME arithmetic the
	// allocation bar uses (see resolveAllocation): pool minus everything assigned.
	//
	// It used to be a switch covering zero-based and simple only, so envelope and
	// flex fell through to zero — and those surfaces then printed "155% of your
	// income · $3,260.20 more than you earn" on the bar while the rail beneath it
	// offered nothing to do about it. Stating a problem and withholding the remedy
	// is worse than not mentioning it (Cam, 2026-08-31).
	//
	// Simple's old formula omitted rollover and savings, so on a household with
	// either it could disagree with the bar directly above it by exactly those
	// amounts. Sharing the bar's arithmetic makes that impossible rather than
	// unlikely.
	overAssigned := int64(0)
	overAssignedKnown := p.overAssignedKnown
	if overAssignedKnown {
		overAssigned = p.overAssigned
	}
	// #64: offer the guided close flow when the period is in its last five days or
	// already ended — the moment the checklist is actually useful. It rides in the
	// action group rather than on a bar of its own: it is one more thing you can do
	// about this month, and a third stacked strip for a single button was most of
	// what made the summary's foot read as clutter (Cam, 2026-08-31).
	// Timing is necessary but not sufficient: the offer also has to have something to
	// review. monthclose.Summary.Clean() ("no overspent budgets and no
	// over-assignment") was written for exactly this and had NO production caller —
	// only tests — so the chip appeared in every period's last five days and opened
	// on "Nothing ended over budget. Nice." / "Your plan fits the expected income."
	// An invitation that reliably leads nowhere teaches people to ignore it
	// (Cam, 2026-08-31).
	var closeOffer ui.Node = Fragment()
	if daysLeft := int(p.wEnd.Sub(budgetNow()).Hours() / 24); (p.hist || (daysLeft >= 0 && daysLeft <= 5)) && p.actualIncomeKnown && overAssignedKnown {
		// Built from figures this render already has — statuses, the over-assignment
		// the rail uses, and the single income scan above — so the gate costs no
		// extra passes over the ledger. Clean() stays the one definition of "nothing
		// to review", shared with the flow the chip opens.
		review := monthclose.Build(p.v.Statuses, nil, p.v.BannerIncome, p.actualIncome, overAssigned, p.pr.BudgetRolloverLeftover)
		if review.Error == nil && !review.Clean() {
			closeOffer = ui.CreateElement(monthCloseOfferChip, struct{}{})
		}
	}
	return budgetSummaryAllocationResult{
		alloc: alloc, hasAlloc: hasAlloc, overAssigned: overAssigned, closeOffer: closeOffer,
	}
}

// budgetSummaryIssuesProps carries the explicit inputs for budgetSummaryIssues.
type budgetSummaryIssuesProps struct {
	app           *appstate.App
	v             budgetView
	smartSettings smart.Settings
	overAssigned  int64
	fundFree      int64
	fundHasPool   bool
	hist          bool
}

// budgetSummaryIssues builds the summary issues section.
func budgetSummaryIssues(p budgetSummaryIssuesProps) ui.Node {
	// UX-05: open follow-up to-dos linked to any budget feed the detail disclosure.
	followUps := 0
	for _, t := range p.app.Tasks() {
		if t.RelatedType == domain.RelatedBudget && t.RelatedID != "" && t.Status != domain.StatusDone {
			followUps++
		}
	}
	rail := ui.CreateElement(budgetIssuesRail, budgetIssuesRailProps{
		View: p.v, SmartSettings: p.smartSettings,
		OverAssigned: p.overAssigned, FundFree: p.fundFree, FundHasPool: p.fundHasPool,
		Hist: p.hist, FollowUps: followUps,
	})

	return rail
}

// budgetSummaryStatusProps carries the explicit inputs for budgetSummaryStatus.
type budgetSummaryStatusProps struct {
	normalizedBudgetTotal int64
	windowSpent           int64
	windowSpentSet        bool
	vw                    period.Window
	v                     budgetView
	futurePeriod          bool
	hist                  bool
	normalizationOK       bool
	overAssigned          int64
	fundingKnown          bool
	fundingEligible       bool
	funding               budgeting.FundingRead
	excluded              int
	issues                []budgeting.ViewIssue
	filterAttn            ui.Handler
	attnCount             int
	attnOver              int
	smartSettings         smart.Settings
}

// budgetSummaryStatus renders the status strip and its primary action.
func budgetSummaryStatus(p budgetSummaryStatusProps) ui.Node {
	statusAmount := p.normalizedBudgetTotal
	statusState := "fits"
	spendingKnown, spendingComplete := true, true
	if p.v.SummaryReady {
		spendingKnown, spendingComplete = p.v.SummaryKnown, p.v.SummaryComplete
	}
	if len(p.v.Incomplete) > 0 {
		statusState = "unknown"
	} else if !p.normalizationOK {
		statusState = "unknown"
	}
	if !spendingComplete {
		statusState = "unknown"
	}
	if statusState == "fits" && p.excluded == 0 && !p.futurePeriod && p.overAssigned > 0 {
		statusAmount = p.overAssigned
		statusState = "over"
	} else if statusState == "fits" && p.excluded == 0 && !p.futurePeriod && p.fundingEligible && p.fundingKnown && p.funding.Meaningful() && !p.funding.FullyFunded() && p.funding.Unfunded() > 0 {
		statusAmount = p.funding.Unfunded()
		statusState = "unfunded"
	}
	var statusAction ui.Node = Fragment()
	if (statusState == "over" || statusState == "unfunded") && p.fundingKnown && p.fundingEligible {
		label := uistate.T("bud2.bringPlanDown")
		if statusState == "over" {
			label = uistate.T("bud2.resolveAmount", fmtMoney(money.New(statusAmount, p.v.Base)))
		}
		statusAction = ui.CreateElement(budgetStatusAction, budgetStatusActionProps{
			Label: label, Percent: p.funding.ReduceToFitPct(), TestID: "budgets-status-primary-action",
		})
	}
	headlineView := p.v
	if p.v.SummaryReady {
		headlineView.TotalSpent = p.v.SummaryActual
	}
	if p.windowSpentSet {
		headlineView.TotalSpent = p.windowSpent
	}
	spent, plan := budgetHeadlineAmounts(headlineView, p.normalizedBudgetTotal)
	statusFigure := fmtMoney(money.New(spent, p.v.Base))
	if !spendingKnown {
		statusFigure = "—"
	}
	statusCopy := uistate.T("budgets.headlineSpentMonthly", fmtMoney(money.New(plan, p.v.Base)))
	if p.hist {
		statusCopy = uistate.T("budgets.headlineSpentMonthlyPast", fmtMoney(money.New(plan, p.v.Base)), p.vw.Label())
	}
	if p.futurePeriod {
		statusCopy = uistate.T("budgets.future.planning",
			fmtMoney(money.New(p.normalizedBudgetTotal, p.v.Base)),
			fmtMoney(money.New(p.v.BannerIncome, p.v.Base)))
	}
	if len(p.v.Incomplete) > 0 || !p.normalizationOK || !spendingComplete {
		statusCopy = uistate.T("budgets.incomplete")
	}
	if p.excluded > 0 {
		statusCopy += " · " + budgetExclusionsLabel(p.excluded)
	}
	fillPct := budgetHeadlinePercent(spent, plan)
	statusLine := Div(css.Class("bud3-headline"), Attr("data-state", statusState),
		Div(css.Class("bud3-headline-copy"), Attr("role", "status"),
			Span(css.Class("bud3-headline-figure"), Attr("data-testid", "budgets-status-figure"), statusFigure),
			Span(css.Class("bud3-headline-secondary"), Attr("data-testid", "budgets-status-sentence"), statusCopy),
			Span(css.Class("bud3-headline-window"), Attr("data-testid", "budgets-status-window"), uistate.T("budgets.headlineWindowLabel", budgetHeadlineWindowLabel(p.vw, p.v.LastMonthMode))),
		),
		If(spendingComplete, Div(css.Class("bud3-headline-meter"), Attr("data-testid", "budgets-status-meter"), Attr("role", "meter"), Attr("aria-label", uistate.T("budgets.headlineMeterLabel")),
			Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-valuenow", strconv.Itoa(fillPct)),
			Div(css.Class("bud3-headline-meter-fill"), Attr("style", "width:"+strconv.Itoa(fillPct)+"%")),
			If(!p.hist && !p.v.LastMonthMode && !p.futurePeriod, Div(css.Class("bud3-headline-pace"), Attr("aria-hidden", "true"), Attr("style", "left:"+strconv.Itoa(budgetPeriodElapsedPct(p.vw, budgetNow()))+"%"))),
		)),
		If(p.attnCount > 0 && !p.hist && !p.v.LastMonthMode,
			Div(css.Class("bud3-headline-attention"),
				Button(css.Class("bud3-attention-link"), Type("button"), Attr("data-testid", "budgets-hero-attn"), OnClick(p.filterAttn), uistate.T("budgets.headlineAttention", p.attnCount)),
				If(p.attnOver > 0 && p.smartSettings.IsEnabled(coverAllFeatureCode), Span(css.Class("bud3-attention-separator"), "·")),
				If(p.attnOver > 0 && p.smartSettings.IsEnabled(coverAllFeatureCode), ui.CreateElement(coverAllBannerButton, coverAllButtonProps{})),
			),
		),
		statusAction,
	)

	return Fragment(statusLine, budgetSummaryDiagnosticLines(p.issues))
}

func budgetSummaryDiagnosticLines(issues []budgeting.ViewIssue) ui.Node {
	var failures, notes []string
	for _, issue := range issues {
		if issue.Informational {
			notes = append(notes, uistate.T("budgets.rolloverStartAssumed"))
			continue
		}
		message := issue.Message
		if message == "" {
			message = uistate.T("bud2.cantCalculate")
		}
		failures = append(failures, message)
	}
	return Fragment(
		If(len(failures) > 0, P(css.Class("budget-sub", tw.TextWarn), Attr("data-testid", "budgets-status-issues"), strings.Join(failures, " · "))),
		If(len(notes) > 0, P(css.Class("budget-sub", tw.TextDim), Attr("data-testid", "budgets-status-notes"), strings.Join(notes, " · "))),
	)
}

func budgetHeadlineWindowLabel(vw period.Window, previous bool) string {
	if previous {
		vw = vw.Shift(-1)
	}
	return uistate.T("budgets.headlineWindowLabel", vw.Label())
}

func budgetHeadlineAmounts(v budgetView, normalizedPlan int64) (int64, int64) {
	return v.TotalSpent, normalizedPlan
}

func budgetHeadlinePercent(spent, plan int64) int {
	return budgetview.HeadlinePercent(spent, plan)
}

func budgetPeriodElapsedPct(vw period.Window, now time.Time) int {
	start, end := vw.Range()
	span := end.Sub(start)
	if span <= 0 || !now.After(start) {
		return 0
	}
	if !now.Before(end) {
		return 100
	}
	return int(now.Sub(start) * 100 / span)
}

func budgetExclusionsLabel(count int) string {
	if count == 1 {
		return uistate.T("bud2.excludesOneBudget")
	}
	return uistate.T("bud2.excludesManyBudgets", count)
}

// budgetSummaryDetailsProps carries the explicit inputs for budgetSummaryDetails.
type budgetSummaryDetailsProps struct {
	v               budgetView
	normalizationOK bool
	alloc           ui.Node
	futurePeriod    bool
	funding         budgeting.FundingRead
	fundingKnown    bool
	fundingEligible bool
	hist            bool
	vw              period.Window
	app             *appstate.App
	activeMemberID  string
	pr              prefs.Prefs
	rail            ui.Node
	incomeActual    ui.Node
	lastMonthTag    ui.Node
	closeOffer      ui.Node
	toAssignChip    ui.Node
	hasAlloc        bool
	basisBtn        ui.Node
	ageChip         ui.Node
}

// budgetSummaryDetails renders the existing details disclosure and action groups.
func budgetSummaryDetails(p budgetSummaryDetailsProps) ui.Node {
	var fundingCallout ui.Node = Fragment()
	if !p.futurePeriod && p.fundingKnown && p.fundingEligible {
		fundingCallout = ui.CreateElement(budgetFundedCallout, budgetFundedProps{
			Funding: p.funding, Base: p.v.Base, Historical: p.hist, PeriodLabel: p.vw.FromLabel(),
			ScalableCount: budgetReconcileCount(p.app, p.activeMemberID, p.v, p.vw, p.pr, p.funding.ReduceToFitPct()),
		})
	}
	details := Details(css.Class("bud2-status-details"), Attr("data-testid", "budgets-status-details"),
		Summary(uistate.T("bud2.moreDetails")),
		budgetIncompleteNotice(p.v.Incomplete),
		If(!p.normalizationOK, P(css.Class("bud2-normalization-error"), Attr("role", "status"), uistate.T("bud2.normalizationUnavailable"))),
		p.alloc,
		ui.CreateElement(transferLeakNotice, transferLeakProps{Txns: p.v.LeakRows, Where: uistate.T("txnleak.whereBudgets")}),
		fundingCallout,
		p.rail,
		Div(css.Class("budget-hero-cap"),
			p.incomeActual,
			// Split by CONSEQUENCE, not by type. The row used to give one treatment to
			// five different things — a list filter, two controls that write to your
			// data, a link off the page, and a frozen statistic — so nothing signalled
			// which ones change anything (Cam, 2026-08-31). Acts come first and keep
			// button weight; the passive group is pushed right and set quiet.
			Div(css.Class("budget-hero-side"),
				Div(css.Class("budget-hero-acts"),
					If(p.v.LastMonthMode, p.lastMonthTag),
					p.closeOffer,
				),
				Div(css.Class("budget-hero-meta"),
					p.toAssignChip,
					// C524: spend-per-category already exists in full on /reports — a
					// magnitude histogram, prior-period deltas, sparklines and a
					// drill-through per bar. Cam asked for it because nothing pointed
					// at it. A link is the fix; a second surface would be a second set
					// of numbers to disagree with the first.
					spendReportLink(),
					// Only a call to action while there is no basis yet — once one is
					// set, the quiet "Change" in the allocation caption owns it, and two
					// buttons for one action is clutter.
					If(!p.hasAlloc, p.basisBtn),
					p.ageChip,
				),
			),
		),
	)

	return details
}
