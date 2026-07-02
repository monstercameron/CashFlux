// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	"github.com/monstercameron/CashFlux/internal/tasktree"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

type todoSummaryProps struct{ App *appstate.App }
type todoToolbarProps struct{ App *appstate.App }
type todoListProps struct{ App *appstate.App }

// todoCounts is the at-a-glance portfolio picture over ALL tasks (independent of the
// current view filters): how many are open, overdue, done, and the completion percent.
type todoCounts struct {
	Open, Overdue, Done, Total, DonePct int
}

// computeTodoCounts reduces the task list to the summary counts. Overdue = an open task
// whose due date is before today.
func computeTodoCounts(tasks []domain.Task, now time.Time) todoCounts {
	c := todoCounts{}
	today := dateutil.FormatDate(now)
	for _, t := range tasks {
		if t.Status == domain.StatusDone {
			c.Done++
			continue
		}
		c.Open++
		if !t.Due.IsZero() && dateutil.FormatDate(t.Due) < today {
			c.Overdue++
		}
	}
	c.Total = c.Open + c.Done
	if c.Total > 0 {
		c.DonePct = c.Done * 100 / c.Total
	}
	return c
}

// --- todo-summary ----------------------------------------------------------------

// todoSummaryWidget is the headline tile: a completion "loader" bar (done-of-total) with
// the Open / Overdue / Done figures rendered inside it, mirroring the goals summary.
// Renders nothing when there are no tasks (the list tile owns the empty CTA).
func todoSummaryWidget(props todoSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	tasks := props.App.Tasks()
	if len(tasks) == 0 {
		return Fragment()
	}
	c := computeTodoCounts(tasks, time.Now())

	overdueTone := "budget-loader-value"
	if c.Overdue > 0 {
		overdueTone = "budget-loader-value neg"
	}
	body := Div(ClassStr("budget-loader"),
		Attr("role", "progressbar"), Attr("aria-valuenow", fmt.Sprintf("%d", c.DonePct)),
		Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("todo.completionLabel")),
		Div(css.Class("budget-loader-fill"), Attr("style", fmt.Sprintf("width:%d%%", c.DonePct))),
		Div(css.Class("budget-loader-figs"),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("todo.openLabel")),
				Div(css.Class("budget-loader-value"), fmt.Sprintf("%d", c.Open)),
			),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("todo.overdueLabel")),
				Div(css.Class(overdueTone), fmt.Sprintf("%d", c.Overdue)),
			),
			Div(css.Class("budget-loader-fig", "is-right"),
				Div(css.Class("budget-loader-label"), uistate.T("todo.doneLabel")),
				Div(css.Class("budget-loader-value is-hero"), fmt.Sprintf("%d%%", c.DonePct)),
			),
		),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- todo-toolbar ----------------------------------------------------------------

// todoToolbarWidget is the actions row: a lightweight priority filter, a hide/show-done
// toggle, and the primary "Add task" button. The filter + hide-done state live in shared
// atoms (read by the list tile too), mirroring the budgets/goals toolbars.
func todoToolbarWidget(props todoToolbarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	hideDone := uistate.UseTodoHideDone()
	filterPrio := uistate.UseTodoFilterPrio()
	sortMode := uistate.UseTodoSortMode()
	page := uistate.UseTodoPage()

	// Changing the filter or sort resets to the first page so the view can't land on a
	// now-empty page.
	toggleHideDone := ui.UseEvent(Prevent(func() { hideDone.Set(!hideDone.Get()); page.Set(1) }))
	onFilterPrio := ui.UseEvent(func(e ui.Event) { filterPrio.Set(e.GetValue()); page.Set(1) })
	onSort := ui.UseEvent(func(e ui.Event) { sortMode.Set(e.GetValue()); page.Set(1) })
	addTask := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("task") }))

	hideLabel := uistate.T("todo.hideDone")
	if hideDone.Get() {
		hideLabel = uistate.T("todo.showAll")
	}
	sm := sortMode.Get()

	hideToggleCls := "strip-toggle"
	if hideDone.Get() {
		hideToggleCls += " is-on"
	}
	// A single compact filter strip: sort + priority filter as small labelled "pill"
	// selects (auto-width, not full-width bars), a hide-done toggle, and the primary
	// Add task pushed to the right. The strip layout is shared with /goals (.filter-strip).
	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Label(css.Class("todo-ctrl"),
				uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(css.Class("todo-ctrl-label"), uistate.T("todo.sortShort")),
				Select(css.Class("todo-select"), Attr("data-testid", "todo-sort"), Attr("aria-label", uistate.T("todo.sortLabel")), OnChange(onSort),
					Option(Value("smart"), SelectedIf(sm == "smart"), uistate.T("todo.sortSmart")),
					Option(Value("priority"), SelectedIf(sm == "priority"), uistate.T("todo.sortPriority")),
					Option(Value("due"), SelectedIf(sm == "due"), uistate.T("todo.sortDue")),
					Option(Value("az"), SelectedIf(sm == "az"), uistate.T("todo.sortAZ")),
				),
			),
			Label(css.Class("todo-ctrl"),
				Span(css.Class("todo-ctrl-label"), uistate.T("todo.showShort")),
				Select(css.Class("todo-select"), Attr("data-testid", "todo-filter-prio"), Attr("aria-label", uistate.T("todo.filterPrioLabel")), OnChange(onFilterPrio),
					Option(Value(""), SelectedIf(filterPrio.Get() == ""), uistate.T("todo.filterPrioAll")),
					Option(Value(string(domain.PriorityHigh)), SelectedIf(filterPrio.Get() == string(domain.PriorityHigh)), uistate.T("priority.high")),
					Option(Value(string(domain.PriorityMedium)), SelectedIf(filterPrio.Get() == string(domain.PriorityMedium)), uistate.T("priority.medium")),
					Option(Value(string(domain.PriorityLow)), SelectedIf(filterPrio.Get() == string(domain.PriorityLow)), uistate.T("priority.low")),
				),
			),
			Button(css.Class(hideToggleCls), Type("button"), Attr("aria-pressed", ariaBool(hideDone.Get())),
				Attr("data-testid", "todo-hide-done"), OnClick(toggleHideDone), Text(hideLabel)),
		),
		Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "todo-add"), Title(uistate.T("todo.addFirst")), OnClick(addTask),
			uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("todo.addTask"))),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
	})
}

// --- todo-list -------------------------------------------------------------------

// todoListWidget is the rows tile: the parent/child task tree (or the first-run empty
// CTA / all-done note). It owns the task mutation closures (toggle/complete, cascade
// delete, add sub-task) and reads the shared hide-done + priority filter atoms.
func todoListWidget(props todoListProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	hideDone := uistate.UseTodoHideDone()
	filterPrio := uistate.UseTodoFilterPrio()
	sortMode := uistate.UseTodoSortMode()
	pageAtom := uistate.UseTodoPage()
	collapsed := uistate.UseTodoCollapsed()
	errMsg := ui.UseState("")
	prevPage := ui.UseEvent(Prevent(func() {
		if p := pageAtom.Get(); p > 1 {
			pageAtom.Set(p - 1)
		}
	}))
	nextPage := ui.UseEvent(Prevent(func() { pageAtom.Set(pageAtom.Get() + 1) }))

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
			t.Status = domain.StatusOpen
			if err := app.PutTask(t); err != nil {
				errMsg.Set(err.Error())
				return
			}
		} else {
			// CompleteTask spawns a recurring task's next occurrence atomically.
			if err := app.CompleteTask(taskID, id.New(), time.Now()); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		uistate.BumpDataRevision()
	}
	deleteTask := func(taskID string) {
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
			uistate.BumpDataRevision()
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
			uistate.BumpDataRevision()
		})
	}

	// Order + filter, then nest into the parent/child tree (C72). hide-done filters
	// first so a done parent's open child surfaces as a root; the priority filter runs
	// after so the two cooperate.
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
	// Direct-children tally (over ALL tasks, ignoring view filters) for the collapsible
	// parent summary, plus the collapsed set that prunes hidden sub-trees.
	childStats := tasktree.ChildStats(tasks)
	collapsedSet := collapsed.Get()
	// Paginate by ROOT task (sub-trees stay together), ordered by the chosen sort mode.
	const todoPageSize = 20
	nodes, totalRoots := tasktree.Page(filtered, tasksort.ParseMode(sortMode.Get()), pageAtom.Get(), todoPageSize, collapsedSet)
	totalPages := (totalRoots + todoPageSize - 1) / todoPageSize
	if totalPages < 1 {
		totalPages = 1
	}
	curPage := pageAtom.Get()
	if curPage < 1 {
		curPage = 1
	}
	if curPage > totalPages {
		curPage = totalPages
	}

	var listBody ui.Node
	switch {
	case len(tasks) == 0:
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("todo.empty"), CTALabel: uistate.T("todo.addFirst"), AddTarget: "task", Icon: icon.Todo})
	case len(nodes) == 0:
		listBody = P(css.Class("empty"), uistate.T("todo.allDone"))
	default:
		toggleCollapse := func(id string) { uistate.ToggleTodoCollapsed(id) }
		rows := MapKeyed(nodes,
			func(n tasktree.Node) any { return n.Task.ID },
			func(n tasktree.Node) ui.Node {
				st := childStats[n.Task.ID]
				return ui.CreateElement(TaskRow, taskRowProps{
					Task: n.Task, Depth: n.Depth,
					OnToggle: toggleTask, OnDelete: deleteTask, OnAddSub: addSub,
					Accounts: accounts, Budgets: budgets, Goals: goals, Transactions: txns,
					ChildTotal: st.Total, ChildDone: st.Done,
					Collapsed: collapsedSet[n.Task.ID], OnToggleCollapse: toggleCollapse,
				})
			},
		)
		listBody = Div(css.Class("rows"), rows)
	}

	// When hide-done is active and there ARE completed tasks, acknowledge the hidden
	// work so it doesn't feel lost (mirrors the Transactions filter-summary pattern).
	var hiddenDoneNote ui.Node = Fragment()
	if hideDone.Get() {
		if c := computeTodoCounts(tasks, time.Now()); c.Done > 0 {
			hiddenDoneNote = P(css.Class("empty", tw.TextDim), uistate.T("todo.hiddenDone", c.Done))
		}
	}

	// Pager: shown only when the roots span more than one page. Prev/Next are disabled at
	// the ends; a range caption ("21–40 of 63") reads in tabular figures.
	var pager ui.Node = Fragment()
	if totalPages > 1 {
		first := (curPage-1)*todoPageSize + 1
		last := first + todoPageSize - 1
		if last > totalRoots {
			last = totalRoots
		}
		prevArgs := []any{css.Class("todo-page-btn"), Type("button"), Attr("data-testid", "todo-prev"), Attr("aria-label", uistate.T("todo.pagePrev")), OnClick(prevPage), uiw.Icon(icon.ChevronLeft, css.Class(tw.W4, tw.H4)), Span(uistate.T("todo.pagePrev"))}
		if curPage <= 1 {
			prevArgs = append(prevArgs, Attr("disabled", ""))
		}
		nextArgs := []any{css.Class("todo-page-btn"), Type("button"), Attr("data-testid", "todo-next"), Attr("aria-label", uistate.T("todo.pageNext")), OnClick(nextPage), Span(uistate.T("todo.pageNext")), uiw.Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4))}
		if curPage >= totalPages {
			nextArgs = append(nextArgs, Attr("disabled", ""))
		}
		pager = Div(css.Class("todo-pager"),
			Span(css.Class("todo-pager-range"), Attr("data-testid", "todo-pager-range"), uistate.T("todo.pageRange", first, last, totalRoots)),
			Div(css.Class("todo-pager-nav"),
				Button(prevArgs...),
				Span(css.Class("todo-pager-page"), uistate.T("todo.pageOf", curPage, totalPages)),
				Button(nextArgs...),
			),
		)
	}

	body := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("todo.listTitle"),
		Body: Fragment(
			If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
			listBody,
			hiddenDoneNote,
			pager,
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
