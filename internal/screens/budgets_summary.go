// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/monthclose"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetSummaryPlanResult carries the explicit results for budgetSummaryPlan.
type budgetSummaryPlanResult struct {
	normalized            budgeting.BudgetNormalization
	normalizedBudgetTotal int64
	normalizationOK       bool
	planView              budgetView
	smartSettings         smart.Settings
	wStart                time.Time
	wEnd                  time.Time
	hist                  bool
}

// budgetSummaryPlan resolves monthly normalization and the period/member lens.
func budgetSummaryPlan(v budgetView, app *appstate.App, vw period.Window) budgetSummaryPlanResult {
	// BUD1 exposes checked, explainable cadence conversion. Income on this screen
	// is monthly, so every budget contributes its monthly equivalent before the
	// plan is compared with income. Fall back to the existing view total only if a
	// currency or cadence cannot be converted; the disclosure below says so.
	rates := currency.Rates{Base: v.Base, Rates: app.Settings().FXRates}
	statusBudgets := make([]domain.Budget, 0, len(v.Statuses))
	for _, status := range v.Statuses {
		statusBudgets = append(statusBudgets, status.Budget)
	}
	normalized, normalizeErr := budgeting.NormalizeBudgets(statusBudgets, domain.PeriodMonthly, rates)
	normalizedBudgetTotal := v.TotalLimit
	normalizationOK := normalizeErr == nil && len(v.Incomplete) == 0
	if normalizationOK {
		normalizedBudgetTotal = normalized.Total.Amount
	}
	planView := v
	planView.TotalLimit = normalizedBudgetTotal
	smartSettings := uistate.LoadSmartSettings()
	// UX-05: a wholly-past view window flips the summary into past-tense wording —
	// "Jun 2026 spending", "ended over budget" — because "so far this month" over a
	// closed month reads as a live figure that will still move. Range is half-open,
	// so the window is historical exactly when its end is not after now.
	wStart, wEnd := vw.Range()
	hist := !time.Now().Before(wEnd)
	return budgetSummaryPlanResult{
		normalized: normalized, normalizedBudgetTotal: normalizedBudgetTotal, normalizationOK: normalizationOK, planView: planView,
		smartSettings: smartSettings, wStart: wStart, wEnd: wEnd, hist: hist,
	}
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
			smartTooltipFor(p.smartSettings, "budget-agemoney", uistate.T("budgets.ageMoneyLabel"), why))
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
func budgetSummaryFunding(p budgetSummaryFundingProps) (ui.Node, budgeting.FundingRead, int64) {
	// #64: the plan figure is EXPECTED income — say what actually arrived, so the
	// basis never masquerades as money already received. One quiet sub-line.
	var incomeActual ui.Node = Fragment()
	// C587: the funding read — what the plan was built on, what has arrived, and
	// what is assigned. A plan can be 100% assigned against an expected $10,709.16
	// while only $6,961.00 has come in, and "fully assigned" then looks exactly
	// like "fully funded". The difference is the part that decides whether a
	// payment clears.
	var funding budgeting.FundingRead
	// Scanned ONCE per render and reused: the funding read below and the month-close
	// gate further down both need it, and IncomeByCategory walks every transaction.
	// Computing it twice made a full ledger sweep the price of drawing this tile.
	actualIncome := int64(0)
	if lines, err := reports.IncomeByCategory(p.app.Transactions(), p.wStart, p.wEnd, currency.Rates{Base: p.v.Base, Rates: p.app.Settings().FXRates}); err == nil {
		for _, ln := range lines {
			actualIncome += ln.Amount
		}
	}
	if p.normalizationOK && p.v.BannerIncome > 0 && (p.v.Method == budgeting.MethodZeroBased || p.v.Method == budgeting.MethodSimple) {
		funding = budgeting.FundingRead{
			Expected: p.v.BannerIncome, Received: actualIncome,
			Assigned: p.normalizedBudgetTotal + p.v.SavingsAssigned,
			// The savings/investment assignments are part of the plan but not part
			// of what a bulk budget adjustment can scale, so the reconcile
			// percentage has to be computed against the budgets alone.
			Fixed: p.v.SavingsAssigned,
		}
		key := "budgets.incomeActualSoFar"
		if p.hist {
			key = "budgets.incomeActualEnded"
		}
		// Only speak while income is still outstanding. This line exists to stop the
		// basis masquerading as money already in hand — once everything expected has
		// arrived there is no gap to report, and "$5,900.00 of the $5,900.00 your plan
		// expects" is a tautology that reads as "fully funded" directly beneath a hero
		// saying the plan is over-assigned (Cam, 2026-08-31).
		if actualIncome < p.v.BannerIncome {
			incomeActual = P(css.Class("budget-sub"), Attr("data-testid", "budgets-income-actual"),
				uistate.T(key, fmtMoney(money.New(actualIncome, p.v.Base)), fmtMoney(money.New(p.v.BannerIncome, p.v.Base))))
		}
	}

	return incomeActual, funding, actualIncome
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
	if pool := p.v.BannerIncome + p.v.RolledOver; p.normalizationOK && pool > 0 {
		if ta := budgeting.ToAssign(pool, p.normalizedBudgetTotal+p.v.SavingsAssigned); ta < 0 {
			overAssigned = -ta
		}
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
	if daysLeft := int(time.Until(p.wEnd).Hours() / 24); p.hist || (daysLeft >= 0 && daysLeft <= 5) {
		// Built from figures this render already has — statuses, the over-assignment
		// the rail uses, and the single income scan above — so the gate costs no
		// extra passes over the ledger. Clean() stays the one definition of "nothing
		// to review", shared with the flow the chip opens.
		review := monthclose.Build(p.v.Statuses, nil, p.v.BannerIncome, p.actualIncome, overAssigned, p.pr.BudgetRolloverLeftover)
		if !review.Clean() {
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
	vw                    period.Window
	v                     budgetView
	futurePeriod          bool
	normalizationOK       bool
	overAssigned          int64
	funding               budgeting.FundingRead
}

// budgetSummaryStatus renders the status strip and its primary action.
func budgetSummaryStatus(p budgetSummaryStatusProps) ui.Node {
	statusAmount := p.normalizedBudgetTotal
	statusCopy := uistate.T("bud2.statusFits", p.vw.Label())
	statusState := "fits"
	if len(p.v.Incomplete) > 0 {
		statusCopy = uistate.T("budgets.incomplete")
		statusState = "unknown"
	} else if p.futurePeriod {
		statusCopy = uistate.T("budgets.future.planning",
			fmtMoney(money.New(p.normalizedBudgetTotal, p.v.Base)),
			fmtMoney(money.New(p.v.BannerIncome, p.v.Base)))
	} else if !p.normalizationOK {
		statusCopy = uistate.T("bud2.normalizationUnavailable")
		statusState = "unknown"
	} else if p.v.BannerIncome <= 0 {
		statusCopy = uistate.T("bud2.statusNoIncome", p.vw.Label())
	}
	if !p.futurePeriod && p.overAssigned > 0 {
		statusAmount = p.overAssigned
		statusCopy = uistate.T("bud2.statusOver", p.vw.Label())
		statusState = "over"
	} else if !p.futurePeriod && p.funding.Meaningful() && !p.funding.FullyFunded() && p.funding.Unfunded() > 0 {
		statusAmount = p.funding.Unfunded()
		statusCopy = uistate.T("bud2.statusUnfunded", p.vw.Label())
		statusState = "unfunded"
	}
	var statusAction ui.Node = Fragment()
	if statusState == "over" || statusState == "unfunded" {
		label := uistate.T("bud2.bringPlanDown")
		if statusState == "over" {
			label = uistate.T("bud2.resolveAmount", fmtMoney(money.New(statusAmount, p.v.Base)))
		}
		statusAction = ui.CreateElement(budgetStatusAction, budgetStatusActionProps{
			Label: label, Percent: p.funding.ReduceToFitPct(), TestID: "budgets-status-primary-action",
		})
	}
	statusFigure := fmtMoney(money.New(statusAmount, p.v.Base))
	if len(p.v.Incomplete) > 0 {
		statusFigure = "—"
	}
	statusLine := Div(css.Class("bud2-status-line"), Attr("role", "status"), Attr("data-state", statusState),
		Attr("style", "display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:.75rem;min-width:0"),
		Div(css.Class("bud2-status-copy"), Attr("style", "display:flex;align-items:baseline;flex-wrap:wrap;gap:.2rem .65rem;min-width:0"),
			Span(css.Class("bud2-status-figure"), Attr("data-testid", "budgets-status-figure"),
				statusFigure),
			Span(css.Class("bud2-status-sentence"), Attr("data-testid", "budgets-status-sentence"),
				Attr("style", "max-width:100%;white-space:nowrap;overflow:hidden;text-overflow:ellipsis"), statusCopy),
		),
		statusAction,
	)

	return statusLine
}

// budgetSummaryDetailsProps carries the explicit inputs for budgetSummaryDetails.
type budgetSummaryDetailsProps struct {
	v               budgetView
	normalizationOK bool
	normalized      budgeting.BudgetNormalization
	alloc           ui.Node
	futurePeriod    bool
	funding         budgeting.FundingRead
	hist            bool
	vw              period.Window
	app             *appstate.App
	activeMemberID  string
	pr              prefs.Prefs
	rail            ui.Node
	incomeActual    ui.Node
	lastMonthTag    ui.Node
	attnChip        ui.Node
	attnOver        int
	smartSettings   smart.Settings
	closeOffer      ui.Node
	toAssignChip    ui.Node
	hasAlloc        bool
	basisBtn        ui.Node
	ageChip         ui.Node
}

// budgetSummaryDetails renders the existing details disclosure and action groups.
func budgetSummaryDetails(p budgetSummaryDetailsProps) ui.Node {
	details := Details(css.Class("bud2-status-details"), Attr("data-testid", "budgets-status-details"),
		Summary(uistate.T("bud2.moreDetails")),
		budgetIncompleteNotice(p.v.Incomplete),
		If(p.normalizationOK, budgetNormalizationDetails(p.normalized)),
		If(!p.normalizationOK, P(css.Class("bud2-normalization-error"), Attr("role", "status"), uistate.T("bud2.normalizationUnavailable"))),
		p.alloc,
		ui.CreateElement(transferLeakNotice, transferLeakProps{Txns: p.v.LeakRows, Where: uistate.T("txnleak.whereBudgets")}),
		If(!p.futurePeriod, ui.CreateElement(budgetFundedCallout, budgetFundedProps{
			Funding: p.funding, Base: p.v.Base, Historical: p.hist, PeriodLabel: p.vw.FromLabel(),
			ScalableCount: budgetReconcileCount(p.app, p.activeMemberID, p.v, p.vw, p.pr, p.funding.ReduceToFitPct()),
		})),
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
					p.attnChip,
					// Cover-all re-homes here from the retired budget-attention tile: it
					// belongs beside the chip that names the overages it fixes.
					If(p.attnOver > 0 && p.smartSettings.IsEnabled(coverAllFeatureCode),
						ui.CreateElement(coverAllBannerButton, coverAllButtonProps{})),
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
