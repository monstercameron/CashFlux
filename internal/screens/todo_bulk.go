// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/taskbulk"
	"github.com/monstercameron/CashFlux/internal/todoview"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// todoBulkBarProps is everything the bulk-action bar needs. The list owns the
// selection itself — which rows are picked has to be known by the rows — so the
// bar receives only the count and reports gestures back through callbacks.
type todoBulkBarProps struct {
	// Active is selection mode. Off, the bar is a single "Select several" button.
	Active bool
	// Count is how many rows are currently picked.
	Count int
	// HasRows suppresses the entry point on an empty list: offering to
	// multi-select nothing is dead UI.
	HasRows bool
	Members []domain.Member
	// UndoCount / UndoLabel describe the last bulk write that can be reversed.
	UndoCount int
	UndoLabel string

	OnEnter      func()
	OnExit       func()
	OnSelectAll  func()
	OnClear      func()
	OnAssign     func(memberID string)
	OnReschedule func(shift taskbulk.DueShift)
	OnComplete   func()
	OnDelete     func()
	OnUndo       func()
}

// dueShiftOption pairs a bulk due-date target with its catalog key.
type dueShiftOption struct {
	shift taskbulk.DueShift
	key   string
}

// dueShiftOptions is the reschedule menu, in the order a triage pass wants it:
// the near targets first, then the relative push, then the escape hatch.
var dueShiftOptions = []dueShiftOption{
	{taskbulk.ShiftToday, "todo.bulkDueToday"},
	{taskbulk.ShiftTomorrow, "todo.bulkDueTomorrow"},
	{taskbulk.ShiftNextWeek, "todo.bulkDueNextWeek"},
	{taskbulk.ShiftNextMonth, "todo.bulkDueNextMonth"},
	{taskbulk.ShiftPushWeek, "todo.bulkDuePushWeek"},
	{taskbulk.ShiftClear, "todo.bulkDueClear"},
}

// todoBulkBar renders the to-do list's multi-select action bar plus the undo
// banner for the last bulk write (C402).
//
// It is a component rather than inline markup for a structural reason: its
// buttons need event hooks, and the list function returns early for the board
// and calendar views. Hooks declared after those returns would change position
// whenever the user switched view, which is the one thing GWC's hook model
// cannot survive. A child owns its own hook list and simply is not rendered.
//
// The undo banner is deliberately independent of selection mode — leaving the
// mode should not strand the one control that puts twelve overwritten rows back.
func todoBulkBar(props todoBulkBarProps) ui.Node {
	// The pending edit lives here, not in the list: it is the bar's own scratch
	// state and nothing else reads it.
	member := ui.UseState("")
	due := ui.UseState("")

	onMember := ui.UseEvent(func(e ui.Event) { member.Set(e.GetValue()) })
	onDue := ui.UseEvent(func(e ui.Event) { due.Set(e.GetValue()) })
	enter := ui.UseEvent(Prevent(func() { props.OnEnter() }))
	exit := ui.UseEvent(Prevent(func() { props.OnExit() }))
	selectAll := ui.UseEvent(Prevent(func() { props.OnSelectAll() }))
	clear := ui.UseEvent(Prevent(func() { props.OnClear() }))
	assign := ui.UseEvent(Prevent(func() {
		if member.Get() != "" {
			props.OnAssign(member.Get())
		}
	}))
	reschedule := ui.UseEvent(Prevent(func() {
		if due.Get() != "" {
			props.OnReschedule(taskbulk.DueShift(due.Get()))
		}
	}))
	complete := ui.UseEvent(Prevent(func() { props.OnComplete() }))
	del := ui.UseEvent(Prevent(func() { props.OnDelete() }))
	undo := ui.UseEvent(Prevent(func() { props.OnUndo() }))

	var undoBanner ui.Node = Fragment()
	if props.UndoCount > 0 {
		undoBanner = Div(css.Class("todo-bulkbar"), Attr("data-testid", "todo-bulk-undo-bar"),
			Span(css.Class("muted"), props.UndoLabel),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-undo"),
				OnClick(undo), uistate.T("action.undo")))
	}

	if !props.Active {
		var entry ui.Node = Fragment()
		if props.HasRows {
			entry = Div(css.Class("todo-bulk-entry"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-enter"),
					OnClick(enter), uistate.T("todo.bulkSelectMode")))
		}
		return Fragment(entry, undoBanner)
	}

	memberOpts := []ui.Node{Option(Value(""), SelectedIf(member.Get() == ""), uistate.T("todo.bulkAssignPlaceholder"))}
	for _, m := range props.Members {
		memberOpts = append(memberOpts, Option(Value(m.ID), SelectedIf(member.Get() == m.ID), m.Name))
	}
	memberOpts = append(memberOpts,
		Option(Value(taskbulk.Unassigned), SelectedIf(member.Get() == taskbulk.Unassigned), uistate.T("todo.bulkUnassign")))

	dueOpts := []ui.Node{Option(Value(""), SelectedIf(due.Get() == ""), uistate.T("todo.bulkDuePlaceholder"))}
	for _, o := range dueShiftOptions {
		dueOpts = append(dueOpts, Option(Value(string(o.shift)), SelectedIf(due.Get() == string(o.shift)), uistate.T(o.key)))
	}

	// The action controls appear only once something is picked. A row of buttons
	// that would act on nothing invites the click and then does nothing with it.
	actions := Fragment()
	if props.Count > 0 {
		actions = Fragment(
			Select(css.Class("field"), Attr("data-testid", "todo-bulk-member"),
				Attr("aria-label", uistate.T("todo.bulkAssignPlaceholder")), OnChange(onMember), memberOpts),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-assign"),
				OnClick(assign), uistate.T("todo.bulkAssign")),
			Select(css.Class("field"), Attr("data-testid", "todo-bulk-due"),
				Attr("aria-label", uistate.T("todo.bulkDuePlaceholder")), OnChange(onDue), dueOpts),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-reschedule"),
				OnClick(reschedule), uistate.T("todo.bulkReschedule")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-complete"),
				OnClick(complete), uistate.T("todo.bulkComplete")),
			Button(css.Class("btn-del btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-delete"),
				OnClick(del), uistate.T("todo.bulkDelete")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-clear"),
				OnClick(clear), uistate.T("todo.bulkClear")),
		)
	}

	return Fragment(
		Div(css.Class("todo-bulkbar"), Attr("data-testid", "todo-bulkbar"),
			Attr("role", "group"), Attr("aria-label", uistate.T("todo.bulkBarLabel")),
			// A live region: the count changes by keyboard and by shift-click range,
			// and a screen-reader user needs to hear how many rows the next button
			// press is about to affect.
			Span(css.Class("muted"), Attr("data-testid", "todo-bulk-count"),
				Attr("role", "status"), Attr("aria-live", "polite"),
				uistate.T("todo.bulkSelected", props.Count)),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-all"),
				OnClick(selectAll), uistate.T("todo.bulkSelectAll")),
			actions,
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "todo-bulk-exit"),
				OnClick(exit), uistate.T("todo.bulkExit")),
		),
		undoBanner,
	)
}

// viewSaveError turns a todoview save failure into copy a person can act on
// (C404). The package returns sentinel errors precisely so the message can be
// translated here rather than baked into a pure package in English.
func viewSaveError(err error) string {
	switch {
	case errors.Is(err, todoview.ErrNameRequired):
		return uistate.T("todo.viewNeedsName")
	case errors.Is(err, todoview.ErrTooMany):
		return uistate.T("todo.viewTooMany", todoview.MaxViews)
	}
	return err.Error()
}

// todoPriorityName is a priority filter value as its human label, for a filter
// chip. An unknown value renders as itself rather than as an empty chip that
// says nothing was filtered when something was.
func todoPriorityName(p string) string {
	switch domain.TaskPriority(p) {
	case domain.PriorityHigh:
		return uistate.T("priority.high")
	case domain.PriorityMedium:
		return uistate.T("priority.medium")
	case domain.PriorityLow:
		return uistate.T("priority.low")
	}
	return p
}

// todoLinkName is a link-type filter value as its human label, for a filter chip.
func todoLinkName(l string) string {
	switch l {
	case uistate.TodoLinkGoal:
		return uistate.T("todo.linkGoalPl")
	case uistate.TodoLinkBudget:
		return uistate.T("todo.linkBudgetPl")
	case uistate.TodoLinkAccount:
		return uistate.T("todo.linkAccountPl")
	case uistate.TodoLinkTransaction:
		return uistate.T("todo.linkTransactionPl")
	case uistate.TodoLinkNone:
		return uistate.T("todo.linkFilterNone")
	}
	return l
}
