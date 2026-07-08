// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

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

	now := time.Now()
	kind := g.EffectiveKind()
	financial := kind.IsFinancial()

	// Kind-aware progress: money for financial, linked-to-do count for checklist,
	// a binary done for milestone, check-ins/streak for habit.
	prog := goalsvc.EvaluateProgress(g, props.Tasks, now)
	pct := prog.Percent
	complete := prog.Complete
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
	default: // financial
		mainFig = Span(css.Class("budget-amount"), Span(css.Class("budget-spent"), fmtMoney(g.CurrentAmount)), " / "+fmtMoney(g.TargetAmount))
	}

	// Header chips. Financial: pace badge + monthly-needed + sinking-fund set-aside.
	// Habit: a streak chip. Others: none.
	var paceBadgeNode, monthlyChip, fundChip, streakChip ui.Node = Fragment(), Fragment(), Fragment(), Fragment()
	if financial {
		paceBadgeNode = paceBadge(pace)
		if !complete && !g.TargetDate.IsZero() {
			if per, ok, _ := goalsvc.MonthlyNeeded(g, now); ok {
				monthlyChip = Span(ClassStr("pace-badge pace-rate"),
					Attr("title", uistate.T("goals.paceNeededTitle")),
					uistate.T("goals.paceNeeded", fmtMoney(per)))
			}
		}
		if g.IsSinkingFund && props.FundSetAside > 0 {
			fundAmt := money.New(props.FundSetAside, g.CurrentAmount.Currency)
			fundChip = Span(ClassStr("pace-badge pace-rate"), Attr("data-testid", "fund-setaside-"+g.ID),
				uistate.T("goals.monthlySetAside", fmtMoney(fundAmt)))
		}
	} else if kind == domain.GoalKindHabit && prog.Streak > 0 {
		streakChip = Span(ClassStr("pace-badge pace-rate"), Attr("data-testid", "goal-streak-"+g.ID),
			uistate.T("goals.streakFmt", prog.Streak))
	}

	// Sub-section under the bar. Financial keeps its rich actionable copy (remaining,
	// deadline, monthly, over-fund, what-next, linked account, fund category). Non-
	// financial goals get a compact deadline / complete line.
	var subSection ui.Node
	if financial {
		rem, _ := goalsvc.Remaining(g)
		overfund, _ := goalsvc.Overfund(g)
		var subPrimary, subSecondary string
		if complete {
			subPrimary = uistate.T("goals.complete")
		} else {
			subPrimary = uistate.T("goals.remaining", fmtMoney(rem))
			subSecondary = fmt.Sprintf("%d%%", pct)
			if !g.TargetDate.IsZero() {
				subPrimary += uistate.T("goals.bySuffix", pr.FormatDate(g.TargetDate))
				if per, ok, _ := goalsvc.MonthlyNeeded(g, now); ok {
					subPrimary += uistate.T("goals.saveSuffix", fmtMoney(per))
				}
			}
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
			linkedLine = Span(css.Class("budget-sub"),
				Button(css.Class("budget-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drillAcct),
					Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
					uistate.T("goals.linkedSuffix", linkedName)))
		}
		var catLine ui.Node = Fragment()
		if g.IsSinkingFund && props.LinkedCategoryName != "" {
			catLine = Span(css.Class("budget-sub"), Attr("data-testid", "fund-category-"+g.ID),
				uistate.T("goals.fundLinkedCategory", props.LinkedCategoryName))
		}
		subSection = Fragment(
			Div(css.Class("budget-sub goal-sub"),
				Span(subPrimary),
				If(subSecondary != "", Span(css.Class("goal-sub-dim"), " · "+subSecondary)),
			),
			overfundNote, whatNext, linkedLine, catLine,
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

	// Primary footer action, per kind: Contribute (financial), Mark done / Reopen
	// (milestone), Check in (habit). Checklist has no direct action — its steps come
	// from linked to-dos (managed on the to-do page).
	var primaryAction ui.Node = Fragment()
	if !g.Archived {
		switch kind {
		case domain.GoalKindFinancial:
			primaryAction = Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-contribute-"+g.ID), Attr("aria-label", uistate.T("goals.contributeTitle")), Title(uistate.T("goals.contributeTitle")), OnClick(openContribute), uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.contribute")))
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

	// Archive / Unarchive live in the ⋯ menu (archive shows on any complete active goal).
	var archiveItem ui.Node = Fragment()
	if g.Archived {
		archiveItem = Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "goal-unarchive-"+g.ID), OnClick(doUnarchive), uistate.T("goals.unarchive"))
	} else if complete {
		archiveItem = Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "goal-archive-"+g.ID), OnClick(doArchive), uistate.T("goals.archive"))
	}

	// Contribution controls in the ⋯ menu (financial goals only): undo the most
	// recent contribution when there's a logged one, and reset saved progress to
	// zero when the goal holds anything. Both are hidden on archived goals.
	var undoItem, resetItem ui.Node = Fragment(), Fragment()
	if financial && !g.Archived {
		if len(g.Contributions) > 0 {
			undoItem = Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
				Attr("data-testid", "goal-undo-contrib-"+g.ID), OnClick(doUndoContribution), uistate.T("goals.undoContribution"))
		}
		if g.CurrentAmount.Amount > 0 {
			resetItem = Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
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
		var items []ui.Node
		for _, lt := range linked {
			items = append(items, ui.CreateElement(GoalTodoItem, goalTodoProps{Task: lt, OnToggle: toggleTodo}))
		}
		var body ui.Node
		if len(items) == 0 {
			body = P(css.Class("goal-todos-empty"), uistate.T("goals.noSteps"))
		} else {
			body = Div(css.Class("goal-todos-list"), items)
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
			addBtn,
		)
	}

	return Div(ClassStr("goal-card "+cardState),
		Attr("data-testid", "goal-row-"+g.ID), Attr("data-kind", string(kind)),
		// Header: the goal name on its own line, with kind-appropriate chips.
		Div(css.Class("goal-card-head"),
			Span(css.Class("goal-card-title"), g.Name),
			paceBadgeNode,
			monthlyChip,
			fundChip,
			streakChip,
		),
		// The card's "loader": a progress bar with a kind-appropriate label + percent inside it.
		Div(css.Class("goal-card-loader"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(pct)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("goals.progressLabel")),
			Div(ClassStr("bar-fill "+barClass), Attr("style", barFillStyle(pct))),
			Div(css.Class("goal-card-loader-figs"),
				mainFig,
				pctFig,
			),
		),
		subSection,
		todosSection,
		// Footer: the primary kind action + Edit open the flip modal; the shared
		// viewport-aware ⋯ KebabMenu holds archive + the destructive delete.
		Div(css.Class("goal-card-actions"),
			primaryAction,
			If(!g.Archived, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-edit-btn-"+g.ID), Attr("aria-label", uistate.T("goals.editTitle")), Title(uistate.T("goals.editTitle")), OnClick(openEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit")))),
			uiw.KebabMenu(uiw.KebabMenuProps{
				ID:           "goal-menu-" + g.ID,
				AriaLabel:    uistate.T("goals.moreActions"),
				ToggleTestID: "goal-menu-btn-" + g.ID,
				Items: []ui.Node{
					undoItem,
					resetItem,
					archiveItem,
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
