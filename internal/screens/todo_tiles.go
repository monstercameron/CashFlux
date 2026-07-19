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
	"github.com/monstercameron/CashFlux/internal/taskboard"
	"github.com/monstercameron/CashFlux/internal/taskchecklist"
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
				// The raw numerator/denominator beneath the percent so "72%" reads
				// concretely as "33 of 46 done" at a glance.
				Div(css.Class("todo-done-count"), uistate.T("todo.doneCount", c.Done, c.Total)),
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
	linkID := uistate.UseTodoFilterLinkID()
	page := uistate.UseTodoPage()
	view := uistate.UseTodoView()
	boardGroup := uistate.UseTodoBoardGroup()
	quickView := uistate.UseTodoQuickView()

	// Changing any filter/search/sort resets to the first page so the view can't land on a
	// now-empty page.
	toggleHideDone := ui.UseEvent(Prevent(func() { hideDone.Set(!hideDone.Get()); page.Set(1) }))
	onFilterPrio := ui.UseEvent(func(e ui.Event) { filterPrio.Set(e.GetValue()); page.Set(1) })
	onSort := ui.UseEvent(func(e ui.Event) { sortMode.Set(e.GetValue()); page.Set(1) })
	onSearch := ui.UseEvent(func(v string) { search.Set(v); page.Set(1) })
	// Changing the link-type filter clears any specific-entity narrowing (deep-link) so the
	// dropdown always means what it says.
	onLink := ui.UseEvent(func(e ui.Event) { linkFilter.Set(e.GetValue()); linkID.Set(""); page.Set(1) })
	clearSearch := ui.UseEvent(Prevent(func() { search.Set(""); page.Set(1) }))
	addTask := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("task") }))
	// Checklist templates: one click instantiates a parent task + its steps
	// (taskchecklist.Instantiate) for the recurring rituals — the month-end
	// financial close and tax preparation. Plain closures; the OverflowMenu
	// items own their hooks.
	runChecklist := func(titleKey string, items []taskchecklist.Item, due time.Time) {
		app := props.App
		if app == nil {
			return
		}
		title := uistate.T(titleKey, due.Format("Jan 2006"))
		tasks := taskchecklist.Instantiate(title, items, due, id.New)
		for _, task := range tasks {
			if err := app.PutTask(task); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
		}
		uistate.BumpDataRevision()
		uistate.PostUndoable(uistate.T("todo.checklistAdded", title, len(tasks)-1))
	}
	addMonthEndClose := func() {
		now := time.Now()
		monthEnd := dateutil.MonthStart(now).AddDate(0, 1, -1)
		runChecklist("todo.tmplMonthEnd", []taskchecklist.Item{
			{Title: uistate.T("todo.tmplMEReconcile"), DueOffsetDays: -2},
			{Title: uistate.T("todo.tmplMEReview"), DueOffsetDays: -2},
			{Title: uistate.T("todo.tmplMEBudgets"), DueOffsetDays: -1},
			{Title: uistate.T("todo.tmplMEGoals"), DueOffsetDays: -1},
			{Title: uistate.T("todo.tmplMEReports")},
		}, monthEnd)
	}
	addTaxPrep := func() {
		due := time.Now().AddDate(0, 0, 30)
		runChecklist("todo.tmplTaxPrep", []taskchecklist.Item{
			{Title: uistate.T("todo.tmplTaxIncome"), DueOffsetDays: -21},
			{Title: uistate.T("todo.tmplTaxDeductible"), DueOffsetDays: -14},
			{Title: uistate.T("todo.tmplTaxReceipts"), DueOffsetDays: -14},
			{Title: uistate.T("todo.tmplTaxDonations"), DueOffsetDays: -7},
			{Title: uistate.T("todo.tmplTaxExport")},
		}, due)
	}
	// View switch (list / board / calendar) + the board's group-by. Fixed set of
	// controls, so their handlers sit at stable hook positions (no loop).
	setViewList := ui.UseEvent(Prevent(func() { view.Set(uistate.TodoViewList) }))
	setViewBoard := ui.UseEvent(Prevent(func() { view.Set(uistate.TodoViewBoard) }))
	setViewCal := ui.UseEvent(Prevent(func() { view.Set(uistate.TodoViewCalendar) }))
	onBoardGroup := ui.UseEvent(func(e ui.Event) { boardGroup.Set(e.GetValue()) })
	// Quick-view lens (All / Today / Overdue): three fixed buttons, so their handlers sit
	// at stable hook positions (not a loop). Switching resets to page 1.
	setQuickAll := ui.UseEvent(Prevent(func() { quickView.Set(string(tasksort.QuickAll)); page.Set(1) }))
	setQuickToday := ui.UseEvent(Prevent(func() { quickView.Set(string(tasksort.QuickToday)); page.Set(1) }))
	setQuickOverdue := ui.UseEvent(Prevent(func() { quickView.Set(string(tasksort.QuickOverdue)); page.Set(1) }))
	curView := view.Get()
	tvwCls := func(on bool) string {
		if on {
			return "tvw-btn is-active"
		}
		return "tvw-btn"
	}
	viewSwitch := Div(css.Class("todo-viewswitch"), Attr("role", "group"), Attr("aria-label", uistate.T("todo.viewLabel")),
		Button(ClassStr(tvwCls(curView == uistate.TodoViewList)), Type("button"), Attr("data-testid", "todo-view-list"),
			Attr("aria-pressed", ariaBool(curView == uistate.TodoViewList)), OnClick(setViewList), uistate.T("todo.viewList")),
		Button(ClassStr(tvwCls(curView == uistate.TodoViewBoard)), Type("button"), Attr("data-testid", "todo-view-board"),
			Attr("aria-pressed", ariaBool(curView == uistate.TodoViewBoard)), OnClick(setViewBoard), uistate.T("todo.viewBoard")),
		Button(ClassStr(tvwCls(curView == uistate.TodoViewCalendar)), Type("button"), Attr("data-testid", "todo-view-calendar"),
			Attr("aria-pressed", ariaBool(curView == uistate.TodoViewCalendar)), OnClick(setViewCal), uistate.T("todo.viewCalendar")),
	)
	// The board group-by pill appears only in board view (irrelevant elsewhere).
	var boardGroupCtrl ui.Node = Fragment()
	if curView == uistate.TodoViewBoard {
		bg := boardGroup.Get()
		boardGroupCtrl = Label(css.Class("todo-ctrl"),
			uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Span(css.Class("todo-ctrl-label"), uistate.T("todo.boardGroupLabel")),
			Select(css.Class("todo-select"), Attr("data-testid", "todo-board-group"), Attr("aria-label", uistate.T("todo.boardGroupLabel")), OnChange(onBoardGroup),
				Option(Value("status"), SelectedIf(bg == "status"), uistate.T("todo.boardGroupStatus")),
				Option(Value("priority"), SelectedIf(bg == "priority"), uistate.T("todo.boardGroupPriority")),
			),
		)
	}

	// Quick-view segmented control (All / Today / Overdue), badged with how many open
	// tasks are due today / past due. Shares the .todo-viewswitch chrome with the
	// display-view switch so the two read as one system.
	qc := tasksort.CountQuickViews(props.App.Tasks(), dateutil.FormatDate(time.Now()))
	cq := tasksort.ParseQuickView(quickView.Get())
	quickBadge := func(n int) ui.Node {
		if n <= 0 {
			return Fragment()
		}
		return Span(css.Class("tvw-count"), fmt.Sprintf("%d", n))
	}
	quickSwitch := Div(css.Class("todo-viewswitch"), Attr("role", "group"), Attr("aria-label", uistate.T("todo.lensLabel")),
		Button(ClassStr(tvwCls(cq == tasksort.QuickAll)), Type("button"), Attr("data-testid", "todo-quick-all"),
			Attr("aria-pressed", ariaBool(cq == tasksort.QuickAll)), OnClick(setQuickAll), uistate.T("todo.lensAll")),
		Button(ClassStr(tvwCls(cq == tasksort.QuickToday)), Type("button"), Attr("data-testid", "todo-quick-today"),
			Attr("aria-pressed", ariaBool(cq == tasksort.QuickToday)), OnClick(setQuickToday), uistate.T("todo.lensToday"), quickBadge(qc.Today)),
		Button(ClassStr(tvwCls(cq == tasksort.QuickOverdue)+" is-overdue"), Type("button"), Attr("data-testid", "todo-quick-overdue"),
			Attr("aria-pressed", ariaBool(cq == tasksort.QuickOverdue)), OnClick(setQuickOverdue), uistate.T("todo.lensOverdue"), quickBadge(qc.Overdue)),
	)

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
	// One standardized command bar (not a scattered tool collection): LEFT = search +
	// the active display view; MIDDLE = the quick-view lens, sort, and the task filters;
	// RIGHT = a single primary action (Add task) and one "More" menu for uncommon tools
	// (the checklist templates). Zones are laid out by .todo-cmdbar (registerTodoPolish).
	searchCtrl := Label(ClassStr(searchCls),
		uiw.Icon(icon.Search, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		Input(css.Class("todo-search-input"), Type("search"), Attr("data-testid", "todo-search"),
			Attr("aria-label", uistate.T("todo.searchLabel")), Placeholder(uistate.T("todo.searchPlaceholder")),
			Value(search.Get()), OnInput(onSearch)),
		If(search.Get() != "", Button(css.Class("todo-search-clear"), Type("button"), Attr("data-testid", "todo-search-clear"),
			Attr("aria-label", uistate.T("todo.searchClear")), OnClick(clearSearch), uiw.Icon(icon.Close, css.Class(tw.W3, tw.H3)))),
	)
	sortCtrl := Label(css.Class("todo-ctrl"),
		uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		Span(css.Class("todo-ctrl-label"), uistate.T("todo.sortShort")),
		Select(css.Class("todo-select"), Attr("data-testid", "todo-sort"), Attr("aria-label", uistate.T("todo.sortLabel")), OnChange(onSort),
			Option(Value("smart"), SelectedIf(sm == "smart"), uistate.T("todo.sortSmart")),
			Option(Value("priority"), SelectedIf(sm == "priority"), uistate.T("todo.sortPriority")),
			Option(Value("due"), SelectedIf(sm == "due"), uistate.T("todo.sortDue")),
			Option(Value("az"), SelectedIf(sm == "az"), uistate.T("todo.sortAZ")),
			Option(Value("manual"), SelectedIf(sm == "manual"), uistate.T("todo.sortManual")),
		),
	)
	prioCtrl := Label(css.Class("todo-ctrl"),
		Span(css.Class("todo-ctrl-label"), uistate.T("todo.showShort")),
		Select(css.Class("todo-select"), Attr("data-testid", "todo-filter-prio"), Attr("aria-label", uistate.T("todo.filterPrioLabel")), OnChange(onFilterPrio),
			Option(Value(""), SelectedIf(filterPrio.Get() == ""), uistate.T("todo.filterPrioAll")),
			Option(Value(string(domain.PriorityHigh)), SelectedIf(filterPrio.Get() == string(domain.PriorityHigh)), uistate.T("priority.high")),
			Option(Value(string(domain.PriorityMedium)), SelectedIf(filterPrio.Get() == string(domain.PriorityMedium)), uistate.T("priority.medium")),
			Option(Value(string(domain.PriorityLow)), SelectedIf(filterPrio.Get() == string(domain.PriorityLow)), uistate.T("priority.low")),
		),
	)
	linkCtrl := Label(css.Class("todo-ctrl"),
		uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		Span(css.Class("todo-ctrl-label"), uistate.T("todo.linkFilterShort")),
		Select(css.Class("todo-select"), Attr("data-testid", "todo-filter-link"), Attr("aria-label", uistate.T("todo.linkFilterLabel")), OnChange(onLink),
			Option(Value(uistate.TodoLinkAll), SelectedIf(lf == uistate.TodoLinkAll), uistate.T("todo.linkFilterAll")),
			Option(Value(uistate.TodoLinkGoal), SelectedIf(lf == uistate.TodoLinkGoal), uistate.T("todo.linkGoalPl")),
			Option(Value(uistate.TodoLinkBudget), SelectedIf(lf == uistate.TodoLinkBudget), uistate.T("todo.linkBudgetPl")),
			Option(Value(uistate.TodoLinkAccount), SelectedIf(lf == uistate.TodoLinkAccount), uistate.T("todo.linkAccountPl")),
			Option(Value(uistate.TodoLinkTransaction), SelectedIf(lf == uistate.TodoLinkTransaction), uistate.T("todo.linkTransactionPl")),
			Option(Value(uistate.TodoLinkNone), SelectedIf(lf == uistate.TodoLinkNone), uistate.T("todo.linkFilterNone")),
		),
	)
	hideToggle := Button(css.Class(hideToggleCls), Type("button"), Attr("aria-pressed", ariaBool(hideDone.Get())),
		Attr("data-testid", "todo-hide-done"), OnClick(toggleHideDone), Text(hideLabel))
	// One "More" menu for the uncommon tools — the checklist templates (month-end close
	// / tax-prep) that instantiate a parent task + ordered steps.
	moreMenu := uiw.OverflowMenu(uiw.OverflowMenuProps{
		TriggerText:   uistate.T("todo.moreTools"),
		TriggerLabel:  uistate.T("todo.moreTools"),
		TriggerTestID: "todo-checklists-btn",
		TriggerClass:  "btn btn-tool",
		Items: []uiw.OverflowMenuItem{
			{Label: uistate.T("todo.checklistMonthEnd"), Icon: icon.Calendar, TestID: "todo-checklist-monthend", OnSelect: addMonthEndClose},
			{Label: uistate.T("todo.checklistTaxPrep"), Icon: icon.FileText, TestID: "todo-checklist-taxprep", OnSelect: addTaxPrep},
		},
	})
	addBtn := Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "todo-add"), Title(uistate.T("todo.addFirst")), OnClick(addTask),
		uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("todo.addTask")))

	toolbar := Div(css.Class("todo-cmdbar"),
		Div(css.Class("cmdbar-group cmdbar-left"),
			searchCtrl,
			viewSwitch,
		),
		Div(css.Class("cmdbar-group cmdbar-mid"),
			quickSwitch,
			boardGroupCtrl,
			sortCtrl,
			prioCtrl,
			linkCtrl,
			hideToggle,
		),
		Div(css.Class("cmdbar-group cmdbar-right"),
			addBtn,
			moreMenu,
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
	dataRev := uistate.UseDataRevision().Get()
	app := props.App
	hideDone := uistate.UseTodoHideDone()
	filterPrio := uistate.UseTodoFilterPrio()
	sortMode := uistate.UseTodoSortMode()
	searchAtom := uistate.UseTodoSearch()
	linkAtom := uistate.UseTodoFilterLink()
	linkIDAtom := uistate.UseTodoFilterLinkID()
	pageAtom := uistate.UseTodoPage()
	pageSizeAtom := uistate.UseTodoPageSize()
	collapsed := uistate.UseTodoCollapsed()
	viewAtom := uistate.UseTodoView()
	quickViewAtom := uistate.UseTodoQuickView()
	boardGroupAtom := uistate.UseTodoBoardGroup()
	calOffsetAtom := uistate.UseTodoCalOffset()
	prefsAtom := uistate.UsePrefs()
	errMsg := ui.UseState("")
	dragSrc := ui.UseState("") // id of the task currently being dragged (Custom order)

	// The top field is a SEARCH box, but users reliably type a task title into it and
	// expect it to add — then hit "No tasks match". When a search yields nothing, the
	// empty state offers to add the typed text AS a task (opening the add-task modal
	// pre-filled with that title via the TaskAddSeed seam), turning the dead end into
	// the action they wanted.
	addFromSearch := ui.UseEvent(Prevent(func() {
		t := strings.TrimSpace(searchAtom.Get())
		if t != "" {
			uistate.SetTaskAddSeed(uistate.TaskAddSeed{Title: t})
		}
		uistate.SetAddTarget("task")
	}))

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
	// Specific-entity narrowing: a deep-link from e.g. a budget card's to-do panel filters
	// to just that budget's follow-ups (on top of the link-type filter above).
	if id := linkIDAtom.Get(); id != "" {
		kept := filtered[:0:0]
		for _, t := range filtered {
			if t.RelatedID == id {
				kept = append(kept, t)
			}
		}
		filtered = kept
	}
	// Quick-view lens (All / Today / Overdue): a coarse date narrowing to open tasks due
	// today or past due, applied like the other filters before tree nesting (so a matching
	// child surfaces as a root) and to every view projection below.
	filtered = tasksort.FilterQuickView(filtered, tasksort.ParseQuickView(quickViewAtom.Get()), dateutil.FormatDate(time.Now()))

	// Board / calendar views are alternate projections of the SAME filtered set (search +
	// priority + link + hide-done all still apply). They don't paginate or nest, so they
	// return here, before the list-only tree/pagination work. All hooks are above, so
	// these early returns are hook-safe.
	openTask := func(taskID string) { uistate.SetTaskEdit(uistate.TaskEdit{ID: taskID}) }
	errNode := If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get()))
	wrapTodoList := func(body ui.Node) ui.Node {
		return uiw.Widget(uiw.WidgetProps{
			ID: "todo-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
			Body: uiw.EntityListSection(uiw.EntityListSectionProps{Title: uistate.T("todo.listTitle"), Body: body}),
		})
	}
	switch viewAtom.Get() {
	case uistate.TodoViewBoard:
		by := taskboard.GroupByStatus
		if boardGroupAtom.Get() == "priority" {
			by = taskboard.GroupByPriority
		}
		moveTask := func(taskID, toKey string) {
			t, ok := byID[taskID]
			if !ok {
				return
			}
			if by == taskboard.GroupByPriority {
				p := domain.TaskPriority(toKey)
				if !p.Valid() {
					return
				}
				t.Priority = p
				if err := app.PutTask(t); err != nil {
					errMsg.Set(err.Error())
					return
				}
			} else if toKey == string(domain.StatusDone) {
				if t.Status != domain.StatusDone {
					if err := app.CompleteTask(taskID, id.New(), time.Now()); err != nil {
						errMsg.Set(err.Error())
						return
					}
				}
			} else {
				t.Status = domain.StatusOpen
				if err := app.PutTask(t); err != nil {
					errMsg.Set(err.Error())
					return
				}
			}
			uistate.BumpDataRevision()
		}
		board := ui.CreateElement(TaskBoardView, TaskBoardProps{Tasks: filtered, By: by, OnOpen: openTask, OnMove: moveTask, Rev: dataRev})
		return wrapTodoList(Fragment(errNode, board))
	case uistate.TodoViewCalendar:
		off := calOffsetAtom.Get()
		month := dateutil.AddMonths(dateutil.MonthStart(time.Now()), off)
		cal := todoCalendarView(todoCalendarProps{
			Tasks:      filtered,
			Month:      month,
			Today:      time.Now(),
			WeekStart:  prefsAtom.Get().WeekStartWeekday(),
			OnPrev:     func() { calOffsetAtom.Set(off - 1) },
			OnNext:     func() { calOffsetAtom.Set(off + 1) },
			OnOpenTask: openTask,
			OnAddOnDay: func(day time.Time) { uistate.SetTaskAddDue(dateutil.FormatDate(day)); uistate.SetAddTarget("task") },
		})
		return wrapTodoList(Fragment(errNode, cal))
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
		if q := strings.TrimSpace(searchAtom.Get()); q != "" {
			// Search yielded nothing → offer to add the typed text as a task.
			listBody = Div(css.Class("empty todo-nomatch"), Attr("data-testid", "todo-nomatch"),
				P(css.Class("empty"), uistate.T("todo.noMatches")),
				Button(css.Class("btn btn-primary btn-sm", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("data-testid", "todo-add-from-search"), OnClick(addFromSearch),
					uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("todo.addFromSearch", q))))
		} else if linkAtom.Get() != uistate.TodoLinkAll || filterPrio.Get() != "" {
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

	// The app-standard Pager (range + rows-per-page + prev/next + jump-to-page). A SINGLE
	// block below the list (the command bar already anchors the top of the workspace, so a
	// second, top pager was redundant clutter). setPageSize resets to page 1.
	setPage := func(n int) { pageAtom.Set(n) }
	setPageSize := func(s int) { pageSizeAtom.Set(s); pageAtom.Set(1) }
	var bottomPager ui.Node = Fragment()
	if len(nodes) > 0 {
		bottomPager = uiw.Pager(uiw.PagerProps{
			Page: curPage, Total: totalRoots, PageSize: todoPageSize,
			PageSizes: []int{10, 20, 50, 100}, OnPage: setPage, OnPageSize: setPageSize,
			Top: false, IDPrefix: "todo",
		})
	}

	body := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("todo.listTitle"),
		Body: Fragment(
			If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
			// Committed/manual tasks lead. The pager belongs to this list.
			listBody,
			hiddenDoneNote,
			bottomPager,
			// Condition-triggered proposals (stale balances, review backlog, overspent
			// budgets) now sit BELOW the user's own tasks, in a clearly-labeled section
			// that starts collapsed — one-click Add/Dismiss, never silent creation.
			ui.CreateElement(todoSuggestStrip, app),
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
