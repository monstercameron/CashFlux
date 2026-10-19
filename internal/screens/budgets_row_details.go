// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalfunded"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetRowCoverage builds the row coverage section.
func budgetRowCoverage(s budgeting.Status, props budgetRowProps) (bool, ui.Node) {
	hasRecurring := s.Budget.RecurringCover != nil
	// Coverage badge — differentiate continual (recurring) from a one-time cover this
	// period. Recurring wins (it's inherently covered), so the two never both show.
	var coverageLine ui.Node = Fragment()
	if hasRecurring {
		coverageLine = Span(css.Class("budget-sub", "budget-recurring"), Attr("data-testid", "recurring-badge-"+s.Budget.ID), uistate.T("budgets.recurringBadge"))
	} else if props.Covered {
		coverageLine = Span(css.Class("budget-sub", "budget-covered"), Attr("data-testid", "covered-badge-"+s.Budget.ID), uistate.T("budgets.coveredBadge"))
	}

	return hasRecurring, coverageLine
}

// budgetRowTitleResult carries the explicit results for budgetRowTitle.
type budgetRowTitleResult struct {
	noteText   string
	hasNote    bool
	notesLabel string
	drillTitle string
	ownerLine  ui.Node
	methodLine ui.Node
}

// budgetRowTitle builds the row title section.
func budgetRowTitle(s budgeting.Status, title string, drillScope budgeting.Scope, props budgetRowProps) budgetRowTitleResult {
	// The note, and what to call the action that opens it. Every other item in the
	// row's ⋯ menu is a verb phrase; "Notes" was the one noun, and it read the same
	// whether it would create a note or overwrite one.
	noteText := strings.TrimSpace(s.Budget.Notes)
	hasNote := noteText != ""
	notesLabel := uistate.T("budgets.notesAdd")
	if hasNote {
		notesLabel = uistate.T("budgets.notesEdit")
	}
	// One tooltip for the drill action, in BOTH densities. The two used to differ,
	// and the compact one interpolated the primary category name — which is empty
	// for a multi-category or tag-tracking budget, leaving "See all  transactions
	// in this period" on exactly the budgets whose scope most needed naming.
	drillTitle := budgetDrillTitle(title, s.Budget, drillScope)

	// Owner tag (L106 learning): an INDIVIDUAL budget only counts its owner's spending, so a household
	// can't otherwise tell why a shared expense didn't move it. Flag whose it is — but only for
	// individual budgets (OwnerID matches a real member); shared/household budgets (the common default,
	// OwnerID = group) stay unlabeled to keep rows clean.
	var ownerLine ui.Node = Fragment()
	for _, m := range props.Members {
		if m.ID == s.Budget.OwnerID {
			ownerLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.individualOwner", m.Name))
			break
		}
	}

	// C118: show a small method badge when this budget has its own method override,
	// so the user can see at a glance which budget uses a different approach from
	// the household default. Hidden when the budget inherits the global method.
	var methodLine ui.Node = Fragment()
	if s.Budget.Methodology != "" {
		methodLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.methodOverrideRow", budgetMethodLabel(budgeting.ParseMethodology(s.Budget.Methodology))))
	}
	return budgetRowTitleResult{
		noteText: noteText, hasNote: hasNote, notesLabel: notesLabel, drillTitle: drillTitle, ownerLine: ownerLine, methodLine: methodLine,
	}
}

// budgetRowMetadata builds the row metadata section.
func budgetRowMetadata(props budgetRowProps, s budgeting.Status, lastMonthMode bool) (ui.Node, ui.Node, ui.Node) {

	// Custom-field summary (e.g. "Priority: High · Review: Q3") — shown when the budget
	// has any user-defined field values, so custom data stays visible on the row.
	var customLine ui.Node = Fragment()
	if cs := customSummary(props.BudgetDefs, s.Budget.Custom); cs != "" {
		customLine = Span(css.Class("budget-sub", tw.TextFaint), cs)
	}

	// Envelope methodology: show the carried-forward balance under the period row.
	// Like every this-month caption below, it hides in last-month mode: the bar then
	// shows a DIFFERENT period's spend, and a this-month pace/carry/cap line under a
	// last-month bar reads as an internal contradiction (the L-overlay critique).
	var envLine ui.Node = Fragment()
	if !lastMonthMode && props.Envelope != "" {
		cls := "budget-sub"
		if props.EnvelopeNeg {
			cls += " " + tw.Fold(tw.TextDown)
		}
		envLine = Span(ClassStr(cls), uistate.T("budgets.envelopeRow", props.Envelope))
	}

	// BG5: an overdrawn envelope names what next period starts down, so the overdraft
	// doesn't quietly vanish at the boundary. Caution amber (not danger red): it's a
	// heads-up about where next period STARTS, not a "you're over now" alarm.
	var envDebtLine ui.Node = Fragment()
	if !lastMonthMode && props.EnvelopeDebtStart != "" {
		envDebtLine = Span(css.Class("budget-sub", tw.TextWarn), Attr("data-testid", "budget-envdebt-"+s.Budget.ID),
			Attr("role", "status"), props.EnvelopeDebtStart)
	}

	return customLine, envLine, envDebtLine
}

// budgetRowPaceNotesResult carries the explicit results for budgetRowPaceNotes.
type budgetRowPaceNotesResult struct {
	paceLine         ui.Node
	paceMarkLine     ui.Node
	paceLegend       ui.Node
	rolloverLine     ui.Node
	effectiveCapLine ui.Node
}

// budgetRowPaceNotes builds the row pace notes section.
func budgetRowPaceNotes(lastMonthMode bool, props budgetRowProps, s budgeting.Status, prefs prefs.Prefs) budgetRowPaceNotesResult {
	// Pace projection (D2): a gentle heads-up when current spending would blow the
	// budget by period end, shown only while the period is still in progress.
	var paceLine ui.Node = Fragment()
	if !lastMonthMode && props.PaceOver != "" {
		paceLine = Span(css.Class("budget-sub", tw.TextDown), uistate.T("budgets.paceOver", props.PaceOver))
	}

	// BG3: the even-pace caption ("on pace" / "running $38 hot" / "$12 under pace so
	// far"). A hot budget reads in the caution amber; on/under pace stays faint. Only
	// shown while in progress and when the pace overspend warning isn't already owning
	// the message (paceOver is the stronger signal).
	var paceMarkLine ui.Node = Fragment()
	if !lastMonthMode && props.PaceCaption != "" && props.PaceOver == "" {
		tone := tw.TextFaint
		if props.PaceHot {
			tone = tw.TextWarn
		}
		paceMarkLine = Span(css.Class("budget-sub", tone), Attr("data-testid", "budget-pace-caption-"+s.Budget.ID),
			Attr("role", "status"), props.PaceCaption)
	}
	var paceLegend ui.Node = Fragment()
	if !lastMonthMode && props.PaceMarkerPct > 0 && s.Spent.Amount > 0 {
		anchor := props.Anchor
		if anchor.IsZero() {
			anchor = time.Now()
		}
		from, to := budgeting.PeriodRange(s.Budget.Period, anchor, prefs.WeekStartWeekday())
		elapsed := 0
		if total := to.Sub(from).Hours(); total > 0 {
			elapsed = int(time.Since(from).Hours() / total * 100)
			if elapsed < 0 {
				elapsed = 0
			} else if elapsed > 100 {
				elapsed = 100
			}
		}
		paceLegend = Span(css.Class("budget-pace-legend"), Attr("data-testid", "budget-pace-legend-"+s.Budget.ID),
			uistate.T("budgets.paceLegend", elapsed))
	}

	var rolloverLine ui.Node = Fragment()
	if !lastMonthMode && props.RolloverCarry != "" {
		// A carried-in deficit is historical context about where the period STARTED, not a
		// live alert — keep it quiet/neutral (TextDim) so an over-budget card has ONE colored
		// signal (the red status line) rather than several caption lines competing for the eye.
		cls := "budget-sub " + tw.Fold(tw.TextDim)
		rolloverLine = Span(ClassStr(cls), uistate.T("budgets.rolloverCarry", props.RolloverCarry))
	}

	// C136: show the effective cap when it differs from the base limit — WITH its
	// arithmetic (base limit ± carry-over ± top-up), so the number is explainable at a
	// glance instead of an opaque figure. Hidden when cap == base limit (no note needed).
	var effectiveCapLine ui.Node = Fragment()
	if !lastMonthMode && props.EffectiveCap != "" {
		if props.EffectiveCapMath != "" {
			effectiveCapLine = Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-capmath-"+s.Budget.ID),
				uistate.T("budgets.effectiveCapMath", props.EffectiveCap, props.EffectiveCapMath))
		} else {
			effectiveCapLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.effectiveCap", props.EffectiveCap))
		}
	}
	return budgetRowPaceNotesResult{
		paceLine: paceLine, paceMarkLine: paceMarkLine, paceLegend: paceLegend, rolloverLine: rolloverLine,
		effectiveCapLine: effectiveCapLine,
	}
}

// budgetRowPeriodNotesResult carries the explicit results for budgetRowPeriodNotes.
type budgetRowPeriodNotesResult struct {
	proratedLine ui.Node
	lastMonthTag ui.Node
	thisMonthRef ui.Node
	isOver       bool
	coverBtn     ui.Node
}

// budgetRowPeriodNotes builds the row period notes section.
func budgetRowPeriodNotes(lastMonthMode bool, props budgetRowProps, s budgeting.Status, openCover ui.Handler) budgetRowPeriodNotesResult {
	// C143: even-pace guidance — how much of what's left can be spent over the days
	// still in the period, so the user knows the sustainable daily-ish pace instead
	// of seeing only a lump remaining. Quiet faint line; only set while in-progress.
	var proratedLine ui.Node = Fragment()
	if !lastMonthMode && props.ProratedRest != "" {
		proratedLine = Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-prorated"),
			uistate.T("budgets.proratedRest", props.ProratedRest))
	}

	// "Last month's spend" overlay: an overline tag above the (now last-month) bar, plus a
	// faint reference line showing what THIS month has actually spent — so the tile leads
	// with last month while this month stays visible for comparison.
	var lastMonthTag, thisMonthRef ui.Node = Fragment(), Fragment()
	if lastMonthMode {
		lastMonthTag = Div(css.Class("budget-lastmonth-tag"), Attr("data-testid", "budget-lastmonth-"+s.Budget.ID),
			Attr("aria-label", uistate.T("budgets.lastMonthAria", props.LastMonthSpent, props.LastMonthDelta)),
			uistate.T("budgets.lastMonthCap"))
		thisMonthRef = Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-thismonth-ref-"+s.Budget.ID),
			uistate.T("budgets.thisMonthRef", fmtMoney(s.Spent)))
	}

	// "Cover…" is offered on an over-budget row and opens the flip modal (which lists
	// the other budgets to pull from). Top up is offered when not over.
	isOver := s.State == budgeting.StateOver
	var coverBtn ui.Node = Fragment()
	if isOver {
		coverBtn = Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-cover-btn-"+s.Budget.ID), Title(uistate.T("budgets.coverTitle")), OnClick(openCover), uistate.T("budgets.actionCoverFromOther"))
	}
	return budgetRowPeriodNotesResult{
		proratedLine: proratedLine, lastMonthTag: lastMonthTag, thisMonthRef: thisMonthRef, isOver: isOver, coverBtn: coverBtn,
	}
}

// budgetRowGoalFunding builds the row goal funding section.
func budgetRowGoalFunding(lastMonthMode bool, s budgeting.Status, props budgetRowProps, dataRevision int) (ui.Node, string) {
	// COMPACT density: one scannable line — name, mini bar, spent/limit, what's left,
	// a status chip, and the ⋯ menu (which absorbs the money moves above). Everything
	// analytical (pie, metrics, captions, notes, to-dos) belongs to the card layout;
	// compact is for reading fifteen budgets without scrolling (design critique #9).
	// WF18: spending a goal paid for is not overspending. Two transactions can
	// look identical in the ledger and mean opposite things — one blew the budget,
	// the other came out of a fund saved for exactly this — and a budget that
	// cannot tell them apart punishes somebody for executing their own plan.
	//
	// The funded amount is SEPARATED, never deducted: the money still left the
	// account, and a budget that quietly shrank its own totals would be a worse
	// lie than the one being fixed.
	var fundedLine ui.Node = Fragment()
	fundedChip, fundedTitle := "", ""
	if app := appstate.Default; app != nil && !lastMonthMode {
		cats := map[string]bool{}
		if s.Budget.CategoryID != "" {
			cats[s.Budget.CategoryID] = true
		}
		for _, c := range s.Budget.TrackedCategoryIDs() {
			cats[c] = true
		}
		base := app.Settings().BaseCurrency
		if base == "" {
			base = budgetBaseCurrency(base)
		}
		// The row does not carry its period bounds, so derive them the same way the
		// page does — from the budget's own period against the household's week
		// start. Re-deriving here rather than threading a new prop keeps this
		// additive, and both paths call the same function so they cannot disagree.
		anchor := props.Anchor
		if anchor.IsZero() {
			anchor = time.Now()
		}
		payAnchor := budgetPayCycleAnchor(uistate.LoadPrefs().PayCycleAnchor)
		pStart, pEnd := budgeting.PeriodRangeAnchored(s.Budget.Period, anchor,
			uistate.LoadPrefs().WeekStartWeekday(), payAnchor)
		split := memoGoalFunded(app, dataRevision, s.Budget.ID, pStart, pEnd, func() goalfunded.Split {
			return goalfunded.Summarize(app.Transactions(), cats, pStart, pEnd,
				currency.Rates{Base: base, Rates: app.Settings().FXRates})
		})
		if split.HasFunded() {
			key := "budgets.goalFundedAvailableMany"
			if len(split.Goals) == 1 {
				key = "budgets.goalFundedAvailableOne"
			}
			fundedTitle = uistate.T(key, fmtMoney(money.New(split.FundedMinor, base)), budgetGoalNames(app.Goals(), split.Goals), fmtMoney(s.Limit))
			fundedChip = fundedTitle
			fundedLine = Span(css.Class("budget-sub", tw.TextDim),
				Attr("data-testid", "budget-goalfunded-"+s.Budget.ID), fundedTitle)
		}
	}

	return fundedLine, fundedChip
}

func budgetGoalNames(goals []domain.Goal, ids []string) string {
	namesByID := make(map[string]string, len(goals))
	for _, goal := range goals {
		namesByID[goal.ID] = strings.TrimSpace(goal.Name)
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		name := strings.TrimSpace(namesByID[id])
		if name == "" {
			name = uistate.T("budgets.goalFundedUnnamed")
		}
		names = append(names, "'"+name+"'")
	}
	if len(names) == 0 {
		return uistate.T("budgets.goalFundedUnnamed")
	}
	return strings.Join(names, ", ")
}

// budgetRowNotesProps carries the explicit inputs for budgetRowNotes.
type budgetRowNotesProps struct {
	hasNote    bool
	s          budgeting.Status
	notesLabel string
	title      string
	openNotes  ui.Handler
	noteText   string
}

// budgetRowNotes builds the row notes section.
func budgetRowNotes(p budgetRowNotesProps) (ui.Node, bool) {
	var notesNode ui.Node = Fragment()
	hasNotes := p.hasNote
	if p.hasNote {
		// Clicking the note opens the full note in the flip modal (handy when it's long) —
		// the card shows a clamped preview.
		//
		// The accessible name carries the budget it belongs to. On a page listing a
		// dozen budgets, an unqualified "Notes, button" repeated on every card left a
		// screen-reader user unable to tell whose note was next without backing up to
		// the heading.
		notesNode = Button(ClassStr("acct-notes budget-notes"), Type("button"), Attr("data-testid", "budget-notes-"+p.s.Budget.ID),
			Attr("aria-label", p.notesLabel+" — "+p.title),
			Title(uistate.T("budgets.notesTitle")), OnClick(p.openNotes),
			uiw.Icon(icon.FileText, css.Class("acct-notes-icon", tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("acct-notes-text"), p.noteText),
		)
	}

	return notesNode, hasNotes
}

// budgetRowMetrics builds the row metrics section.
func budgetRowMetrics(lastMonthMode bool, props budgetRowProps, s budgeting.Status, prefs prefs.Prefs) ui.Node {
	// A quiet link to the To-dos page when this budget has linked to-dos.
	// A quick, scannable metric strip — the full-width card has room for the TIME
	// dimension the prose lines don't make glanceable: how much is left to spend per
	// remaining day, how many days remain (and when it resets), and how far through the
	// period we are. Hidden in the last-month overlay (which shows a different period).
	var metricsStrip ui.Node = Fragment()
	if !lastMonthMode {
		// QA CF-05: the period must come from the VIEW's anchor, not today — a
		// June card paged into July was showing "14 days left · 52% elapsed ·
		// Aug 1" (all current-month pacing). Completed periods state final facts;
		// future periods say when they start; only a period containing today gets
		// the live per-day / days-left / elapsed race.
		nowT := time.Now()
		anchorT := props.Anchor
		if anchorT.IsZero() {
			anchorT = nowT
		}
		pStart, pEnd := budgeting.PeriodRange(s.Budget.Period, anchorT, prefs.WeekStartWeekday())
		switch {
		case !nowT.Before(pEnd): // completed period — history has no pacing
			metricsStrip = Div(css.Class("budget-metrics"), Attr("data-testid", "budget-metrics-"+s.Budget.ID),
				budgetMetric(uistate.T("budgetMetrics.ended"), prefs.FormatDate(pEnd.AddDate(0, 0, -1)), ""),
				budgetMetric(uistate.T("budgetMetrics.elapsed"), "100%", ""),
			)
		case nowT.Before(pStart): // future period — nothing has elapsed yet
			metricsStrip = Div(css.Class("budget-metrics"), Attr("data-testid", "budget-metrics-"+s.Budget.ID),
				budgetMetric(uistate.T("budgetMetrics.startsOn"), prefs.FormatDate(pStart), ""),
				budgetMetric(uistate.T("budgetMetrics.elapsed"), "0%", ""),
			)
		default: // the viewed period contains today — live pacing
			daysLeft := int(pEnd.Sub(nowT).Hours() / 24)
			if daysLeft < 1 {
				daysLeft = 1
			}
			elapsed := 0
			if total := pEnd.Sub(pStart).Hours(); total > 0 {
				elapsed = int(nowT.Sub(pStart).Hours() / total * 100)
				if elapsed < 0 {
					elapsed = 0
				} else if elapsed > 100 {
					elapsed = 100
				}
			}
			perDay := int64(0)
			if s.Remaining.Amount > 0 {
				perDay = s.Remaining.Amount / int64(daysLeft)
			}
			metricsStrip = Div(css.Class("budget-metrics"), Attr("data-testid", "budget-metrics-"+s.Budget.ID),
				budgetMetric(uistate.T("budgetMetrics.perDay"), fmtMoney(money.New(perDay, s.Remaining.Currency)), ""),
				budgetMetric(uistate.T("budgetMetrics.daysLeft"), strconv.Itoa(daysLeft), prefs.FormatDate(pEnd)),
				budgetMetric(uistate.T("budgetMetrics.elapsed"), fmt.Sprintf("%d%%", elapsed), ""),
			)
		}
	}

	return metricsStrip
}

// budgetRowFollowUpsProps carries the explicit inputs for budgetRowFollowUps.
type budgetRowFollowUpsProps struct {
	s               budgeting.Status
	todosOpen       ui.State[bool]
	toggleTodos     ui.Handler
	openBudgetTodos func()
	openTodos       ui.Handler
	hasNotes        bool
}

// budgetRowFollowUps builds the row follow ups section.
func budgetRowFollowUps(p budgetRowFollowUpsProps) (ui.Node, bool) {
	// any) — check-off in place, like the transaction follow-ups. Shown whenever either
	// exists; otherwise the column is omitted and the main content fills the width.
	var linkedTodos []followUpItem
	if app := appstate.Default; app != nil {
		linkedTodos = memoBudgetFollowUps(app, app.Rev(), p.s.Budget.ID, func() []followUpItem {
			return budgetLinkedTodos(p.s.Budget.ID)
		})
	}
	var todosPanel ui.Node = Fragment()
	if len(linkedTodos) > 0 {
		const maxTodos = 5
		open := 0
		for _, it := range linkedTodos {
			if !it.Done {
				open++
			}
		}
		toggleHint := uistate.T("budgets.followUpsShow")
		chev := icon.ChevronDown
		if p.todosOpen.Get() {
			toggleHint = uistate.T("budgets.followUpsHide")
			chev = icon.ChevronUp
		}
		kids := []any{css.Class("budget-todos"), Attr("data-testid", "budget-todos-"+p.s.Budget.ID),
			Button(css.Class("budget-todos-head"), Type("button"),
				Attr("data-testid", "budget-todos-toggle-"+p.s.Budget.ID),
				Attr("aria-label", uistate.T("budgets.followUpsHead", open, len(linkedTodos))),
				Attr("aria-expanded", ariaBool(p.todosOpen.Get())), Title(toggleHint), OnClick(p.toggleTodos),
				Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0",
					"font": "inherit", "color": "inherit", "cursor": "pointer",
					"display": "inline-flex", "align-items": "center", "gap": "0.25rem", "width": "fit-content", "min-width": "0"}),
				Span(css.Class("budget-followup-count"), strconv.Itoa(open)),
				uiw.Icon(chev, css.Class(tw.ShrinkO, tw.W35, tw.H35)))}
		if p.todosOpen.Get() {
			for i, it := range linkedTodos {
				if i >= maxTodos {
					break
				}
				kids = append(kids, ui.CreateElement(txnFollowUpItem, txnFollowUpItemProps{ID: it.ID, Title: it.Title, Done: it.Done, Due: it.Due, OnOpen: p.openBudgetTodos}))
			}
			if extra := len(linkedTodos) - maxTodos; extra > 0 {
				kids = append(kids, Button(css.Class("txnfu-pop-foot"), Type("button"), Attr("data-testid", "budget-todos-more-"+p.s.Budget.ID),
					OnClick(p.openTodos), uistate.T("budgets.followUpsMore", extra)))
			}
		}
		todosPanel = Div(kids...)
	}
	hasSide := p.hasNotes || len(linkedTodos) > 0

	return todosPanel, hasSide
}

type budgetFollowUpMemoKey struct {
	app      *appstate.App
	revision uint64
	budgetID string
}

var budgetFollowUpMemos = struct {
	sync.Mutex
	values map[budgetFollowUpMemoKey][]followUpItem
}{values: make(map[budgetFollowUpMemoKey][]followUpItem)}

func memoBudgetFollowUps(app *appstate.App, revision uint64, budgetID string, calculate func() []followUpItem) []followUpItem {
	key := budgetFollowUpMemoKey{app: app, revision: revision, budgetID: budgetID}
	budgetFollowUpMemos.Lock()
	if value, ok := budgetFollowUpMemos.values[key]; ok {
		budgetFollowUpMemos.Unlock()
		return value
	}
	budgetFollowUpMemos.Unlock()
	value := calculate()
	budgetFollowUpMemos.Lock()
	if len(budgetFollowUpMemos.values) > 512 {
		budgetFollowUpMemos.values = make(map[budgetFollowUpMemoKey][]followUpItem)
	}
	budgetFollowUpMemos.values[key] = append([]followUpItem(nil), value...)
	budgetFollowUpMemos.Unlock()
	return value
}
