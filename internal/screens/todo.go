// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"time"

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
	goLink := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath(linkRoute)) }))

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
	plabel, _ := priorityMeta(t.Priority)

	// The signature move: PRIORITY is encoded in the checkbox RING colour (red high /
	// accent medium / faint low) rather than a badge — you scan urgency by the rings.
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
	if linkPresent {
		metaParts = append(metaParts, linkNode)
	}
	if t.Notes != "" {
		const maxNoteRune = 100
		noteDisplay := t.Notes
		if len([]rune(noteDisplay)) > maxNoteRune {
			noteDisplay = string([]rune(noteDisplay)[:maxNoteRune]) + "…"
		}
		metaParts = append(metaParts, Span(css.Class("todo-meta-note"), Title(t.Notes), noteDisplay))
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
		Button(ClassStr("todo-check p-"+string(t.Priority)+map[bool]string{true: " is-done", false: ""}[done]), Type("button"),
			Attr("role", "checkbox"), Attr("aria-checked", ariaBool(done)),
			Attr("aria-label", uistate.T("todo.toggle")+" — "+plabel), Attr("data-testid", "task-check-"+t.ID),
			Title(uistate.T("todo.toggle")), OnClick(toggle), checkGlyph),
		Div(css.Class("todo-main"),
			Div(css.Class("todo-headline"),
				Span(css.Class("todo-title"), t.Title),
				dueNode,
			),
			metaLine,
		),
		// Edit opens the flip modal; the shared ⋯ KebabMenu (viewport-aware) holds Add
		// sub-task + the destructive Delete.
		Div(css.Class("todo-actions"),
			Button(css.Class("todo-icon-btn"), Type("button"), Attr("data-testid", "task-edit-btn-"+t.ID), Attr("aria-label", uistate.T("todo.editTitle")), Title(uistate.T("todo.editTitle")), OnClick(openEdit), uiw.Icon(icon.Pencil, css.Class(tw.W4, tw.H4))),
			uiw.KebabMenu(uiw.KebabMenuProps{
				ID:           "task-menu-" + t.ID,
				AriaLabel:    uistate.T("todo.moreActions"),
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
