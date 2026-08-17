// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/taskboard"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// TaskBoardProps drives the to-do board (kanban) view. The coordinator owns the
// list/board/calendar switcher and the group-by control, so it supplies the task
// list, the grouping dimension, the open/move callbacks, and a data-revision.
type TaskBoardProps struct {
	// Tasks is the (already view-filtered) list of tasks to lay out on the board.
	Tasks []domain.Task
	// By selects the grouping dimension (status or priority).
	By taskboard.GroupBy
	// OnOpen opens a task for editing; the coordinator supplies it (may be nil).
	OnOpen func(taskID string)
	// OnMove moves a task to the column identified by toKey — a domain.TaskStatus
	// string ("open"/"done") when grouping by status, or a domain.TaskPriority
	// string ("high"/"med"/"low") when grouping by priority. The coordinator maps
	// the key back onto the task's field and persists it (may be nil).
	OnMove func(taskID string, toKey string)
	// Rev is a data-revision the coordinator bumps when the underlying tasks
	// change. It is part of the props so a re-render defeats prop-equality
	// memoization when the data moves but the slice identity does not.
	Rev int
}

// TaskBoardView renders the board: a horizontally-scrolling row of column cards
// grouped by TaskBoardProps.By. Each card is a BoardCard sub-component (owning its
// own click hooks) so this view never registers On* handlers inside a loop.
func TaskBoardView(props TaskBoardProps) ui.Node {
	pr := uistate.UsePrefs().Get()
	// Rev is read purely so the value participates in this component's render; the
	// coordinator bumps it to force a re-render when task data changes underneath.
	_ = props.Rev

	now := time.Now()
	todayISO := dateutil.FormatDate(now)
	cols := taskboard.Columns(props.Tasks, props.By)

	return Div(css.Class("tdb-wrap"),
		Attr("data-testid", "todo-board"),
		Attr("role", "list"), Attr("aria-label", uistate.T("todoboard.boardLabel")),
		MapKeyed(cols,
			func(c taskboard.Column) any { return c.Key },
			func(c taskboard.Column) ui.Node {
				// The next column a card in this lane advances to (one-click move).
				nextKey, hasNext := taskboard.NextKey(props.By, c.Key)
				nextTitle := ""
				if hasNext {
					nextTitle = uistate.T(columnTitleForKey(cols, nextKey))
				}

				var body ui.Node
				if len(c.Tasks) == 0 {
					// DP-F5a: an empty column was dead-end text. It is also the one place
					// on the board where the user's intent is unambiguous — they are
					// looking at an empty lane — so it is the cheapest possible place to
					// offer the add. Its own component because the handler would otherwise
					// be registered inside this MapKeyed loop, which the hook model forbids.
					body = ui.CreateElement(boardEmptyColumn, boardEmptyColumnProps{
						ColumnKey: c.Key, By: props.By,
					})
				} else {
					cards := MapKeyed(c.Tasks,
						func(t domain.Task) any { return t.ID },
						func(t domain.Task) ui.Node {
							due := ""
							overdue := false
							if !t.Due.IsZero() {
								due = pr.FormatDate(t.Due)
								if t.Status != domain.StatusDone && dateutil.FormatDate(t.Due) < todayISO {
									overdue = true
								}
							}
							// The one-click advance is offered ONLY when grouping by status,
							// where "advance" means the safe, additive To-do → Done move. In
							// priority grouping the next column is a LOWER priority, so a generic
							// "advance" would silently demote a meaningful field — there the board
							// is a read/triage view and priority changes go through the editor.
							nk := ""
							if hasNext && props.By == taskboard.GroupByStatus {
								nk = nextKey
							}
							return ui.CreateElement(BoardCard, boardCardProps{
								Task: t, DueLabel: due, Overdue: overdue,
								NextKey: nk, NextTitle: nextTitle,
								OnOpen: props.OnOpen, OnMove: props.OnMove,
							})
						},
					)
					body = Div(css.Class("tdb-col-body"), cards)
				}

				return Div(css.Class("tdb-col"),
					Attr("data-testid", "todo-board-col-"+c.Key), Attr("role", "listitem"),
					Div(css.Class("tdb-col-head"),
						Span(css.Class("tdb-col-title"), uistate.T(c.Title)),
						Span(css.Class("tdb-count"),
							Attr("aria-label", uistate.T("todoboard.countLabel", len(c.Tasks))),
							uistate.T("todoboard.count", len(c.Tasks)),
						),
					),
					body,
				)
			},
		),
	)
}

// columnTitleForKey returns the i18n title key of the column with the given key,
// or "" when no column matches (defensive — NextKey only yields real keys).
func columnTitleForKey(cols []taskboard.Column, key string) string {
	for _, c := range cols {
		if c.Key == key {
			return c.Title
		}
	}
	return ""
}

// boardCardProps drives a single task card on the board.
type boardCardProps struct {
	Task     domain.Task
	DueLabel string // pre-formatted due date, "" when the task has none
	Overdue  bool
	// NextKey is the column key the card's one-click "Next" advances to; "" hides
	// the affordance (the card is already in the last column).
	NextKey   string
	NextTitle string // resolved title of the next column, for the affordance tooltip
	OnOpen    func(string)
	OnMove    func(string, string)
}

// BoardCard renders one task card: a priority dot, the title, a due chip, and a
// one-click "Next" advance affordance. It owns its own click hooks so the parent
// board can Map over a variable number of cards without registering On* in a loop.
func BoardCard(props boardCardProps) ui.Node {
	t := props.Task
	done := t.Status == domain.StatusDone

	open := ui.UseEvent(Prevent(func() {
		if props.OnOpen != nil {
			props.OnOpen(t.ID)
		}
	}))
	// Advancing must not also open the task, so the move handler stops the click
	// from bubbling to the card's open handler.
	move := ui.UseEvent(func(e ui.Event) {
		e.PreventDefault()
		e.StopPropagation()
		if props.OnMove != nil && props.NextKey != "" {
			props.OnMove(t.ID, props.NextKey)
		}
	})

	cardCls := "tdb-card"
	if done {
		cardCls += " is-done"
	}

	var dueNode ui.Node = Fragment()
	if props.DueLabel != "" {
		dcls := "tdb-due"
		if props.Overdue {
			dcls += " is-overdue"
		}
		dueNode = Span(ClassStr(dcls), props.DueLabel)
	}

	// C368: a recurring task carries a repeat glyph on the board too (list/board
	// parity) so its auto-respawning nature is legible without opening the editor.
	// A positive reminder lead adds a bell. Icon-only with a tooltip + aria-label
	// (colour is never the only cue).
	var metaNode ui.Node = Fragment()
	var metaKids []ui.Node
	if t.Recurrence != "" {
		metaKids = append(metaKids, Span(ClassStr("tdb-meta-glyph"), Attr("data-testid", "tdb-recur-"+t.ID),
			Attr("role", "img"), Attr("aria-label", taskCadenceLabel(t.Recurrence)), Title(taskCadenceLabel(t.Recurrence)),
			uiw.Icon(icon.Refresh, css.Class(tw.ShrinkO, tw.W35, tw.H35))))
	}
	if t.ReminderLeadDays > 0 {
		metaKids = append(metaKids, Span(ClassStr("tdb-meta-glyph"), Attr("data-testid", "tdb-remind-"+t.ID),
			Attr("role", "img"), Attr("aria-label", taskReminderLabel(t.ReminderLeadDays)), Title(taskReminderLabel(t.ReminderLeadDays)),
			uiw.Icon(icon.Bell, css.Class(tw.ShrinkO, tw.W35, tw.H35))))
	}
	if len(metaKids) > 0 {
		metaArgs := []any{css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap1, tw.TextFaint)}
		for _, k := range metaKids {
			metaArgs = append(metaArgs, k)
		}
		metaNode = Span(metaArgs...)
	}

	// The advance affordance is status-only now (To-do → Done), so it reads as an
	// explicit "mark done": a check glyph + the target-naming tooltip.
	var nextNode ui.Node = Fragment()
	if props.NextKey != "" {
		nextNode = Button(css.Class("tdb-next"), Type("button"),
			Attr("data-testid", "todo-board-next-"+t.ID),
			Title(uistate.T("todoboard.nextTitle", props.NextTitle)),
			Attr("aria-label", uistate.T("todoboard.nextTitle", props.NextTitle)),
			OnClick(move),
			uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Span(uistate.T("todoboard.next")),
		)
	}

	return Div(ClassStr(cardCls),
		Attr("data-testid", "todo-board-card-"+t.ID),
		Attr("data-prio", string(t.Priority)),
		Attr("role", "button"), Attr("tabindex", "0"),
		Attr("aria-label", t.Title),
		OnClick(open),
		Div(css.Class("tdb-card-top"),
			Span(css.Class("tdb-prio-dot"), Attr("data-prio", string(t.Priority)), Attr("aria-hidden", "true")),
			Span(css.Class("tdb-card-title"), t.Title),
		),
		Div(css.Class("tdb-card-foot"),
			metaNode,
			dueNode,
			nextNode,
		),
	)
}

// boardEmptyColumnProps identifies which lane the add would land in.
type boardEmptyColumnProps struct {
	ColumnKey string
	By        taskboard.GroupBy
}

// boardEmptyColumn renders an empty lane's message with an add affordance
// (DP-F5a).
//
// It is a component, not inline markup, for the reason the ticket anticipates:
// the button owns a click hook, and this renders inside a MapKeyed over columns
// where registering one would violate the hook-position rule.
//
// The add SEEDS the new task with the lane it was opened from, so dropping a
// card into "High" by adding it there does not then require moving it. An add
// button that ignores which column it was pressed in is a shortcut that saves no
// steps.
func boardEmptyColumn(props boardEmptyColumnProps) ui.Node {
	add := ui.UseEvent(Prevent(func() {
		seed := uistate.TaskAddSeed{}
		switch props.By {
		case taskboard.GroupByPriority:
			seed.Priority = props.ColumnKey
		case taskboard.GroupByStatus:
			// A "Done" lane is the one column where seeding the status would be
			// perverse — nobody adds a task in order to have already finished it — so
			// the status seed is deliberately not set and the task lands open.
		}
		uistate.SetTaskAddSeed(seed)
		uistate.SetAddTarget("task")
	}))
	return Div(css.Class("tdb-empty"),
		Span(uistate.T("todoboard.emptyColumn")),
		Button(css.Class("btn-link", tw.Text12), Type("button"),
			Attr("data-testid", "tdb-add-"+props.ColumnKey),
			Attr("aria-label", uistate.T("todoboard.addHereAria")),
			OnClick(add), uistate.T("todoboard.addHere")),
	)
}
