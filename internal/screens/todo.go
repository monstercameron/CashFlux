// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/tasklink"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

type taskRowProps struct {
	Task         domain.Task
	Depth        int // nesting depth (0 = top level) → indentation (C72)
	OnToggle     func(string)
	OnDelete     func(string)
	OnAddSub     func(parentID string)
	Accounts     []domain.Account
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Transactions []domain.Transaction
	// Sub-task summary + collapse (parents only; ChildTotal 0 = leaf). Counts are over
	// ALL children regardless of the current view filter.
	ChildTotal       int
	ChildDone        int
	Collapsed        bool
	OnToggleCollapse func(string)
	// Drag-to-reorder (only in the "Custom order" sort). Draggable shows a grip handle and
	// makes the row a drop target; OnDragStart marks this row as the one being dragged and
	// OnDrop asks the list to move the dragged row into this row's slot.
	Draggable   bool
	OnDragStart func()
	OnDrop      func()
}

// TaskRow renders one task with complete/edit/delete. It can be edited inline
// (title, priority, due, notes, entity link). All hooks are declared
// unconditionally so the edit toggle never reorders them.
func TaskRow(props taskRowProps) ui.Node {
	t := props.Task

	nav := router.UseNavigate()
	toggle := ui.UseEvent(Prevent(func() { props.OnToggle(t.ID) }))
	del := ui.UseEvent(Prevent(func() { props.OnDelete(t.ID) }))
	addSub := ui.UseEvent(Prevent(func() {
		if props.OnAddSub != nil {
			props.OnAddSub(t.ID)
		}
	}))
	pr := uistate.UsePrefs().Get()
	// Edit opens the shell-root flip modal (TaskEditHost) instead of an inline row form —
	// the row lives under transformed tile ancestors, so an in-row modal would be off-centre.
	openEdit := ui.UseEvent(Prevent(func() { uistate.SetTaskEdit(uistate.TaskEdit{ID: t.ID}) }))
	toggleCollapse := ui.UseEvent(Prevent(func() {
		if props.OnToggleCollapse != nil {
			props.OnToggleCollapse(t.ID)
		}
	}))
	// Row-display deep-link: declared unconditionally here so the hook slot never
	// shifts across renders (framework rule).
	linkRoute := tasklink.Route(t.RelatedType)
	goLink := ui.UseEvent(Prevent(func() {
		nav.Navigate(uistate.RoutePath(linkRoute))
		// The aggregate review task's population IS the guided Review inbox
		// (UX-10) — open it on arrival so the link lands on the exact filtered
		// set the task names, not the full ledger.
		if t.RelatedType == domain.RelatedReviewQueue {
			uistate.OpenReviewInbox()
		}
	}))
	// Drag-to-reorder hooks (declared unconditionally; wired into the row only when
	// props.Draggable, i.e. the "Custom order" sort is active).
	dragStart := ui.UseEvent(func() {
		if props.OnDragStart != nil {
			props.OnDragStart()
		}
	})
	dragOver := ui.UseEvent(Prevent(func() {}))
	drop := ui.UseEvent(Prevent(func() {
		if props.OnDrop != nil {
			props.OnDrop()
		}
	}))

	// Linked entity → a quiet inline text-link on the meta line (leading per-type icon).
	// A linked goal is accent-toned so the goal↔to-do connection stands out among the
	// otherwise dim metadata; account/budget/txn stay neutral.
	var linkNode ui.Node = Fragment()
	linkPresent := false
	if linkRoute != "" {
		name, ok := tasklink.EntityName(t.RelatedType, t.RelatedID,
			props.Accounts, props.Budgets, props.Goals, props.Transactions)
		if ok {
			ariaLabel := uistate.T("todo.linkAriaLabel", name)
			linkNode = Button(css.Class("todo-link "+linkChipClass(t.RelatedType)), Type("button"),
				Attr("data-testid", "task-link-"+t.ID), Attr("aria-label", ariaLabel), Title(ariaLabel), OnClick(goLink),
				uiw.Icon(linkTypeIcon(t.RelatedType), css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(name))
			linkPresent = true
		} else if t.RelatedID != "" {
			linkNode = Span(css.Class("todo-meta-note"), uistate.T("todo.linkRemoved"))
			linkPresent = true
		}
	}

	done := t.Status == domain.StatusDone

	// PRIORITY reads as a small neutral tag on the headline (High / Low; Medium is the
	// quiet default). It is deliberately NOT colour-coded: the checkbox ring is reserved
	// for the done state (fills green when complete) and red is reserved for "Overdue", so
	// encoding priority in red/green too would put contradictory signals in one row (a
	// green "looks-done" ring beside red "Overdue" text). Kept off the red/green scale.
	itemClass := "todo-item"
	if done {
		itemClass += " is-done"
	}
	if props.Depth > 0 {
		itemClass += " is-subtask"
	}

	// Due emphasis: overdue → red "Overdue · <date>", today → amber "Today", else a quiet
	// right-aligned date. (Colour + word, not colour alone — B15.)
	todayISO := dateutil.FormatDate(time.Now())
	overdue := !done && !t.Due.IsZero() && dateutil.FormatDate(t.Due) < todayISO
	dueToday := !done && !overdue && !t.Due.IsZero() && dateutil.FormatDate(t.Due) == todayISO
	var dueNode ui.Node = Fragment()
	if !t.Due.IsZero() {
		switch {
		case overdue:
			dueNode = Span(css.Class("todo-due is-overdue"), uistate.T("todo.overdueLabel")+" · "+pr.FormatDate(t.Due))
		case dueToday:
			dueNode = Span(css.Class("todo-due is-today"), uistate.T("todo.dueTodayShort"))
		default:
			dueNode = Span(css.Class("todo-due"), pr.FormatDate(t.Due))
		}
	}

	// Secondary meta line (repeat · linked entity · notes) — quiet, middot-separated,
	// rendered only when there's something to show.
	var metaParts []ui.Node
	if t.Recurrence != "" {
		metaParts = append(metaParts, Span(css.Class("todo-meta-item"), Attr("data-testid", "recur-badge-"+t.ID),
			uiw.Icon(icon.Refresh, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(taskCadenceLabel(t.Recurrence))))
	}
	// Reminder chip — only when a positive lead is set (0 = on the due date is the
	// quiet default and gets no chip, keeping the row clean).
	if t.ReminderLeadDays > 0 {
		metaParts = append(metaParts, Span(css.Class("todo-meta-item", "is-reminder"), Attr("data-testid", "remind-badge-"+t.ID),
			uiw.Icon(icon.Bell, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(taskReminderLabel(t.ReminderLeadDays))))
	}
	// Assignee chip — a task claimed by one member says whose it is.
	if t.MemberID != "" {
		if app := appstate.Default; app != nil {
			for _, m := range app.Members() {
				if m.ID == t.MemberID {
					metaParts = append(metaParts, Span(css.Class("todo-meta-item"), Attr("data-testid", "assignee-badge-"+t.ID),
						uiw.Icon(icon.Users, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(m.Name)))
					break
				}
			}
		}
	}
	if linkPresent {
		metaParts = append(metaParts, linkNode)
	}
	if t.Notes != "" {
		// The note is single-line clamped in the row (CSS). Keep a generous DOM cap so an
		// expanded note reveals meaningful text; only a truly huge note gets an ellipsis.
		const maxNoteRune = 280
		runes := []rune(t.Notes)
		noteDisplay := t.Notes
		if len(runes) > maxNoteRune {
			noteDisplay = string(runes[:maxNoteRune]) + "…"
		}
		// A note long enough to be clamped by the row's single-line width earns the
		// expand affordance: a resting dotted underline + hover/keyboard-focus reveal
		// (CSS .todo-meta-note.is-expandable). Short notes that already fit stay plain,
		// so the cue only appears where there's actually more to read.
		noteCls := "todo-meta-note"
		// The full note is always the tooltip; a clamped note also becomes keyboard-
		// focusable so the hover/focus expand works from the keyboard too.
		noteArgs := []any{Title(t.Notes)}
		if len(runes) > 46 {
			noteCls += " is-expandable"
			noteArgs = append(noteArgs, Attr("tabindex", "0"), Attr("aria-label", uistate.T("todo.noteExpandHint")))
		}
		noteArgs = append([]any{css.Class(noteCls)}, noteArgs...)
		noteArgs = append(noteArgs, noteDisplay)
		metaParts = append(metaParts, Span(noteArgs...))
	}
	// Sub-task summary chip (parents only): "N/M" done, leading the meta line — and, when
	// collapsed, the only hint that hidden work lives under this row.
	hasKids := props.ChildTotal > 0
	if hasKids {
		substat := Span(css.Class("todo-substat"), Attr("data-testid", "task-substat-"+t.ID),
			Title(uistate.T("todo.subCountTitle", props.ChildDone, props.ChildTotal)),
			uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Span(fmt.Sprintf("%d/%d", props.ChildDone, props.ChildTotal)))
		metaParts = append([]ui.Node{substat}, metaParts...)
	}
	var metaLine ui.Node = Fragment()
	if len(metaParts) > 0 {
		metaChildren := make([]ui.Node, 0, len(metaParts)*2)
		for i, part := range metaParts {
			if i > 0 {
				metaChildren = append(metaChildren, Span(css.Class("todo-sep"), "·"))
			}
			metaChildren = append(metaChildren, part)
		}
		metaLine = Div(css.Class("todo-meta"), metaChildren)
	}

	// The check-off ritual: a circular ring in the priority colour; on done it fills
	// accent-green with a check that pops in.
	var checkGlyph ui.Node = Fragment()
	if done {
		checkGlyph = uiw.Icon(icon.Check, css.Class(tw.W35, tw.H35))
	}

	rowArgs := []any{ClassStr(itemClass), Attr("id", t.ID), Attr("data-testid", "task-card"), Attr("data-prio", string(t.Priority))}
	if props.Depth > 0 {
		// Indent via padding (border-box) — NOT margin-left, which pushes the full-width
		// row past its container and scrolls the whole page sideways.
		rowArgs = append(rowArgs, Style(map[string]string{"padding-left": strconv.Itoa(12+props.Depth*24) + "px"}))
	}
	// Drag-to-reorder: the row is a drop target and leads with a grip handle you pick up.
	// Only in "Custom order" — otherwise the algorithmic sort owns the order.
	if props.Draggable {
		rowArgs = append(rowArgs, OnDragOver(dragOver), OnDrop(drop),
			Span(css.Class("todo-grip"), Attr("draggable", "true"), Attr("role", "button"),
				Attr("data-testid", "task-grip-"+t.ID), Attr("aria-label", uistate.T("todo.dragReorder")),
				Title(uistate.T("todo.dragReorder")), OnDragStart(dragStart),
				uiw.Icon(icon.Grip, css.Class(tw.W35, tw.H35))))
	}
	// A leading connector glyph makes a nested sub-task unmistakable as a child of the
	// row above it (paired with the indent + left guide rail in CSS).
	if props.Depth > 0 {
		rowArgs = append(rowArgs, Span(css.Class("todo-subarrow"), Attr("aria-hidden", "true"), "↳"))
	}
	// Disclosure: parents get a chevron that collapses/expands their sub-tasks; leaves get
	// an equal-width spacer so every checkbox stays aligned.
	if hasKids {
		discloseCls := "todo-disclose"
		if props.Collapsed {
			discloseCls += " is-collapsed"
		}
		lbl := uistate.T("todo.collapse")
		if props.Collapsed {
			lbl = uistate.T("todo.expand")
		}
		rowArgs = append(rowArgs, Button(ClassStr(discloseCls), Type("button"),
			Attr("aria-label", lbl), Attr("aria-expanded", ariaBool(!props.Collapsed)),
			Attr("data-testid", "task-collapse-"+t.ID), Title(lbl), OnClick(toggleCollapse),
			uiw.Icon(icon.ChevronRight, css.Class(tw.W35, tw.H35))))
	} else {
		rowArgs = append(rowArgs, Span(css.Class("todo-disclose-spacer"), Attr("aria-hidden", "true")))
	}
	rowArgs = append(rowArgs,
		Button(ClassStr("todo-check"+map[bool]string{true: " is-done", false: ""}[done]), Type("button"),
			Attr("role", "checkbox"), Attr("aria-checked", ariaBool(done)),
			Attr("aria-label", uistate.T("todo.toggle")+" — "+t.Title), Attr("data-testid", "task-check-"+t.ID),
			Title(uistate.T("todo.toggle")), OnClick(toggle), checkGlyph),
		Div(css.Class("todo-main"),
			Div(css.Class("todo-headline"),
				Div(css.Class("todo-headline-lead"),
					// A fixed-width slot so every title starts at the same x whether or not the
					// row carries a High/Low priority tag (Medium leaves the slot empty) — no
					// ragged left edge down the list.
					Span(css.Class("todo-prio-slot"), todoPriorityTag(t.Priority)),
					Span(css.Class("todo-title"), t.Title),
				),
				dueNode,
			),
			metaLine,
		),
		// Edit opens the flip modal; the shared ⋯ KebabMenu (viewport-aware) holds Add
		// sub-task + the destructive Delete.
		Div(css.Class("todo-actions"),
			Button(css.Class("todo-icon-btn"), Type("button"), Attr("data-testid", "task-edit-btn-"+t.ID), Attr("aria-label", uistate.T("todo.editTitle")+" — "+t.Title), Title(uistate.T("todo.editTitle")), OnClick(openEdit), uiw.Icon(icon.Pencil, css.Class(tw.W4, tw.H4))),
			uiw.KebabMenu(uiw.KebabMenuProps{
				ID:           "task-menu-" + t.ID,
				AriaLabel:    uistate.T("todo.moreActions") + " — " + t.Title,
				ToggleTestID: "task-menu-btn-" + t.ID,
				ToggleClass:  "todo-icon-btn",
				Items: []ui.Node{
					Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "task-addsub-"+t.ID), OnClick(addSub), uistate.T("todo.addSub")),
					Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "task-delete-btn-"+t.ID), Attr("aria-label", uistate.T("todo.deleteTitle")), Title(uistate.T("todo.deleteTitle")), OnClick(del), uistate.T("action.delete")),
				},
			}),
		),
	)
	return Div(rowArgs...)
}

// linkTypeIcon maps a task's linked-entity type to its glyph for the link chip.
func linkTypeIcon(rt domain.RelatedType) icon.Name {
	switch rt {
	case domain.RelatedGoal:
		return icon.Goals
	case domain.RelatedAccount:
		return icon.Accounts
	case domain.RelatedBudget:
		return icon.Budgets
	case domain.RelatedTransaction:
		return icon.Transactions
	default:
		return icon.Paperclip
	}
}

// linkChipClass returns the tint modifier for a task's link chip — a goal link is
// accent-tinted so the goal↔to-do connection stands out from account/budget/txn links.
func linkChipClass(rt domain.RelatedType) string {
	switch rt {
	case domain.RelatedGoal:
		return "is-goal"
	case domain.RelatedAccount:
		return "is-account"
	case domain.RelatedBudget:
		return "is-budget"
	case domain.RelatedTransaction:
		return "is-txn"
	default:
		return ""
	}
}

// todoPriorityTag renders the neutral priority marker shown on the row headline: a
// quiet uppercase "High"/"Low" tag (Medium — the default — gets none, keeping most
// rows clean). It carries no colour on the red/green scale so it never contradicts the
// overdue-red / done-green signals; the checkbox's aria-label already announces the
// priority, so the tag is aria-hidden to avoid double-reading.
func todoPriorityTag(p domain.TaskPriority) ui.Node {
	switch p {
	case domain.PriorityHigh:
		return Span(css.Class("todo-prio is-high"), Attr("aria-hidden", "true"), uistate.T("priority.high"))
	case domain.PriorityLow:
		return Span(css.Class("todo-prio is-low"), Attr("aria-hidden", "true"), uistate.T("priority.low"))
	default:
		return Fragment()
	}
}

func priorityMeta(p domain.TaskPriority) (label, class string) {
	switch p {
	case domain.PriorityHigh:
		return uistate.T("priority.high"), "prio-high"
	case domain.PriorityLow:
		return uistate.T("priority.low"), "prio-low"
	default:
		return uistate.T("priority.medium"), "prio-med"
	}
}

// taskCadenceLabel returns the human-readable label for a RecurringCadence
// value, used in the recurring-task add/edit selects and the row badge.
func taskCadenceLabel(c domain.RecurringCadence) string {
	switch c {
	case domain.CadenceWeekly:
		return uistate.T("todo.repeatWeekly")
	case domain.CadenceMonthly:
		return uistate.T("todo.repeatMonthly")
	case domain.CadenceQuarterly:
		return uistate.T("todo.repeatQuarterly")
	case domain.CadenceYearly:
		return uistate.T("todo.repeatYearly")
	default:
		return string(c)
	}
}

// taskReminderLabel returns the human-readable lead label for a recurring task's
// ReminderLeadDays, used in the add-form summary and the row reminder chip. Known
// leads (1 / 3 / 7 days) map to their friendly phrasing; any other positive value
// falls back to an "N days early" form. 0 (remind on the due date) returns "".
func taskReminderLabel(days int) string {
	switch {
	case days <= 0:
		return ""
	case days == 1:
		return uistate.T("todo.remind1Day")
	case days == 3:
		return uistate.T("todo.remind3Days")
	case days == 7:
		return uistate.T("todo.remind1Week")
	default:
		return uistate.T("todo.reminderBadgeLead", fmt.Sprintf("%dd", days))
	}
}
