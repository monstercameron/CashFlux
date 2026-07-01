// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/tasklink"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
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
	// The ⋯ actions menu (add sub-task + the destructive delete), so the row stays
	// uncluttered and a misclick can't delete a task (and its whole sub-tree).
	menuID := "task-menu-" + t.ID
	menuOpen := ui.UseState(false)
	toggleMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(!menuOpen.Get()) }))
	closeMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(false) }))
	menuHidden := ""
	if !menuOpen.Get() {
		menuHidden = " hidden-menu"
	}

	// Row-display deep-link: declared unconditionally here so the hook slot never
	// shifts across renders (framework rule).
	linkRoute := tasklink.Route(t.RelatedType)
	goLink := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath(linkRoute)) }))

	// Build the row deep-link chip (goLink + linkRoute already declared above). The
	// linked entity is a first-class chip with a per-type icon; a goal link is accent-
	// tinted so the goal↔to-do connection reads at a glance.
	var linkNode ui.Node = Fragment()
	if linkRoute != "" {
		name, ok := tasklink.EntityName(t.RelatedType, t.RelatedID,
			props.Accounts, props.Budgets, props.Goals, props.Transactions)
		if ok {
			ariaLabel := uistate.T("todo.linkAriaLabel", name)
			linkNode = Button(css.Class("task-link-chip "+linkChipClass(t.RelatedType)), Type("button"),
				Attr("data-testid", "task-link-"+t.ID), Attr("aria-label", ariaLabel), Title(ariaLabel), OnClick(goLink),
				uiw.Icon(linkTypeIcon(t.RelatedType), css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(name))
		} else if t.RelatedID != "" {
			// Entity was deleted — show a muted note rather than nothing.
			linkNode = Span(css.Class("task-chip is-muted"), uistate.T("todo.linkRemoved"))
		}
	}

	done := t.Status == domain.StatusDone
	plabel, pclass := priorityMeta(t.Priority)
	cardClass := "task-card tp-" + string(t.Priority)
	if done {
		cardClass += " is-done"
	}
	if props.Depth > 0 {
		cardClass += " is-subtask"
	}

	// Overdue = open task whose due date is before today. Due-today = open task due on
	// today's date. Colour + word (not colour alone — B15): red "overdue" / amber
	// "due today" / neutral future date.
	todayISO := dateutil.FormatDate(time.Now())
	overdue := !done && !t.Due.IsZero() && dateutil.FormatDate(t.Due) < todayISO
	dueToday := !done && !overdue && !t.Due.IsZero() && dateutil.FormatDate(t.Due) == todayISO
	chips := []ui.Node{Span(ClassStr("task-chip badge-prio "+pclass), plabel)}
	if !t.Due.IsZero() {
		var dueText, dueCls string
		switch {
		case overdue:
			dueText, dueCls = uistate.T("todo.due")+" "+pr.FormatDate(t.Due)+" · overdue", "task-chip is-overdue"
		case dueToday:
			dueText, dueCls = uistate.T("todo.dueToday"), "task-chip is-today"
		default:
			dueText, dueCls = uistate.T("todo.due")+" "+pr.FormatDate(t.Due), "task-chip"
		}
		chips = append(chips, Span(ClassStr(dueCls), dueText))
	}
	if t.Recurrence != "" {
		chips = append(chips, Span(ClassStr("task-chip is-recur"), Attr("data-testid", "recur-badge-"+t.ID),
			uistate.T("todo.recurBadge", taskCadenceLabel(t.Recurrence))))
	}
	if linkRoute != "" {
		chips = append(chips, linkNode)
	}
	if t.Notes != "" {
		const maxNoteRune = 90
		noteDisplay := t.Notes
		if len([]rune(noteDisplay)) > maxNoteRune {
			noteDisplay = string([]rune(noteDisplay)[:maxNoteRune]) + "…"
		}
		chips = append(chips, Span(css.Class("task-chip is-note"), Title(t.Notes), noteDisplay))
	}

	rowArgs := []any{ClassStr(cardClass), Attr("id", t.ID), Attr("data-testid", "task-card")}
	if props.Depth > 0 {
		rowArgs = append(rowArgs, Style(map[string]string{"margin-left": strconv.Itoa(props.Depth*24) + "px"}))
	}
	// The custom checkbox: an empty rounded box when open, an accent-filled check when done.
	var checkGlyph ui.Node = Fragment()
	if done {
		checkGlyph = uiw.Icon(icon.Check, css.Class(tw.W35, tw.H35))
	}
	rowArgs = append(rowArgs,
		Button(ClassStr("task-check"+map[bool]string{true: " done", false: ""}[done]), Type("button"),
			Attr("role", "checkbox"), Attr("aria-checked", ariaBool(done)),
			Attr("data-testid", "task-check-"+t.ID), Title(uistate.T("todo.toggle")), OnClick(toggle), checkGlyph),
		Div(css.Class("task-body"),
			Span(css.Class("task-title"), t.Title),
			Div(css.Class("task-chips"), chips),
		),
		// Edit opens the flip modal; the ⋯ menu holds Add sub-task + the destructive Delete.
		Div(css.Class("task-actions"),
			Button(css.Class("btn row-2nd", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "task-edit-btn-"+t.ID), Title(uistate.T("todo.editTitle")), OnClick(openEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
			Div(css.Class("add-wrap"), Attr("id", menuID),
				Button(css.Class("btn"), Type("button"), Attr("title", uistate.T("todo.moreActions")), Attr("aria-label", uistate.T("todo.moreActions")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(menuOpen.Get())), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
				Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
				Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
					Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "task-addsub-"+t.ID), OnClick(addSub), uistate.T("todo.addSub")),
					Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "task-delete-btn-"+t.ID), Attr("aria-label", uistate.T("todo.deleteTitle")), Title(uistate.T("todo.deleteTitle")), OnClick(del), uistate.T("action.delete")),
				),
			),
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
