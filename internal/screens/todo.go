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

	// Build the row deep-link node (goLink + linkRoute already declared above).
	var linkNode ui.Node
	if linkRoute != "" {
		name, ok := tasklink.EntityName(t.RelatedType, t.RelatedID,
			props.Accounts, props.Budgets, props.Goals, props.Transactions)
		if ok {
			linkLabel := "→ " + name
			ariaLabel := uistate.T("todo.linkAriaLabel", name)
			linkNode = Button(css.Class("btn row-meta"), Type("button"),
				Attr("aria-label", ariaLabel), Title(ariaLabel), OnClick(goLink), linkLabel)
		} else if t.RelatedID != "" {
			// Entity was deleted — show a muted note rather than nothing.
			linkNode = Span(css.Class("row-meta text-muted"), uistate.T("todo.linkRemoved"))
		}
	}

	done := t.Status == domain.StatusDone
	rowClass := "row"
	glyph := "☐"
	if done {
		rowClass = "row done"
		glyph = "☑"
	}
	plabel, pclass := priorityMeta(t.Priority)

	// Overdue = open task whose due date is before today. Due-today = open task
	// due on today's date. Flag overdue with danger tone + word "overdue" (colour
	// + text, not colour alone — B15). Flag due-today with warning tone + "due today"
	// so Nina can distinguish time-sensitive-now from comfortably-future (G6 D4).
	todayISO := dateutil.FormatDate(time.Now())
	overdue := !done && !t.Due.IsZero() && dateutil.FormatDate(t.Due) < todayISO
	dueToday := !done && !overdue && !t.Due.IsZero() && dateutil.FormatDate(t.Due) == todayISO
	meta := []ui.Node{Span(ClassStr("badge badge-prio "+pclass), plabel)}
	if !t.Due.IsZero() {
		var dueText string
		dueCls := "row-meta"
		switch {
		case overdue:
			// Keep the existing "· overdue" literal so the text-down style and word
			// are unchanged for the danger cue (C52 regression anchor).
			dueText = uistate.T("todo.due") + " " + pr.FormatDate(t.Due) + " · overdue"
			dueCls = "row-meta text-down"
		case dueToday:
			// G6 D4: amber "due today" — distinct from both overdue (red) and future (neutral).
			dueText = uistate.T("todo.dueToday")
			dueCls = "row-meta text-warn"
		default:
			dueText = uistate.T("todo.due") + " " + pr.FormatDate(t.Due)
		}
		meta = append(meta, Span(ClassStr(dueCls), dueText))
	}
	if t.Notes != "" {
		// Truncate long notes inline and expose the full text in a tooltip so the
		// row stays scannable without cutting off information (C52).
		const maxNoteRune = 80
		noteDisplay := t.Notes
		if len([]rune(noteDisplay)) > maxNoteRune {
			noteDisplay = string([]rune(noteDisplay)[:maxNoteRune]) + "…"
		}
		meta = append(meta, Span(css.Class("row-meta"), Title(t.Notes), noteDisplay))
	}
	if t.Recurrence != "" {
		recurLabel := taskCadenceLabel(t.Recurrence)
		meta = append(meta, Span(ClassStr("row-meta badge badge-recur"), Attr("data-testid", "recur-badge-"+t.ID),
			uistate.T("todo.recurBadge", recurLabel)))
	}
	if linkNode != nil {
		meta = append(meta, linkNode)
	}

	if props.Depth > 0 {
		rowClass += " subtask"
	}
	rowArgs := []any{ClassStr(rowClass), Attr("id", t.ID)}
	if props.Depth > 0 {
		rowArgs = append(rowArgs, Style(map[string]string{"margin-left": strconv.Itoa(props.Depth*22) + "px"}))
	}
	rowArgs = append(rowArgs,
		Button(css.Class("check"), Type("button"), Title(uistate.T("todo.toggle")), OnClick(toggle), glyph),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), t.Title),
			Div(css.Class("task-meta"), meta),
		),
		// Edit opens the flip modal; the ⋯ menu holds Add sub-task + the destructive Delete.
		Button(css.Class("btn row-2nd", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "task-edit-btn-"+t.ID), Title(uistate.T("todo.editTitle")), OnClick(openEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		Div(css.Class("add-wrap row-2nd"), Attr("id", menuID),
			Button(css.Class("btn"), Type("button"), Attr("title", uistate.T("todo.moreActions")), Attr("aria-label", uistate.T("todo.moreActions")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(menuOpen.Get())), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
			Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
			Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "task-addsub-"+t.ID), OnClick(addSub), uistate.T("todo.addSub")),
				Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "task-delete-btn-"+t.ID), Attr("aria-label", uistate.T("todo.deleteTitle")), Title(uistate.T("todo.deleteTitle")), OnClick(del), uistate.T("action.delete")),
			),
		),
	)
	return Div(rowArgs...)
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
