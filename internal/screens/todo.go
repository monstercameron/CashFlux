// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/tasklink"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	"github.com/monstercameron/CashFlux/internal/tasktree"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Todo is the budgeting task list: add, complete/reopen, delete.
func Todo() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	rev := state.UseAtom("rev:tasks", 0)
	bump := func() { rev.Set(rev.Get() + 1) }
	// Re-render after a whole-dataset replacement (undo/redo, import, decrypt) so
	// the list reflects external changes, not just this screen's own mutations.
	_ = uistate.UseDataRevision().Get()

	hideDone := ui.UseState(false)
	// filterPrio is "" (all), "high", "medium", or "low" (C52 lightweight priority
	// filter — lets the user focus on just one priority level without a full filter
	// panel).
	filterPrio := ui.UseState("")
	errMsg := ui.UseState("")

	toggleHideDone := ui.UseEvent(func() { hideDone.Set(!hideDone.Get()) })
	onFilterPrio := ui.UseEvent(func(e ui.Event) { filterPrio.Set(e.GetValue()) })
	// Open the add-task modal from the card header (G6: page-level add affordance,
	// instead of reaching for the global "+" far from the list).
	addTask := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("task") }))

	tasks := app.Tasks()
	accounts := app.Accounts()
	budgets := app.Budgets()
	goals := app.Goals()
	txns := app.Transactions()
	byID := make(map[string]domain.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}

	toggleTask := func(taskID string) {
		t, ok := byID[taskID]
		if !ok {
			return
		}
		if t.Status == domain.StatusDone {
			// Re-opening a done task: plain status flip, no spawn.
			t.Status = domain.StatusOpen
			if err := app.PutTask(t); err != nil {
				errMsg.Set(err.Error())
				return
			}
		} else {
			// Completing an open task: route through CompleteTask so recurring
			// tasks spawn their next occurrence atomically.
			if err := app.CompleteTask(taskID, id.New(), time.Now()); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		bump()
	}
	deleteTask := func(taskID string) {
		// Guard the destructive delete with a confirm (matches Transactions/Budgets). Deleting a task
		// also cascades to its whole sub-tree (C72), so an unconfirmed misclick was especially costly.
		name := uistate.T("todo.thisTask")
		for _, t := range tasks {
			if t.ID == taskID && t.Title != "" {
				name = t.Title
				break
			}
		}
		uistate.ConfirmModal(uistate.T("todo.deleteConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			// Cascade: deleting a task removes its whole sub-tree (C72).
			for _, d := range tasktree.Descendants(tasks, taskID) {
				_ = app.DeleteTask(d)
			}
			if err := app.DeleteTask(taskID); err != nil {
				errMsg.Set(err.Error())
				return
			}
			bump()
		})
	}
	addSub := func(parentID string) {
		uistate.PromptModal(uistate.T("todo.subtaskPrompt"), "", func(name string) {
			if name == "" {
				return
			}
			t := domain.Task{
				ID: id.New(), Title: name, ParentID: parentID,
				Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceManual,
			}
			if err := app.PutTask(t); err != nil {
				errMsg.Set(err.Error())
				return
			}
			bump()
		})
	}
	// Order + filter, then nest into the parent/child tree (C72). hide-done filters
	// first so a done parent's open child surfaces as a root (tasktree handles it).
	// The priority filter runs after the done filter so it cooperates with hide-done:
	// if you filter to "High" and hide done, only open high-priority tasks appear.
	filtered := tasksort.Visible(tasks, hideDone.Get())
	if p := domain.TaskPriority(filterPrio.Get()); p.Valid() {
		kept := filtered[:0:0]
		for _, t := range filtered {
			if t.Priority == p {
				kept = append(kept, t)
			}
		}
		filtered = kept
	}
	visible := filtered
	nodes := tasktree.Flatten(visible)

	var listBody ui.Node
	switch {
	case len(tasks) == 0:
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("todo.empty"), CTALabel: uistate.T("todo.addFirst"), AddTarget: "task"})
	case len(nodes) == 0:
		listBody = P(css.Class("empty"), uistate.T("todo.allDone"))
	default:
		rows := MapKeyed(nodes,
			func(n tasktree.Node) any { return n.Task.ID },
			func(n tasktree.Node) ui.Node {
				return ui.CreateElement(TaskRow, taskRowProps{
					Task: n.Task, Depth: n.Depth,
					OnToggle: toggleTask, OnDelete: deleteTask, OnAddSub: addSub,
					Accounts: accounts, Budgets: budgets, Goals: goals, Transactions: txns,
				})
			},
		)
		listBody = Div(css.Class("rows"), rows)
	}

	hideLabel := uistate.T("todo.hideDone")
	if hideDone.Get() {
		hideLabel = uistate.T("todo.showAll")
	}

	// Portfolio summary (G6): a compact open/overdue/done count above the list so
	// Nina gets the at-a-glance context every other list screen opens with. Counts
	// are over all tasks, independent of the current view filters.
	openCount, overdueCount, doneCount := 0, 0, 0
	today := dateutil.FormatDate(time.Now())
	for _, t := range tasks {
		if t.Status == domain.StatusDone {
			doneCount++
			continue
		}
		openCount++
		if !t.Due.IsZero() && dateutil.FormatDate(t.Due) < today {
			overdueCount++
		}
	}

	// G6: when "hide done" is active and there ARE completed tasks, show a small
	// muted acknowledgment so Nina knows she has completed work without seeing it —
	// mirrors the Transactions filter-summary pattern.
	var hiddenDoneNote ui.Node
	if hideDone.Get() && doneCount > 0 {
		hiddenDoneNote = P(css.Class("empty", tw.TextDim), uistate.T("todo.hiddenDone", doneCount))
	}

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("todo.listTitle"),
		HeaderAction: Div(css.Class(tw.Flex, tw.Gap2, tw.FlexWrap, tw.ItemsCenter),
			// Priority filter — lightweight selector; "" means show all (C52).
			Select(css.Class("field"), Attr("aria-label", uistate.T("todo.filterPrioLabel")),
				Attr("data-testid", "todo-filter-prio"), OnChange(onFilterPrio),
				Option(Value(""), SelectedIf(filterPrio.Get() == ""), uistate.T("todo.filterPrioAll")),
				Option(Value(string(domain.PriorityHigh)), SelectedIf(filterPrio.Get() == string(domain.PriorityHigh)), uistate.T("priority.high")),
				Option(Value(string(domain.PriorityMedium)), SelectedIf(filterPrio.Get() == string(domain.PriorityMedium)), uistate.T("priority.medium")),
				Option(Value(string(domain.PriorityLow)), SelectedIf(filterPrio.Get() == string(domain.PriorityLow)), uistate.T("priority.low")),
			),
			Button(css.Class("btn"), Type("button"), OnClick(toggleHideDone), hideLabel),
			Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "todo-add"), Title(uistate.T("todo.addFirst")), OnClick(addTask),
				uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("todo.addTask"))),
		),
		Body: Fragment(
			If(len(tasks) > 0, P(css.Class("todo-summary", tw.TextDim),
				Text(uistate.T("todo.summary", openCount, overdueCount, doneCount)))),
			If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
			listBody,
			hiddenDoneNote,
		),
	})
}

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
