// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// growthPctLabel formats an expected-annual-return in basis points as a compact
// percent for the goal card's projection caption (700 → "7%", 750 → "7.5%").
func growthPctLabel(bips int) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", float64(bips)/100), "0"), ".")
	return s + "%"
}

// GoalRow renders one goal's progress toward its target, with contribute and
// (inline) edit actions. All hooks are declared unconditionally so the edit
// toggle never reorders them.
func GoalRow(props goalRowProps) ui.Node {
	g := props.Goal

	del := ui.UseEvent(Prevent(func() {
		// Capture which row holds focus before the row is removed, so focus can
		// be restored to the next row after the delete (§6.7).
		captureRowDeleteFocus(".goal-list", "[data-testid^='goal-row-']")
		props.OnDelete(g.ID)
	}))
	doArchive := ui.UseEvent(Prevent(func() {
		if props.OnArchive != nil {
			props.OnArchive(g.ID, true)
		}
	}))
	doUnarchive := ui.UseEvent(Prevent(func() {
		if props.OnArchive != nil {
			props.OnArchive(g.ID, false)
		}
	}))
	drillAcct := ui.UseEvent(Prevent(func() {
		if props.OnDrillAccount != nil {
			props.OnDrillAccount(g.AccountID)
		}
	}))
	pr := uistate.UsePrefs().Get()
	// Edit + Contribute open the shell-root flip modal (GoalEditHost) — the goal card
	// lives under transformed tile ancestors, so an in-card modal would render off-centre.
	openEdit := ui.UseEvent(Prevent(func() {
		uistate.SetGoalEdit(uistate.GoalEdit{ID: g.ID, Mode: uistate.GoalEditModeEdit})
	}))
	openContribute := ui.UseEvent(Prevent(func() {
		uistate.SetGoalEdit(uistate.GoalEdit{ID: g.ID, Mode: uistate.GoalEditModeContribute})
	}))
	// Virtual allocation: earmark account balances toward the goal (no transaction posted).
	openAllocate := ui.UseEvent(Prevent(func() {
		uistate.SetGoalEdit(uistate.GoalEdit{ID: g.ID, Mode: uistate.GoalEditModeAllocate})
	}))
	// Pause/snooze the goal for N months, with an honest ETA-cost preview (GL7).
	openPause := ui.UseEvent(Prevent(func() {
		uistate.SetGoalEdit(uistate.GoalEdit{ID: g.ID, Mode: uistate.GoalEditModePause})
	}))
	doMarkReviewed := ui.UseEvent(Prevent(func() { markGoalReviewed(g.ID) }))
	doUndoContribution := ui.UseEvent(Prevent(func() {
		if props.OnUndoContribution != nil {
			props.OnUndoContribution(g.ID)
		}
	}))
	doResetGoal := ui.UseEvent(Prevent(func() {
		if props.OnResetGoal != nil {
			props.OnResetGoal(g.ID)
		}
	}))
	// Milestone / habit direct actions (non-financial kinds). Declared unconditionally
	// so hook order is stable; only wired into the footer for the relevant kind.
	markDone := ui.UseEvent(Prevent(func() { setMilestoneDone(g.ID, true) }))
	markUndone := ui.UseEvent(Prevent(func() { setMilestoneDone(g.ID, false) }))
	checkIn := ui.UseEvent(Prevent(func() { addHabitCheckIn(g.ID) }))
	// Add a linked to-do ("step") to this goal (drives checklist progress).
	openAddStep := ui.UseEvent(Prevent(func() { addGoalStep(g.ID) }))
	// toggleTodo flips a linked to-do's done state (plain closure, passed to each
	// GoalTodoItem child which owns its own click hook — no On* in a loop here).
	toggleTodo := func(taskID string) { toggleGoalTodo(taskID) }

	// GL redesign (Task 2): the contribution planner is opt-in per card — hidden by
	// default and revealed by the "Plan contribution" disclosure chip below the figures.
	// The open state lives in a stable top-of-component hook so it never reorders.
	planOpen := ui.UseState(false)
	togglePlan := ui.UseEvent(Prevent(func() { planOpen.Set(!planOpen.Get()) }))
	// UX-06: the card defaults to a COMPACT state — name + status, progress, the three
	// decision figures (to go, needed/mo, landing date), ONE primary action, and a
	// Details control. Everything else (secondary chips, quick-fund, legend, meta,
	// trajectory, notes, steps, planner, secondary/destructive actions) lives in the
	// expanded state; Delete stays in the kebab there per the standing directive.
	cardExpanded := ui.UseState(false)
	toggleExpand := ui.UseEvent(Prevent(func() { cardExpanded.Set(!cardExpanded.Get()) }))

	// Inline target editing (parity with the budgets limit): click the target figure in
	// the bar → a compact number input (Enter/✓ saves, ✕ cancels), undoable. props.Goal
	// is the STORED goal (no evaluation-copy trap here), but the save still resolves the
	// budget from the store so seed and write share one source of truth.
	targetEditing := ui.UseState(false)
	targetDraft := ui.UseState("")
	targetDec := currency.Decimals(g.TargetAmount.Currency)
	startTargetEdit := ui.UseEvent(Prevent(func() {
		seed := g.TargetAmount
		if app := appstate.Default; app != nil {
			for _, gg := range app.Goals() {
				if gg.ID == g.ID {
					seed = gg.TargetAmount
					break
				}
			}
		}
		targetDraft.Set(money.FormatMinor(seed.Amount, targetDec))
		targetEditing.Set(true)
	}))
	cancelTargetEdit := ui.UseEvent(Prevent(func() { targetEditing.Set(false) }))
	// G8: the fast funding gesture — one click earmarks the whole remaining gap (or as
	// much as the best account's free cash allows) without opening the modal. The
	// deliberate desktop answer to "drag money onto the goal": a click is faster, and
	// keyboard/screen-reader users get the same gesture. Recomputed inside the handler
	// so a stale render can't over-reserve; undoable like every money-state change.
	quickFund := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		acctID, acctName, amt := bestQuickEarmark(app, g.ID)
		if acctID == "" || amt <= 0 {
			return
		}
		for _, gg := range app.Goals() {
			if gg.ID != g.ID {
				continue
			}
			cur := gg.TargetAmount.Currency
			allocs := append([]domain.GoalAllocation(nil), gg.Allocations...)
			merged := false
			for i, al := range allocs {
				if al.AccountID == acctID {
					allocs[i].Amount = money.New(al.Amount.Amount+amt, cur)
					merged = true
					break
				}
			}
			if !merged {
				allocs = append(allocs, domain.GoalAllocation{AccountID: acctID, Amount: money.New(amt, cur)})
			}
			if err := app.SetGoalAllocations(gg.ID, allocs); err == nil {
				uistate.PostUndoable(uistate.T("goals.quickFundToast", fmtMoney(money.New(amt, cur)), acctName))
				uistate.BumpDataRevision()
			}
			return
		}
	}))
	onTargetDraft := ui.UseEvent(func(v string) { targetDraft.Set(v) })
	saveTargetEdit := ui.UseEvent(Prevent(func() {
		amt, perr := money.ParseMinor(strings.TrimSpace(targetDraft.Get()), targetDec)
		if perr != nil || amt <= 0 {
			targetEditing.Set(false)
			return
		}
		if app := appstate.Default; app != nil {
			for _, gg := range app.Goals() {
				if gg.ID != g.ID {
					continue
				}
				if gg.TargetAmount.Amount != amt {
					gg.TargetAmount = money.New(amt, gg.TargetAmount.Currency)
					if err := app.PutGoal(gg); err == nil {
						uistate.PostUndoable(uistate.T("goals.targetChangedToast", fmtMoney(gg.TargetAmount)))
						uistate.BumpDataRevision()
					}
				}
				break
			}
		}
		targetEditing.Set(false)
	}))

	now := time.Now()
	kind := g.EffectiveKind()
	financial := kind.IsFinancial()

	// Kind-aware progress: money for financial, linked-to-do count for checklist,
	// a binary done for milestone, check-ins/streak for habit.
	prog := goalsvc.EvaluateProgress(g, props.Tasks, now)
	pct := prog.Percent
	// First-class earmarks: "reached" now counts committed savings PLUS reserved
	// earmarks (CoverageMinor), so grounding a goal in real set-aside money completes
	// it — not only money that has moved. For non-financial kinds Reached == the
	// kind's own completion, so this is a no-op there. coveragePct drives the loader
	// and headline; savedPct (pct) is the moved portion drawn as the solid segment,
	// with the earmarked gap between them shown as a second, hatched tone.
	complete := goalsvc.Reached(g, props.Tasks, now)
	coveragePct := pct
	if financial {
		coveragePct = goalsvc.CoveragePercent(g)
	}
	pace := goalsvc.ClassifyPace(g, now) // money-based; used only on the financial path

	redirect := ui.UseEvent(Prevent(func() {
		if props.OnRedirect != nil {
			props.OnRedirect()
		}
	}))

	// Card tint + progress-bar fill class, per kind.
	var cardState, barClass string
	switch {
	case complete:
		cardState, barClass = "is-done", "done"
	case financial:
		cardState, barClass = goalCardStateClass(pace, complete), paceBarClass(pace)
	case !g.TargetDate.IsZero() && g.TargetDate.Before(now):
		cardState, barClass = "is-overdue", "overdue"
	default:
		cardState, barClass = "is-ontrack", ""
	}

	// Figures inside the loader bar: a left label + a right percent, per kind.
	pctFig := Span(css.Class("budget-pct"), fmt.Sprintf("%d%%", pct))
	var mainFig ui.Node
	switch kind {
	case domain.GoalKindChecklist:
		if prog.Total == 0 {
			mainFig = Span(css.Class("budget-amount"), uistate.T("goals.stepsNone"))
		} else {
			mainFig = Span(css.Class("budget-amount"), uistate.T("goals.stepsFmt", prog.Done, prog.Total))
		}
	case domain.GoalKindMilestone:
		lbl := uistate.T("goals.milestoneOpen")
		if complete {
			lbl = uistate.T("goals.milestoneDone")
		}
		mainFig = Span(css.Class("budget-amount"), lbl)
		pctFig = Fragment() // 0/100 adds nothing next to the label
	case domain.GoalKindHabit:
		mainFig = Span(css.Class("budget-amount"), uistate.T("goals.checkInsFmt", prog.Done, prog.Total))
	default: // financial — lead with the BACKED figure: coverage (saved + earmarked)
		// against the target, so reserved money reads as real progress at a glance.
		// The target figure is a direct affordance: click to edit in place (mirrors the
		// budgets limit; reuses its editor styling).
		cov := money.New(goalsvc.CoverageMinor(g), g.TargetAmount.Currency)
		switch {
		case targetEditing.Get():
			mainFig = Form(css.Class("budget-amount", "budget-limit-editform"), OnSubmit(saveTargetEdit),
				Span(css.Class("budget-spent"), fmtMoney(cov)), Span(" / "),
				Input(css.Class("field", "budget-limit-input"), Attr("autofocus", ""), Type("number"),
					Attr("data-testid", "goal-target-input-"+g.ID), Attr("aria-label", uistate.T("goals.targetLabel")),
					Value(targetDraft.Get()), Step("0.01"), Attr("min", "0.01"), OnInput(onTargetDraft)),
				Button(css.Class("btn btn-sm", "budget-limit-save"), Type("submit"), Attr("data-testid", "goal-target-save-"+g.ID),
					Attr("aria-label", uistate.T("action.save")), Title(uistate.T("action.save")), uiw.Icon(icon.Check, css.Class(tw.W35, tw.H35))),
				Button(css.Class("btn btn-sm", "budget-limit-cancel"), Type("button"), Attr("data-testid", "goal-target-cancel-"+g.ID),
					Attr("aria-label", uistate.T("action.cancel")), Title(uistate.T("action.cancel")), OnClick(cancelTargetEdit), uiw.Icon(icon.Close, css.Class(tw.W35, tw.H35))),
			)
		case !g.Archived:
			mainFig = Span(css.Class("budget-amount"),
				Span(css.Class("budget-spent"), fmtMoney(cov)), Span(" / "),
				Button(css.Class("budget-limit-btn"), Type("button"), Attr("data-testid", "goal-target-btn-"+g.ID),
					Title(uistate.T("goals.targetEditTitle")), Attr("aria-label", uistate.T("goals.targetEditTitle")),
					OnClick(startTargetEdit), fmtMoney(g.TargetAmount)))
		default: // archived: a plain, non-interactive figure
			mainFig = Span(css.Class("budget-amount"), Span(css.Class("budget-spent"), fmtMoney(cov)), " / "+fmtMoney(g.TargetAmount))
		}
		pctFig = Span(css.Class("budget-pct"), fmt.Sprintf("%d%%", coveragePct))
	}

	// Header chips. Financial: pace badge + sinking-fund set-aside. Habit: a streak
	// chip. Others: none. (The monthly-needed rate lives ONLY in the figures grid —
	// one fact, one place.)
	var paceBadgeNode, fundChip, streakChip ui.Node = Fragment(), Fragment(), Fragment()
	if financial {
		paceBadgeNode = paceBadge(pace)
		if g.IsSinkingFund && props.FundSetAside > 0 {
			fundAmt := money.New(props.FundSetAside, g.CurrentAmount.Currency)
			fundChip = Span(ClassStr("pace-badge pace-rate"), Attr("data-testid", "fund-setaside-"+g.ID),
				uistate.T("goals.monthlySetAside", fmtMoney(fundAmt)))
		}
	} else if kind == domain.GoalKindHabit && prog.Streak > 0 {
		streakChip = Span(ClassStr("pace-badge pace-rate"), Attr("data-testid", "goal-streak-"+g.ID),
			uistate.T("goals.streakFmt", prog.Streak))
	}
	// Paused chip (GL7): a quiet, non-alarming note that the goal is intentionally
	// paused until a date — a chosen state, not a scold. Shown for any kind.
	var pausedChip ui.Node = Fragment()
	if g.IsPaused(now) {
		pausedChip = Span(ClassStr("pace-badge pace-paused"), Attr("data-testid", "goal-paused-chip-"+g.ID),
			uistate.T("goals.pausedChip", pr.FormatDate(g.PausedUntil)))
	}
	// Review-due chip (any kind, when the review cadence has elapsed) + a linked-budgets
	// count chip (financial goals that feed one or more budgets).
	var reviewChip, budgetChip ui.Node = Fragment(), Fragment()
	if goalsvc.ReviewDue(g, now) {
		reviewChip = Span(ClassStr("pace-badge pace-review"), Attr("data-testid", "goal-review-due-"+g.ID),
			uistate.T("goals.reviewDue"))
	}
	if len(g.BudgetIDs) > 0 {
		budgetChip = Span(ClassStr("pace-badge pace-rate"), Attr("data-testid", "goal-budgets-"+g.ID),
			uistate.T("goals.linkedBudgetChip", len(g.BudgetIDs)))
	}
	// G8 quick-fund chip (computed at render; the click handler re-derives the same
	// figures so a stale card can never over-reserve).
	var quickFundChip ui.Node = Fragment()
	if financial && !g.Archived && !complete {
		if app := appstate.Default; app != nil {
			if _, qName, qAmt := bestQuickEarmark(app, g.ID); qAmt > 0 {
				// A "Suggested" eyebrow marks this unambiguously as a proposed action —
				// without it, "Set aside $X from Y" can read as a completed statement.
				quickFundChip = Div(css.Class("goal-quickfund"),
					Button(css.Class("goal-quickfund-btn"), Type("button"),
						Attr("data-testid", "goal-quickfund-"+g.ID),
						Title(uistate.T("goals.quickFundTitle")), OnClick(quickFund),
						Span(css.Class("goal-quickfund-eyebrow"), uistate.T("goals.quickFundEyebrow")),
						uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
						Span(uistate.T("goals.quickFundChip", fmtMoney(money.New(qAmt, g.TargetAmount.Currency)), qName)),
					))
			}
		}
	}

	// Sub-section under the bar. Financial keeps its rich actionable copy (remaining,
	// deadline, monthly, over-fund, what-next, linked account, fund category). Non-
	// financial goals get a compact deadline / complete line. The earmark legend is
	// hoisted to card scope because it renders directly under the bar it explains,
	// not in the meta strip.
	var earmarkLegend ui.Node = Fragment()
	var subSection ui.Node
	// UX-06: the compact card's figure strip — to go, needed/mo, landing date only.
	var compactFigs ui.Node = Fragment()
	if financial {
		overfund, _ := goalsvc.Overfund(g)
		// Figures grid (redesign): the key numbers as scannable stat cells instead of a
		// run-on "$X to go · by date · save $X/mo" sentence. Only cells with real data
		// appear, so the grid stays honest and adapts per goal. A completed goal drops the
		// figures entirely — its loader already reads 100% and the meta strip says what's next.
		// "To go" is measured against COVERAGE (target − saved − earmarked), so a goal
		// grounded in earmarks reads as closer to done — consistent with the backed
		// headline. rem (savings-only) still feeds over-fund/what-next below.
		toGoMinor := g.TargetAmount.Amount - goalsvc.CoverageMinor(g)
		if toGoMinor < 0 {
			toGoMinor = 0
		}
		toGo := money.New(toGoMinor, g.TargetAmount.Currency)
		var figs []ui.Node
		if !complete {
			figs = append(figs, goalFig(uistate.T("goalsredesign.figToGo"), fmtMoney(toGo)))
			// QA L5: one bare "Monthly" cell served TWO different figures — the
			// mathematically required pace and the user's own saved plan — so a
			// $140 plan silently displayed as the $121.43 requirement. The pace is
			// now labeled "Needed / mo", and a user-set plan shows as its own
			// "Your plan / mo" cell beside it, never replaced.
			if per, ok, _ := goalsvc.MonthlyNeeded(g, now); ok {
				figs = append(figs, goalFig(uistate.T("goals.figMonthlyNeeded"), fmtMoney(per)))
			}
			if g.MonthlyContribution.Amount > 0 {
				figs = append(figs, goalFig(uistate.T("goals.figMonthlyPlan"), fmtMoney(g.MonthlyContribution)))
			}
			if !g.TargetDate.IsZero() {
				figs = append(figs, goalFig(uistate.T("goalsredesign.figTarget"), pr.FormatDate(g.TargetDate)))
			}
			// Growth-adjusted projection: for a goal with an expected annual return, show
			// when compounding + contributions actually reach the target — often sooner
			// than the flat pace implies. On-device, using the user's own assumed rate.
			if g.ExpectedReturnBips > 0 {
				monthlyMinor := int64(0)
				if per, ok, _ := goalsvc.MonthlyNeeded(g, now); ok {
					monthlyMinor = per.Amount
				} else if g.MonthlyContribution.Amount > 0 {
					monthlyMinor = g.MonthlyContribution.Amount
				}
				proj := goalsvc.ProjectWithGrowth(goalsvc.CoverageMinor(g), g.TargetAmount.Amount, monthlyMinor, g.ExpectedReturnBips, now)
				if proj.Reachable {
					figs = append(figs, goalFig(
						uistate.T("goalsredesign.figProjected", growthPctLabel(g.ExpectedReturnBips)),
						pr.FormatDate(proj.Date)))
				}
			}
		}
		var figsNode ui.Node = Fragment()
		if len(figs) > 0 {
			figsNode = Div(css.Class("goal-figs"), Attr("data-testid", "goal-figs-"+g.ID), figs)
		}
		// The compact strip repeats only the three decision figures.
		if !complete {
			cf := []ui.Node{goalFig(uistate.T("goalsredesign.figToGo"), fmtMoney(toGo))}
			if per, ok, _ := goalsvc.MonthlyNeeded(g, now); ok {
				cf = append(cf, goalFig(uistate.T("goals.figMonthlyNeeded"), fmtMoney(per)))
			}
			if !g.TargetDate.IsZero() {
				cf = append(cf, goalFig(uistate.T("goalsredesign.figTarget"), pr.FormatDate(g.TargetDate)))
			} else if g.ExpectedReturnBips > 0 {
				monthlyMinor := int64(0)
				if per, ok, _ := goalsvc.MonthlyNeeded(g, now); ok {
					monthlyMinor = per.Amount
				} else if g.MonthlyContribution.Amount > 0 {
					monthlyMinor = g.MonthlyContribution.Amount
				}
				if proj := goalsvc.ProjectWithGrowth(goalsvc.CoverageMinor(g), g.TargetAmount.Amount, monthlyMinor, g.ExpectedReturnBips, now); proj.Reachable {
					cf = append(cf, goalFig(uistate.T("goalsredesign.figProjected", growthPctLabel(g.ExpectedReturnBips)), pr.FormatDate(proj.Date)))
				}
			}
			compactFigs = Div(css.Class("goal-figs"), Attr("data-testid", "goal-compact-figs-"+g.ID), cf)
		}
		var whatNext ui.Node = Fragment()
		if complete && !g.Archived {
			whatNext = Div(css.Class("budget-sub"), Attr("data-testid", "goal-whatnext-"+g.ID),
				Span(uistate.T("goals.whatNext")+" "),
				Button(css.Class("budget-drill"), Type("button"), Attr("aria-label", uistate.T("goals.whatNextAction")),
					Attr("data-testid", "goal-redirect-"+g.ID), OnClick(redirect), uistate.T("goals.whatNextAction")))
		}
		var overfundNote ui.Node = Fragment()
		if overfund.IsPositive() {
			overfundNote = Span(css.Class("budget-sub"), Attr("data-testid", "goal-overfund-"+g.ID),
				Style(map[string]string{"color": "var(--up)"}),
				uistate.T("goals.overfundFmt", goalsvc.RawPercent(g), uistate.T("goals.overTarget", fmtMoney(overfund))))
		}
		linkedName := accountName(props.Accounts, g.AccountID)
		var linkedLine ui.Node = Fragment()
		if linkedName != "" {
			// When several accounts are linked, name the primary (drillable) and note the rest,
			// rather than the singular "linked to X" implying it's the only one.
			var moreNode ui.Node = Fragment()
			if extra := len(g.LinkedAccountIDs()) - 1; extra > 0 {
				moreNode = Span(css.Class("goal-sub-dim"), uistate.T("goals.linkedMore", extra))
			}
			linkedLine = Span(css.Class("budget-sub"),
				Button(css.Class("budget-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drillAcct),
					Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
					uistate.T("goals.linkedSuffix", linkedName)),
				moreNode)
		}
		var catLine ui.Node = Fragment()
		if g.IsSinkingFund && props.LinkedCategoryName != "" {
			catLine = Span(css.Class("budget-sub"), Attr("data-testid", "fund-category-"+g.ID),
				uistate.T("goals.fundLinkedCategory", props.LinkedCategoryName))
		}
		// Virtual allocation payoff: a legend that explains the bar's two fills with the
		// actual figures — a solid swatch for money saved (moved), a hatched swatch for
		// money set aside (reserved in place), and the one visible statement that set-aside
		// money never moves. Suppressed on a complete goal (already funded). If the earmark
		// no longer fits the account balance (spent down since), flag it in a warning tone
		// rather than claiming false coverage.
		if am := g.AllocatedMinor(); am > 0 && !complete {
			earmarkMoney := fmtMoney(money.New(am, g.TargetAmount.Currency))
			if props.EarmarkOverbooked {
				earmarkLegend = Span(css.Class("budget-sub", tw.TextWarn), Attr("data-testid", "goal-earmarked-"+g.ID),
					uistate.T("goals.earmarkOverbooked", earmarkMoney))
			} else {
				savedMoney := fmtMoney(money.New(g.CurrentAmount.Amount, g.TargetAmount.Currency))
				earmarkLegend = Div(css.Class("goal-legend"), Attr("data-testid", "goal-earmarked-"+g.ID),
					Span(css.Class("goal-legend-item"),
						Span(css.Class("goal-legend-swatch is-saved"), Attr("aria-hidden", "true")),
						uistate.T("goals.legendSaved", savedMoney)),
					Span(css.Class("goal-legend-item"),
						Span(css.Class("goal-legend-swatch is-earmark"), Attr("aria-hidden", "true")),
						uistate.T("goals.legendSetAside", earmarkMoney)),
					Span(css.Class("goal-legend-note"), uistate.T("goals.legendNote")),
				)
			}
		}
		subSection = Fragment(
			figsNode,
			Div(css.Class("goal-meta"),
				overfundNote, whatNext, linkedLine, catLine,
				goalInterestEtaLine(g, props.Accounts, now),
			),
			// The savings-pace rail lives INSIDE the card's metadata block (right below the
			// figures/notes it summarises), not as a detached section underneath the card.
			goalTrajectoryNode(g, now),
		)
	} else {
		var line string
		if complete {
			line = uistate.T("goals.complete")
		} else if !g.TargetDate.IsZero() {
			line = uistate.T("goals.bySuffix", pr.FormatDate(g.TargetDate))
			if len(line) > 3 && line[:3] == " · " {
				line = line[3:] // drop the leading separator when it stands alone
			}
		}
		if line != "" {
			subSection = Div(css.Class("budget-sub goal-sub"), Span(line))
		} else {
			subSection = Fragment()
		}
	}
	// TX11: show the running round-up jar on the target goal's row ("$6.37 in
	// round-ups"), only when this is the round-up target goal and something has
	// accrued this cadence period.
	if jar, ok := goalRoundUpJar(appstate.Default, g.ID); ok {
		subSection = Fragment(subSection, Div(css.Class("goal-sub"), jar))
	}

	// Primary footer action, per kind: Contribute (financial), Mark done / Reopen
	// (milestone), Check in (habit). Checklist has no direct action — its steps come
	// from linked to-dos (managed on the to-do page).
	var primaryAction ui.Node = Fragment()
	if !g.Archived {
		switch kind {
		case domain.GoalKindFinancial:
			if complete {
				// Reached (saved + earmarked cover the target): the useful next step is to
				// archive it (or keep topping up). Lead with Archive; keep Set aside as a
				// quiet secondary for over-funding.
				primaryAction = Fragment(
					Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-archive-primary-"+g.ID), Attr("aria-label", uistate.T("goals.archive")), Title(uistate.T("goals.archiveReachedTitle")), OnClick(doArchive), uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.archive"))),
					Button(css.Class("btn goal-action-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-setaside-"+g.ID), Attr("aria-label", uistate.T("goals.setAsideTitle")), Title(uistate.T("goals.setAsideTitle")), OnClick(openAllocate), uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.setAside"))),
				)
			} else {
				// Earmark-first: "Set aside" (reserve real balances, no money moves) is the
				// primary planning gesture; logging money already saved is the quiet secondary.
				primaryAction = Fragment(
					Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-setaside-"+g.ID), Attr("aria-label", uistate.T("goals.setAsideTitle")), Title(uistate.T("goals.setAsideTitle")), OnClick(openAllocate), uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.setAside"))),
					Button(css.Class("btn goal-action-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-contribute-"+g.ID), Attr("aria-label", uistate.T("goals.logSavedTitle")), Title(uistate.T("goals.logSavedTitle")), OnClick(openContribute), uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.logSaved"))),
				)
			}
		case domain.GoalKindMilestone:
			if complete {
				primaryAction = Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-reopen-"+g.ID), OnClick(markUndone), uiw.Icon(icon.Refresh, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.markUndone")))
			} else {
				primaryAction = Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-markdone-"+g.ID), OnClick(markDone), uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.markDone")))
			}
		case domain.GoalKindHabit:
			primaryAction = Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-checkin-"+g.ID), OnClick(checkIn), uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.checkIn")))
		}
	}

	// Edit lives at the top of the ⋯ menu (it opens the shell-root flip editor). It used
	// to be an inline footer button; moving it declutters the footer to just the primary
	// kind action + the menu.
	var editItem ui.Node = Fragment()
	if !g.Archived {
		editItem = Button(css.Class("btn btn-tool"), Type("button"),
			Attr("data-testid", "goal-edit-btn-"+g.ID), Attr("aria-label", uistate.T("goals.editTitle")),
			OnClick(openEdit), uistate.T("action.edit"))
	}

	// Mark reviewed (any goal with a review cadence): clears the "review due" flag.
	// Hidden on archived goals. (Set aside is NOT duplicated here — it's the card's
	// primary action; one action, one entry point, one name.)
	var reviewItem ui.Node = Fragment()
	if g.ReviewCadence != "" && !g.Archived {
		reviewItem = Button(css.Class("btn btn-tool"), Type("button"),
			Attr("data-testid", "goal-review-btn-"+g.ID), OnClick(doMarkReviewed), uistate.T("goals.markReviewed"))
	}

	// Pause / Resume (GL7): pause an active, not-yet-complete goal for N months
	// (opens the cost-preview form), or resume one that's paused. A chosen state,
	// framed as a choice — hidden on archived and complete goals.
	var pauseItem ui.Node = Fragment()
	if !g.Archived && !complete {
		if g.IsPaused(now) {
			pauseItem = Button(css.Class("btn btn-tool"), Type("button"),
				Attr("data-testid", "goal-resume-btn-"+g.ID), OnClick(openPause), uistate.T("goals.resumeAction"))
		} else {
			pauseItem = Button(css.Class("btn btn-tool"), Type("button"),
				Attr("data-testid", "goal-pause-btn-"+g.ID), OnClick(openPause), uistate.T("goals.pauseAction"))
		}
	}

	// Archive / Unarchive as inline tool buttons. A complete FINANCIAL goal already
	// leads with Archive as its primary action, so the tool-row duplicate is
	// suppressed there — one action, one entry point.
	var archiveItem ui.Node = Fragment()
	if g.Archived {
		archiveItem = Button(css.Class("btn btn-tool"), Type("button"),
			Attr("data-testid", "goal-unarchive-"+g.ID), OnClick(doUnarchive), uistate.T("goals.unarchive"))
	} else if complete && kind != domain.GoalKindFinancial {
		archiveItem = Button(css.Class("btn btn-tool"), Type("button"),
			Attr("data-testid", "goal-archive-"+g.ID), OnClick(doArchive), uistate.T("goals.archive"))
	}

	// Contribution controls in the ⋯ menu (financial goals only): undo the most
	// recent contribution when there's a logged one, and reset saved progress to
	// zero when the goal holds anything. Both are hidden on archived goals.
	var undoItem, resetItem ui.Node = Fragment(), Fragment()
	if financial && !g.Archived {
		if len(g.Contributions) > 0 {
			undoItem = Button(css.Class("btn btn-tool"), Type("button"),
				Attr("data-testid", "goal-undo-contrib-"+g.ID), OnClick(doUndoContribution), uistate.T("goals.undoContribution"))
		}
		if g.CurrentAmount.Amount > 0 {
			resetItem = Button(css.Class("btn btn-tool"), Type("button"),
				Attr("data-testid", "goal-reset-"+g.ID), OnClick(doResetGoal), uistate.T("goals.resetToZero"))
		}
	}

	// Linked to-dos ("steps"): the tasks joined to this goal. Shown whenever the goal has
	// any, and always for a checklist goal (where the steps ARE the progress) so it offers
	// the add-step CTA even when empty. Each item toggles done live (updating progress);
	// checklist goals get a "+ add step" affordance.
	linked := goalsvc.LinkedTasks(props.Tasks, g.ID)
	stepsDone, stepsTotal := goalsvc.TaskCounts(props.Tasks, g.ID)
	var todosSection ui.Node = Fragment()
	if len(linked) > 0 || kind == domain.GoalKindChecklist {
		// Show only the top 3 steps so a long list never dictates the card's height;
		// open steps come first (most actionable), then completed. A "+N more" line notes
		// the remainder — the full list stays manageable on the to-do page.
		display := append([]domain.Task(nil), linked...)
		sort.SliceStable(display, func(i, j int) bool {
			di := display[i].Status == domain.StatusDone
			dj := display[j].Status == domain.StatusDone
			return !di && dj // open steps before completed ones
		})
		const maxSteps = 3
		hidden := 0
		if len(display) > maxSteps {
			hidden = len(display) - maxSteps
			display = display[:maxSteps]
		}
		var items []ui.Node
		for _, lt := range display {
			items = append(items, ui.CreateElement(GoalTodoItem, goalTodoProps{Task: lt, OnToggle: toggleTodo}))
		}
		var body ui.Node
		if len(items) == 0 {
			body = P(css.Class("goal-todos-empty"), uistate.T("goals.noSteps"))
		} else {
			body = Div(css.Class("goal-todos-list"), items)
		}
		var moreLine ui.Node = Fragment()
		if hidden > 0 {
			moreLine = P(css.Class("goal-todos-more"), Attr("data-testid", "goal-todos-more-"+g.ID), uistate.T("goals.stepsMore", hidden))
		}
		var addBtn ui.Node = Fragment()
		if !g.Archived && kind == domain.GoalKindChecklist {
			addBtn = Button(css.Class("goal-todo-add"), Type("button"), Attr("data-testid", "goal-addstep-"+g.ID), OnClick(openAddStep),
				uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("goals.addStep")))
		}
		todosSection = Div(css.Class("goal-todos"), Attr("data-testid", "goal-todos-"+g.ID),
			Div(css.Class("goal-todos-head"),
				Span(css.Class("goal-todos-title"), uistate.T("goals.todosHead")),
				Span(css.Class("goal-todos-count"), fmt.Sprintf("%d/%d", stepsDone, stepsTotal)),
			),
			body,
			moreLine,
			addBtn,
		)
	}

	// GL redesign (Task 2): the contribution planner is opt-in. Show its disclosure chip
	// only when the slider itself would render (financial, active, with a sensible range),
	// mirroring GoalContribSlider's own guard — so a chip never leads to an empty planner.
	_, _, _, sliderOK := goalsvc.SliderRange(g, now)
	// A REACHED goal has nothing left to plan — the planner chip hides with the figures
	// (completion propagates to every sub-feature, not just the tint).
	showPlanner := financial && !g.Archived && !complete && sliderOK
	planLabel := uistate.T("goalsredesign.planShow")
	if planOpen.Get() {
		planLabel = uistate.T("goalsredesign.planHide")
	}

	// A free-text note on the goal — a clamped preview that opens the goal editor on click
	// (where the full note is read/edited), mirroring the budget card.
	var goalNotesNode ui.Node = Fragment()
	if notes := strings.TrimSpace(g.Notes); notes != "" {
		goalNotesNode = Button(ClassStr("acct-notes goal-notes"), Type("button"), Attr("data-testid", "goal-notes-"+g.ID),
			Attr("aria-label", uistate.T("goals.notesLabel")), Title(uistate.T("goals.editTitle")), OnClick(openEdit),
			uiw.Icon(icon.FileText, css.Class("acct-notes-icon", tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("acct-notes-text"), notes))
	}

	// The card's "loader": a progress bar with a kind-appropriate label + percent
	// inside it. Built once — both the compact and expanded states render it.
	// Progress semantics live on the childless saved-fill bar (role="img" with
	// the percent in the label): the loader wrapper contains focusable figure
	// buttons, and a labeled role wrapping interactive children fails axe
	// nested-interactive (#67).
	loaderNode := Div(css.Class("goal-card-loader"),
		// Two-tone fill: a hatched earmark band runs out to coverage BEHIND the solid
		// saved segment, so the gap between "saved" and "backed" is legible at a glance.
		// Rendered first → painted under the saved fill (same stacking context, later
		// DOM node wins), leaving only the pct..coverage slice showing through.
		If(financial && coveragePct > pct,
			Div(ClassStr("bar-fill bar-earmark"), Attr("data-testid", "goal-bar-earmark-"+g.ID), Attr("style", barFillStyle(coveragePct)))),
		Div(ClassStr("bar-fill "+barClass),
			Attr("role", "img"),
			Attr("aria-label", uistate.T("goals.progressLabel")+": "+strconv.Itoa(coveragePct)+"%"),
			Attr("style", barFillStyle(pct))),
		Div(css.Class("goal-card-loader-figs"),
			mainFig,
			pctFig,
		),
	)

	// UX-06: compact default. One context-sensitive primary action per kind, plus the
	// Details control that reveals the full card.
	if !cardExpanded.Get() {
		var compactPrimary ui.Node = Fragment()
		if !g.Archived {
			switch kind {
			case domain.GoalKindFinancial:
				if complete {
					compactPrimary = Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-archive-primary-"+g.ID), Attr("aria-label", uistate.T("goals.archive")), Title(uistate.T("goals.archiveReachedTitle")), OnClick(doArchive), uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.archive")))
				} else {
					compactPrimary = Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-setaside-"+g.ID), Attr("aria-label", uistate.T("goals.setAsideTitle")), Title(uistate.T("goals.setAsideTitle")), OnClick(openAllocate), uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.setAside")))
				}
			case domain.GoalKindMilestone:
				if complete {
					compactPrimary = Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-reopen-"+g.ID), OnClick(markUndone), uiw.Icon(icon.Refresh, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.markUndone")))
				} else {
					compactPrimary = Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-markdone-"+g.ID), OnClick(markDone), uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.markDone")))
				}
			case domain.GoalKindHabit:
				compactPrimary = Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-checkin-"+g.ID), OnClick(checkIn), uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.checkIn")))
			}
		}
		var compactSub ui.Node
		if financial {
			compactSub = compactFigs
		} else {
			compactSub = subSection
		}
		return Div(ClassStr("goal-card is-compact "+cardState),
			Attr("data-testid", "goal-row-"+g.ID), Attr("data-kind", string(kind)),
			Div(css.Class("goal-card-head"),
				Span(css.Class("goal-card-title"), g.Name),
				paceBadgeNode,
				pausedChip,
			),
			loaderNode,
			compactSub,
			Div(css.Class("goal-card-actions"),
				compactPrimary,
				Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap1), Type("button"),
					Attr("data-testid", "goal-expand-"+g.ID), Attr("aria-expanded", "false"),
					Attr("aria-label", uistate.T("goals.expandTitle")), Title(uistate.T("goals.expandTitle")), OnClick(toggleExpand),
					Span(uistate.T("goals.expand")),
					uiw.Icon(icon.ChevronDown, css.Class(tw.ShrinkO, tw.W35, tw.H35))),
			),
		)
	}

	return Div(ClassStr("goal-card "+cardState),
		Attr("data-testid", "goal-row-"+g.ID), Attr("data-kind", string(kind)),
		// GL6: the goal's vision image as a small banner atop the card (when attached).
		goalImageBanner(appstate.Default, g),
		// Header: the goal name on its own line, with kind-appropriate chips.
		Div(css.Class("goal-card-head"),
			Span(css.Class("goal-card-title"), g.Name),
			// Explicit planning priority, when set (edit form → Priority).
			If(g.Priority > 0, Span(css.Class("goal-chip"), Attr("data-testid", "goal-priority-"+g.ID),
				Title(uistate.T("goals.priorityChipTitle")), uistate.T(goalPriorityKey(g.Priority)))),
			paceBadgeNode,
			pausedChip,
			fundChip,
			streakChip,
			budgetChip,
			reviewChip,
		),
		loaderNode,
		// The bar's legend sits directly beneath it: what's saved (solid) vs set
		// aside (hatched), and the reassurance that reserved money hasn't moved.
		earmarkLegend,
		// G8: the one-click funding gesture — "Set aside $X from <account>" earmarks the
		// remaining gap (capped at the account's free cash) without opening the modal.
		quickFundChip,
		subSection,
		goalNotesNode,
		// GL3: one-tap emergency-fund sizing from the derived essential month.
		ui.CreateElement(GoalEmergencySizer, goalEmergencyProps{App: appstate.Default, Goal: g}),
		// GL4 + Task 2: the contribution slider is opt-in — a disclosure chip reveals it
		// inline, so the default card stays clean. Hidden entirely for goals with no plan.
		If(showPlanner,
			Button(css.Class("goal-plan-toggle"), Type("button"),
				Attr("data-testid", "goal-plan-toggle-"+g.ID),
				Attr("aria-expanded", ariaBool(planOpen.Get())),
				Attr("aria-controls", "goal-plan-"+g.ID),
				OnClick(togglePlan),
				uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(planLabel),
			),
		),
		If(showPlanner && planOpen.Get(),
			ui.CreateElement(GoalContribSlider, goalSliderProps{App: appstate.Default, Goal: g})),
		// GL5: shared-goal per-member pledge split bar (renders only when pledged).
		ui.CreateElement(GoalPledgeBar, goalPledgeProps{App: appstate.Default, Goal: g, Members: props.Members}),
		todosSection,
		// Footer: the primary kind action, the everyday actions surfaced INLINE as labelled
		// tool buttons (Edit / Mark reviewed / Pause / Undo / Reset / Archive), and a slim ⋯
		// menu that now holds only the destructive Delete (standing directive: delete stays
		// in the kebab, never an always-visible row button).
		Div(css.Class("goal-card-actions"),
			primaryAction,
			editItem,
			reviewItem,
			pauseItem,
			undoItem,
			resetItem,
			archiveItem,
			Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap1), Type("button"),
				Attr("data-testid", "goal-collapse-"+g.ID), Attr("aria-expanded", "true"),
				Attr("aria-label", uistate.T("goals.collapseTitle")), Title(uistate.T("goals.collapseTitle")), OnClick(toggleExpand),
				Span(uistate.T("goals.collapse")),
				uiw.Icon(icon.ChevronUp, css.Class(tw.ShrinkO, tw.W35, tw.H35))),
			uiw.KebabMenu(uiw.KebabMenuProps{
				ID:           "goal-menu-" + g.ID,
				AriaLabel:    uistate.T("goals.moreActions"),
				ToggleTestID: "goal-menu-btn-" + g.ID,
				Items: []ui.Node{
					Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "goal-delete-btn-"+g.ID), Attr("aria-label", uistate.T("goals.deleteTitle")), Title(uistate.T("goals.deleteTitle")), OnClick(del), uistate.T("action.delete")),
				},
			}),
		),
	)
}

// goalTodoProps drives a single linked-to-do row on a goal card.
type goalTodoProps struct {
	Task     domain.Task
	OnToggle func(string)
}

// GoalTodoItem renders one linked to-do (a goal "step") with a small toggle check and its
// title. It owns its own click hook so the parent card can Map over a variable number of
// them without registering On* inside a loop.
func GoalTodoItem(props goalTodoProps) ui.Node { return ui.CreateElement(goalTodoItem, props) }

func goalTodoItem(props goalTodoProps) ui.Node {
	t := props.Task
	done := t.Status == domain.StatusDone
	toggle := ui.UseEvent(Prevent(func() {
		if props.OnToggle != nil {
			props.OnToggle(t.ID)
		}
	}))
	checkCls := "goal-todo-check"
	titleCls := "goal-todo-title"
	if done {
		checkCls += " is-done"
		titleCls += " is-done"
	}
	var glyph ui.Node = Fragment()
	if done {
		glyph = uiw.Icon(icon.Check, css.Class(tw.W35, tw.H35))
	}
	return Div(css.Class("goal-todo"),
		Button(ClassStr(checkCls), Type("button"), Attr("role", "checkbox"), Attr("aria-checked", ariaBool(done)),
			Attr("data-testid", "goal-todo-check-"+t.ID), Title(uistate.T("todo.toggle")), OnClick(toggle), glyph),
		Span(css.Class(titleCls), t.Title),
	)
}

// toggleGoalTodo flips a linked to-do's done state — completing routes through
// CompleteTask (so a recurring step spawns its successor), reopening is a plain status
// flip. Bumps the data revision so the goal's progress + checklist figures refresh.
func toggleGoalTodo(taskID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	for _, t := range app.Tasks() {
		if t.ID != taskID {
			continue
		}
		if t.Status == domain.StatusDone {
			t.Status = domain.StatusOpen
			_ = app.PutTask(t)
		} else {
			_ = app.CompleteTask(taskID, id.New(), time.Now())
		}
		uistate.BumpDataRevision()
		return
	}
}

// addGoalStep prompts for a title and adds a to-do linked to the goal (a checklist
// "step"): a Task with RelatedType=goal / RelatedID=goalID.
func addGoalStep(goalID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	uistate.PromptModal(uistate.T("goals.addStepPrompt"), "", func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		t := domain.Task{
			ID: id.New(), Title: name, Status: domain.StatusOpen, Priority: domain.PriorityMedium,
			Source: domain.SourceManual, RelatedType: domain.RelatedGoal, RelatedID: goalID,
		}
		if err := app.PutTask(t); err == nil {
			uistate.BumpDataRevision()
		}
	})
}

// markGoalReviewed stamps the goal's LastReviewedAt (via the appstate seam), clearing its
// "review due" flag until the next cadence step, and surfaces a confirmation toast.
func markGoalReviewed(goalID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	name := uistate.T("goals.thisGoal")
	for _, g := range app.Goals() {
		if g.ID == goalID && g.Name != "" {
			name = g.Name
			break
		}
	}
	if err := app.MarkGoalReviewed(goalID); err == nil {
		uistate.PostNotice(uistate.T("goals.reviewedToast", name), false)
		uistate.BumpDataRevision()
	}
}

// setMilestoneDone marks a milestone goal complete (done=true, stamping DoneAt) or
// reopens it (done=false), persisting through the store and refreshing the surface.
func setMilestoneDone(goalID string, done bool) {
	app := appstate.Default
	if app == nil {
		return
	}
	for _, g := range app.Goals() {
		if g.ID != goalID {
			continue
		}
		if done {
			g.DoneAt = time.Now()
		} else {
			g.DoneAt = time.Time{}
		}
		if err := app.PutGoal(g); err == nil {
			if done {
				uistate.PostNotice(uistate.T("goals.markedDoneToast"), false)
			}
			uistate.BumpDataRevision()
		}
		return
	}
}

// addHabitCheckIn records a check-in on a habit goal (appending now to CheckIns) and
// surfaces the resulting streak. The slice is copied before appending so the store's
// backing array is never mutated in place.
func addHabitCheckIn(goalID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	for _, g := range app.Goals() {
		if g.ID != goalID {
			continue
		}
		g.CheckIns = append(append([]time.Time{}, g.CheckIns...), time.Now())
		if err := app.PutGoal(g); err == nil {
			streak := goalsvc.HabitStreak(g, time.Now())
			uistate.PostNotice(uistate.T("goals.checkInToast", streak), false)
			uistate.BumpDataRevision()
		}
		return
	}
}

// bestQuickEarmark picks the account and amount for the card's one-click "Set aside"
// gesture: the goal's remaining coverage gap (target − saved − earmarked), funded from
// the linked account when it has free cash, else the eligible account with the most —
// never more than that account's free-to-earmark headroom. Returns ("", "", 0) when
// there's nothing to fund or nothing free.
func bestQuickEarmark(app *appstate.App, goalID string) (acctID, acctName string, amt int64) {
	var g domain.Goal
	found := false
	for _, gg := range app.Goals() {
		if gg.ID == goalID {
			g, found = gg, true
			break
		}
	}
	if !found || !g.IsFinancial() || g.TargetAmount.Amount <= 0 {
		return "", "", 0
	}
	gap := g.TargetAmount.Amount - goalsvc.CoverageMinor(g)
	if gap <= 0 {
		return "", "", 0
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	cur := g.TargetAmount.Currency
	if cur == "" {
		cur = base
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()
	free := func(a domain.Account) int64 {
		bal, _ := ledger.Balance(a, txns)
		inGoal := bal.Amount
		if bal.Currency != cur {
			if conv, err := rates.Convert(bal, cur); err == nil {
				inGoal = conv.Amount
			}
		}
		return goalsvc.AvailableToEarmarkMinor(app.Goals(), a.ID, inGoal, g.ID)
	}
	var best domain.Account
	var bestFree int64
	for _, a := range app.Accounts() {
		if a.Archived || !earmarkEligibleType(a.Type) {
			continue
		}
		f := free(a)
		if f <= 0 {
			continue
		}
		// The linked account wins whenever it can cover the gap outright; otherwise
		// the account with the most headroom.
		if a.ID == g.AccountID && f >= gap {
			best, bestFree = a, f
			break
		}
		if f > bestFree {
			best, bestFree = a, f
		}
	}
	if best.ID == "" {
		return "", "", 0
	}
	amt = gap
	if bestFree < amt {
		amt = bestFree
	}
	return best.ID, best.Name, amt
}

// goalFig renders one stat cell in a goal card's figures grid (redesign): a small
// uppercase label above a prominent serif value.
func goalFig(label, value string) ui.Node {
	return Div(css.Class("goal-fig"),
		Span(css.Class("goal-fig-k"), label),
		Span(css.Class("goal-fig-v"), value),
	)
}

// goalCardStateClass tints a goal card by its pace: a green stripe on-track, amber when
// due soon / in the final stretch, red when overdue, a calm "done" when complete.
func goalCardStateClass(p goalsvc.Pace, complete bool) string {
	if complete {
		return "is-done"
	}
	switch p {
	case goalsvc.PaceOverdue:
		return "is-overdue"
	case goalsvc.PaceDueSoon, goalsvc.PaceFinalStretch:
		return "is-soon"
	default:
		return "is-ontrack"
	}
}

// goalPriorityKey maps a stored goal priority (1..3) to its chip label key.
func goalPriorityKey(p int) string {
	switch p {
	case 1:
		return "goals.priorityChipHigh"
	case 2:
		return "goals.priorityChipMedium"
	default:
		return "goals.priorityChipLow"
	}
}
