// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	"github.com/monstercameron/CashFlux/internal/tasktree"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	search := uistate.UseTodoSearch()
	linkFilter := uistate.UseTodoFilterLink()
	page := uistate.UseTodoPage()

	// Changing any filter/search/sort resets to the first page so the view can't land on a
	// now-empty page.
	toggleHideDone := ui.UseEvent(Prevent(func() { hideDone.Set(!hideDone.Get()); page.Set(1) }))
	onFilterPrio := ui.UseEvent(func(e ui.Event) { filterPrio.Set(e.GetValue()); page.Set(1) })
	onSort := ui.UseEvent(func(e ui.Event) { sortMode.Set(e.GetValue()); page.Set(1) })
	onSearch := ui.UseEvent(func(v string) { search.Set(v); page.Set(1) })
	onLink := ui.UseEvent(func(e ui.Event) { linkFilter.Set(e.GetValue()); page.Set(1) })
	clearSearch := ui.UseEvent(Prevent(func() { search.Set(""); page.Set(1) }))
	addTask := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("task") }))

	hideLabel := uistate.T("todo.hideDone")
	if hideDone.Get() {
		hideLabel = uistate.T("todo.showAll")
	}
	sm := sortMode.Get()
	lf := linkFilter.Get()

	hideToggleCls := "strip-toggle"
	if hideDone.Get() {
		hideToggleCls += " is-on"
	}
	searchCls := "todo-ctrl todo-ctrl-search"
	if search.Get() != "" {
		searchCls += " is-active"
	}
	// A cohesive filter strip: a search box leads, then sort + priority + "linked to" pill
	// selects and a hide-done toggle, with the primary Add task pushed to the right. The
	// strip layout is shared with /goals (.filter-strip).
	toolbar := Div(css.Class("filter-toolbar"),
		// Row 1: the search fills the line (standard two-row toolbar, matching
		// transactions/accounts/budgets). Row 2 holds the sort/priority/linked selects,
		// the hide-done toggle, and the primary Add task.
		Div(css.Class("filter-toolbar-primary"),
			Label(ClassStr(searchCls),
				uiw.Icon(icon.Search, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Input(css.Class("todo-search-input"), Type("search"), Attr("data-testid", "todo-search"),
					Attr("aria-label", uistate.T("todo.searchLabel")), Placeholder(uistate.T("todo.searchPlaceholder")),
					Value(search.Get()), OnInput(onSearch)),
				If(search.Get() != "", Button(css.Class("todo-search-clear"), Type("button"), Attr("data-testid", "todo-search-clear"),
					Attr("aria-label", uistate.T("todo.searchClear")), OnClick(clearSearch), uiw.Icon(icon.Close, css.Class(tw.W3, tw.H3)))),
			),
		),
		Div(css.Class("filter-toolbar-actions"),
			Label(css.Class("todo-ctrl"),
				uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(css.Class("todo-ctrl-label"), uistate.T("todo.sortShort")),
				Select(css.Class("todo-select"), Attr("data-testid", "todo-sort"), Attr("aria-label", uistate.T("todo.sortLabel")), OnChange(onSort),
					Option(Value("smart"), SelectedIf(sm == "smart"), uistate.T("todo.sortSmart")),
					Option(Value("priority"), SelectedIf(sm == "priority"), uistate.T("todo.sortPriority")),
					Option(Value("due"), SelectedIf(sm == "due"), uistate.T("todo.sortDue")),
					Option(Value("az"), SelectedIf(sm == "az"), uistate.T("todo.sortAZ")),
					Option(Value("manual"), SelectedIf(sm == "manual"), uistate.T("todo.sortManual")),
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
			Label(css.Class("todo-ctrl"),
				uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(css.Class("todo-ctrl-label"), uistate.T("todo.linkFilterShort")),
				Select(css.Class("todo-select"), Attr("data-testid", "todo-filter-link"), Attr("aria-label", uistate.T("todo.linkFilterLabel")), OnChange(onLink),
					Option(Value(uistate.TodoLinkAll), SelectedIf(lf == uistate.TodoLinkAll), uistate.T("todo.linkFilterAll")),
					Option(Value(uistate.TodoLinkGoal), SelectedIf(lf == uistate.TodoLinkGoal), uistate.T("todo.linkGoalPl")),
					Option(Value(uistate.TodoLinkBudget), SelectedIf(lf == uistate.TodoLinkBudget), uistate.T("todo.linkBudgetPl")),
					Option(Value(uistate.TodoLinkAccount), SelectedIf(lf == uistate.TodoLinkAccount), uistate.T("todo.linkAccountPl")),
					Option(Value(uistate.TodoLinkNone), SelectedIf(lf == uistate.TodoLinkNone), uistate.T("todo.linkFilterNone")),
				),
			),
			Button(css.Class(hideToggleCls), Type("button"), Attr("aria-pressed", ariaBool(hideDone.Get())),
				Attr("data-testid", "todo-hide-done"), OnClick(toggleHideDone), Text(hideLabel)),
			Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "todo-add"), Title(uistate.T("todo.addFirst")), OnClick(addTask),
				uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("todo.addTask"))),
		),
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
	searchAtom := uistate.UseTodoSearch()
	linkAtom := uistate.UseTodoFilterLink()
	pageAtom := uistate.UseTodoPage()
	pageSizeAtom := uistate.UseTodoPageSize()
	collapsed := uistate.UseTodoCollapsed()
	errMsg := ui.UseState("")
	dragSrc := ui.UseState("") // id of the task currently being dragged (Custom order)

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
	// A sub-task now opens the full compose modal (title/notes/priority/due/link/repeat)
	// seeded with the parent, instead of a bare one-line prompt.
	addSub := func(parentID string) {
		uistate.SetTaskAddParent(parentID)
		uistate.SetAddTarget("task")
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
	// Free-text search over title + notes (case-insensitive). Like the other filters it runs
	// on the flat list before tree nesting, so a matching child surfaces as a root.
	if q := strings.ToLower(strings.TrimSpace(searchAtom.Get())); q != "" {
		kept := filtered[:0:0]
		for _, t := range filtered {
			if strings.Contains(strings.ToLower(t.Title), q) || strings.Contains(strings.ToLower(t.Notes), q) {
				kept = append(kept, t)
			}
		}
		filtered = kept
	}
	// Linked-feature filter: tasks tied to goals/budgets/accounts, only unlinked ones, or all.
	if lf := linkAtom.Get(); lf != uistate.TodoLinkAll {
		kept := filtered[:0:0]
		for _, t := range filtered {
			linked := t.RelatedID != "" && t.RelatedType != "" && t.RelatedType != domain.RelatedNone
			var match bool
			if lf == uistate.TodoLinkNone {
				match = !linked
			} else {
				match = linked && string(t.RelatedType) == lf
			}
			if match {
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
	todoPageSize := pageSizeAtom.Get()
	nodes, totalRoots := tasktree.Page(filtered, tasksort.ParseMode(sortMode.Get()), pageAtom.Get(), todoPageSize, collapsedSet)
	curPage := pagination.Clamp(pageAtom.Get(), totalRoots, todoPageSize)

	// Drag-to-reorder is live only in "Custom order"; a drop asks appstate to move the
	// dragged task into the target's slot among its siblings.
	manualMode := tasksort.ParseMode(sortMode.Get()) == tasksort.ModeManual
	reorderTask := func(targetID string) {
		src := dragSrc.Get()
		if src == "" || src == targetID {
			return
		}
		if err := app.ReorderTask(src, targetID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		dragSrc.Set("")
		uistate.BumpDataRevision()
	}

	var listBody ui.Node
	switch {
	case len(tasks) == 0:
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("todo.empty"), CTALabel: uistate.T("todo.addFirst"), AddTarget: "task", Icon: icon.Todo})
	case len(nodes) == 0:
		if searchAtom.Get() != "" || linkAtom.Get() != uistate.TodoLinkAll || filterPrio.Get() != "" {
			listBody = P(css.Class("empty"), uistate.T("todo.noMatches"))
		} else {
			listBody = P(css.Class("empty"), uistate.T("todo.allDone"))
		}
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
					Draggable:   manualMode,
					OnDragStart: func() { dragSrc.Set(n.Task.ID) },
					OnDrop:      func() { reorderTask(n.Task.ID) },
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

	// The app-standard Pager (range + rows-per-page + prev/next + jump-to-page), mirrored
	// top + bottom so a long list never needs a scroll to page. setPageSize resets to page 1.
	setPage := func(n int) { pageAtom.Set(n) }
	setPageSize := func(s int) { pageSizeAtom.Set(s); pageAtom.Set(1) }
	pagerProps := func(top bool) uiw.PagerProps {
		return uiw.PagerProps{
			Page: curPage, Total: totalRoots, PageSize: todoPageSize,
			PageSizes: []int{10, 20, 50, 100}, OnPage: setPage, OnPageSize: setPageSize,
			Top: top, IDPrefix: "todo",
		}
	}
	// Show the top pager whenever there are more tasks than the smallest page size (10) —
	// guarding on the total, not the current page count, so picking a bigger rows-per-page
	// (fewer pages) never makes the top pager disappear under the cursor.
	var topPager ui.Node = Fragment()
	if len(nodes) > 0 && totalRoots > 10 {
		topPager = uiw.Pager(pagerProps(true))
	}
	var bottomPager ui.Node = Fragment()
	if len(nodes) > 0 {
		bottomPager = uiw.Pager(pagerProps(false))
	}

	body := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("todo.listTitle"),
		Body: Fragment(
			If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
			topPager,
			listBody,
			hiddenDoneNote,
			bottomPager,
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
