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
	"github.com/monstercameron/CashFlux/internal/taskbulk"
	"github.com/monstercameron/CashFlux/internal/taskchecklist"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	"github.com/monstercameron/CashFlux/internal/tasktree"
	"github.com/monstercameron/CashFlux/internal/taskview"
	"github.com/monstercameron/CashFlux/internal/todoview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type todoSummaryProps struct{ App *appstate.App }
type todoToolbarProps struct{ App *appstate.App }
type todoListProps struct{ App *appstate.App }

// --- todo-summary ----------------------------------------------------------------

// todoSummaryWidget is the headline tile: a completion "loader" bar (done-of-total) with
// the Open / Overdue / Done figures rendered inside it, mirroring the goals summary.
// Renders nothing when there are no tasks (the list tile owns the empty CTA).
func todoSummaryWidget(props todoSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	tasks := props.App.Tasks()
	today := dateutil.FormatDate(time.Now())
	c := taskSnapshot(props.App, today).Counts
	if len(tasks) == 0 {
		return Fragment()
	}

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
	navTo := router.UseNavigate()
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
	// C404: saved views. The store is re-read per render rather than held in a
	// hook, so a view saved on one tile is visible on the next render of another
	// — the same reason the notification rule config is re-read on toggle.
	savedRev := ui.UseState(0)
	_ = savedRev.Get()
	savedViews := uistate.LoadTodoViews()

	// Changing any filter/search/sort resets to the first page so the view can't land on a
	// now-empty page.
	toggleHideDone := ui.UseEvent(Prevent(func() { hideDone.Set(!hideDone.Get()); page.Set(1) }))
	onFilterPrio := ui.UseEvent(func(e ui.Event) { filterPrio.Set(e.GetValue()); page.Set(1) })
	onSort := ui.UseEvent(func(e ui.Event) { sortMode.Set(e.GetValue()); page.Set(1) })
	// The FilterToolbar owns the search box now (C404); it takes a plain func, so
	// this stays a hook only to keep the hook order in this function stable.
	onSearch := ui.UseEvent(func(v string) { search.Set(v); page.Set(1) })
	_ = onSearch
	// Changing the link-type filter clears any specific-entity narrowing (deep-link) so the
	// dropdown always means what it says.
	onLink := ui.UseEvent(func(e ui.Event) { linkFilter.Set(e.GetValue()); linkID.Set(""); page.Set(1) })
	clearSearch := ui.UseEvent(Prevent(func() { search.Set(""); page.Set(1) }))
	_ = clearSearch
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
	// Two more financial rituals the reviewer asked for (C405). Insurance and
	// subscriptions drift upward quietly, and debt only gets paid off by someone
	// looking at it on a schedule — both are exactly the kind of work that never
	// happens without a dated list.
	addSubscriptionAudit := func() {
		due := time.Now().AddDate(0, 0, 10)
		runChecklist("todo.tmplSubAudit", []taskchecklist.Item{
			{Title: uistate.T("todo.tmplSubList"), DueOffsetDays: -7},
			{Title: uistate.T("todo.tmplSubUnused"), DueOffsetDays: -5},
			{Title: uistate.T("todo.tmplSubPrices"), DueOffsetDays: -3},
			{Title: uistate.T("todo.tmplSubInsurance"), DueOffsetDays: -2},
			{Title: uistate.T("todo.tmplSubCancel")},
		}, due)
	}
	addDebtCheckin := func() {
		due := time.Now().AddDate(0, 0, 7)
		runChecklist("todo.tmplDebt", []taskchecklist.Item{
			{Title: uistate.T("todo.tmplDebtBalances"), DueOffsetDays: -5},
			{Title: uistate.T("todo.tmplDebtRates"), DueOffsetDays: -3},
			{Title: uistate.T("todo.tmplDebtExtra"), DueOffsetDays: -1},
			{Title: uistate.T("todo.tmplDebtPlan")},
		}, due)
	}
	// The automation catalog lives on the Workflows screen; the to-do tools menu is
	// where someone already thinking about recurring work will look for it, so it
	// gets a signpost rather than a duplicate gallery (C405).
	openAutomations := func() { navTo.Navigate(uistate.RoutePath("/workflows")) }
	// A quarterly account review — reconcile balances, prune subscriptions, sanity-check
	// budgets, and rebalance goal funding. Due in two weeks so there's runway to work the
	// steps, wired exactly like the month-end / tax templates above.
	addQuarterlyReview := func() {
		due := time.Now().AddDate(0, 0, 14)
		runChecklist("todo.tmplQuarterly", []taskchecklist.Item{
			{Title: uistate.T("todo.tmplQtrBalances"), DueOffsetDays: -10},
			{Title: uistate.T("todo.tmplQtrSubscriptions"), DueOffsetDays: -7},
			{Title: uistate.T("todo.tmplQtrBudgets"), DueOffsetDays: -3},
			{Title: uistate.T("todo.tmplQtrGoals")},
		}, due)
	}
	// The shared keyed control owns each choice's handler; the page owns state.
	onBoardGroup := ui.UseEvent(func(e ui.Event) { boardGroup.Set(e.GetValue()) })
	curView := view.Get()
	viewSwitch := uiw.SegmentedControl(uiw.SegmentedControlProps{
		Class: "todo-viewswitch", ButtonClass: "tvw-btn", ActiveClass: "is-active",
		Label: uistate.T("todo.viewLabel"), Selected: curView,
		Options: []uiw.SegmentOption{
			{Key: uistate.TodoViewList, Label: uistate.T("todo.viewList"), TestID: "todo-view-list"},
			{Key: uistate.TodoViewBoard, Label: uistate.T("todo.viewBoard"), TestID: "todo-view-board"},
			{Key: uistate.TodoViewCalendar, Label: uistate.T("todo.viewCalendar"), TestID: "todo-view-calendar"},
		},
		OnSelect: func(value string) { view.Set(value) },
	})
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
	today := dateutil.FormatDate(time.Now())
	qc := taskSnapshot(props.App, today).Counts
	cq := tasksort.ParseQuickView(quickView.Get())
	quickSwitch := uiw.SegmentedControl(uiw.SegmentedControlProps{
		Class: "todo-viewswitch", ButtonClass: "tvw-btn", ActiveClass: "is-active", CountClass: "tvw-count",
		Label: uistate.T("todo.lensLabel"), Selected: string(cq),
		Options: []uiw.SegmentOption{
			{Key: string(tasksort.QuickAll), Label: uistate.T("todo.lensAll"), TestID: "todo-quick-all"},
			{Key: string(tasksort.QuickToday), Label: uistate.T("todo.lensToday"), TestID: "todo-quick-today", Count: qc.Today, ShowCount: qc.Today > 0},
			{Key: string(tasksort.QuickOverdue), Label: uistate.T("todo.lensOverdue"), TestID: "todo-quick-overdue", Class: "is-overdue", Count: qc.Overdue, ShowCount: qc.Overdue > 0},
		},
		OnSelect: func(value string) { quickView.Set(value); page.Set(1) },
	})

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
	// ─── C404: saved views ───────────────────────────────────────────────────
	//
	// currentView is the live screen expressed as a todoview.View, which is what
	// makes both directions cheap: saving is a copy, and highlighting the active
	// entry is an equality check. The fields are the atoms' own strings, so
	// applying a view is assignment rather than translation — a mapping layer is
	// somewhere the saved state and the live state could disagree.
	currentView := todoview.View{
		Display: string(view.Get()), Quick: quickView.Get(), Sort: sortMode.Get(),
		Priority: filterPrio.Get(), Link: linkFilter.Get(), BoardGroup: boardGroup.Get(),
		HideDone: hideDone.Get(), Search: search.Get(),
	}
	activeView := savedViews.ActiveName(currentView)
	applyView := func(v todoview.View) {
		view.Set(v.Display)
		quickView.Set(v.Quick)
		sortMode.Set(v.Sort)
		filterPrio.Set(v.Priority)
		linkFilter.Set(v.Link)
		// A saved view names a KIND of link, never one specific entity: a deep-link
		// narrowing survives only for the navigation that set it.
		linkID.Set("")
		boardGroup.Set(v.BoardGroup)
		hideDone.Set(v.HideDone)
		search.Set(v.Search)
		page.Set(1)
	}
	saveCurrentView := func() {
		uistate.PromptModal(uistate.T("todo.viewSavePrompt"), activeView, func(name string) {
			if strings.TrimSpace(name) == "" {
				return
			}
			v := currentView
			v.Name = name
			store := uistate.LoadTodoViews()
			if err := store.Save(v); err != nil {
				uistate.PostNotice(viewSaveError(err), true)
				return
			}
			uistate.SaveTodoViews(store)
			savedRev.Set(savedRev.Get() + 1)
			uistate.PostNotice(uistate.T("todo.viewSaved", strings.TrimSpace(name)), false)
		})
	}
	deleteActiveView := func() {
		if activeView == "" {
			return
		}
		name := activeView
		uistate.ConfirmModal(uistate.T("todo.viewDeleteConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			store := uistate.LoadTodoViews()
			if store.Delete(name) {
				uistate.SaveTodoViews(store)
				savedRev.Set(savedRev.Get() + 1)
				uistate.PostNotice(uistate.T("todo.viewDeleted", name), false)
			}
		})
	}
	viewItems := make([]uiw.OverflowMenuItem, 0, len(savedViews.Views)+2)
	for _, v := range savedViews.Views {
		label := v.Name
		if v.Name == activeView {
			// A check mark, not a disabled row: re-picking the active view is a
			// harmless way to undo an accidental control change.
			label = "✓ " + v.Name
		}
		viewItems = append(viewItems, uiw.OverflowMenuItem{
			Label: label, Icon: icon.Filter, TestID: "todo-view-apply-" + todoview.Key(v.Name),
			OnSelect: func() { applyView(v) },
		})
	}
	viewItems = append(viewItems,
		uiw.OverflowMenuItem{Label: uistate.T("todo.viewSave"), Icon: icon.Check,
			TestID: "todo-view-save", OnSelect: saveCurrentView},
		uiw.OverflowMenuItem{Label: uistate.T("todo.viewDelete"), Icon: icon.Trash,
			TestID: "todo-view-delete", OnSelect: deleteActiveView, Hidden: activeView == ""},
	)
	viewsLabel := uistate.T("todo.views")
	if activeView != "" {
		viewsLabel = activeView
	}
	viewsMenu := uiw.OverflowMenu(uiw.OverflowMenuProps{
		TriggerText:   viewsLabel,
		TriggerLabel:  uistate.T("todo.viewsLabel"),
		TriggerTestID: "todo-views-btn",
		TriggerClass:  "btn btn-tool",
		Items:         viewItems,
	})

	// The "Templates & tools" menu — the checklist templates (month-end close, tax prep,
	// quarterly account review) that instantiate a parent task + ordered steps. Named for
	// what it holds rather than a vague "More".
	moreMenu := uiw.OverflowMenu(uiw.OverflowMenuProps{
		TriggerText:   uistate.T("todo.templatesTools"),
		TriggerLabel:  uistate.T("todo.templatesTools"),
		TriggerTestID: "todo-checklists-btn",
		TriggerClass:  "btn btn-tool",
		Items: []uiw.OverflowMenuItem{
			{Label: uistate.T("todo.checklistMonthEnd"), Icon: icon.Calendar, TestID: "todo-checklist-monthend", OnSelect: addMonthEndClose},
			{Label: uistate.T("todo.checklistTaxPrep"), Icon: icon.FileText, TestID: "todo-checklist-taxprep", OnSelect: addTaxPrep},
			{Label: uistate.T("todo.checklistQuarterly"), Icon: icon.Refresh, TestID: "todo-checklist-quarterly", OnSelect: addQuarterlyReview},
			{Label: uistate.T("todo.checklistSubAudit"), Icon: icon.Repeat, TestID: "todo-checklist-subaudit", OnSelect: addSubscriptionAudit},
			{Label: uistate.T("todo.checklistDebt"), Icon: icon.TrendingDown, TestID: "todo-checklist-debt", OnSelect: addDebtCheckin},
			{Label: uistate.T("todo.automations"), Icon: icon.Workflow, TestID: "todo-automations", OnSelect: openAutomations},
		},
	})
	addBtn := Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "todo-add"), Title(uistate.T("todo.addFirst")), OnClick(addTask),
		uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("todo.addTask")))

	// C404: ONE toolbar row instead of three. The reviewer's complaint was density
	// — sort, priority, link and hide-done were four always-visible controls that
	// most sessions never touch, competing with the two that every session does
	// (search, and which view you are in). They move into the shared FilterToolbar
	// popover, which brings chips summarizing what is narrowed and a badge counting
	// it, so nothing becomes invisible by becoming collapsed.
	filterFields := Div(css.Class("todo-filter-fields"),
		sortCtrl, prioCtrl, linkCtrl, boardGroupCtrl,
		Div(css.Class("todo-filter-row"), hideToggle),
	)
	// Chips name only what is actually narrowing the list. Sort is deliberately
	// NOT a chip: changing the order does not hide anything, and a "clear all
	// filters" that silently reset the sort would be doing more than it says.
	var chips []uiw.Chip
	if filterPrio.Get() != "" {
		chips = append(chips, uiw.Chip{Key: "prio",
			Label: uistate.T("todo.chipPriority", todoPriorityName(filterPrio.Get()))})
	}
	if lf != uistate.TodoLinkAll {
		chips = append(chips, uiw.Chip{Key: "link", Label: uistate.T("todo.chipLink", todoLinkName(lf))})
	}
	if hideDone.Get() {
		chips = append(chips, uiw.Chip{Key: "hidedone", Label: uistate.T("todo.chipHideDone")})
	}
	removeChip := func(key string) {
		switch key {
		case "prio":
			filterPrio.Set("")
		case "link":
			linkFilter.Set(uistate.TodoLinkAll)
			linkID.Set("")
		case "hidedone":
			hideDone.Set(false)
		}
		page.Set(1)
	}
	clearAllFilters := func() {
		filterPrio.Set("")
		linkFilter.Set(uistate.TodoLinkAll)
		linkID.Set("")
		hideDone.Set(false)
		page.Set(1)
	}

	toolbar := Div(css.Class("todo-cmdbar"),
		Div(css.Class("cmdbar-row cmdbar-single"),
			viewSwitch,
			quickSwitch,
			Div(css.Class("cmdbar-grow"),
				uiw.FilterToolbar(uiw.FilterToolbarProps{
					Search:       search.Get(),
					SearchLabel:  uistate.T("todo.searchLabel"),
					OnSearch:     func(v string) { search.Set(v); page.Set(1) },
					FiltersLabel: uistate.T("todo.filters"),
					FiltersTitle: uistate.T("todo.filtersTitle"),
					FilterFields: filterFields,
					ActiveAriaLabel: func(n int) string {
						if n == 0 {
							return uistate.T("todo.filters")
						}
						return uistate.T("todo.filtersActiveAria", n)
					},
					Chips:         chips,
					OnRemoveChip:  removeChip,
					OnClearAll:    clearAllFilters,
					ClearAllLabel: uistate.T("todo.clearFilters"),
					RemoveLabel:   uistate.T("todo.removeFilter"),
					Actions:       []ui.Node{addBtn, viewsMenu, moreMenu},
				}),
			),
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
	// C402: bulk selection. selectMode gates the checkbox column so the common
	// single-task case is not permanently taxed with one; selected holds the picked
	// ids; lastSelID anchors shift-click ranges; bulkMember/bulkDue hold the pending
	// edit; undoTasks is a one-level snapshot of what a bulk write overwrote.
	selectMode := ui.UseState(false)
	selected := ui.UseState(map[string]bool{})
	lastSelID := ui.UseState("")
	undoTasks := ui.UseState([]domain.Task(nil))
	undoLabel := ui.UseState("")

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
	members := app.Members()
	today := dateutil.FormatDate(time.Now())
	snapshot := taskSnapshot(app, today)
	byID, counts, childStats := snapshot.ByID, snapshot.Counts, snapshot.Children

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
		if task, ok := byID[taskID]; ok && task.Title != "" {
			name = task.Title
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

	// All display modes share the same tested, single-pass flat filter. Nesting
	// and root pagination happen afterward so filtered parents never hide children.
	filter := taskview.Filter{
		HideDone: hideDone.Get(), Priority: domain.TaskPriority(filterPrio.Get()),
		Search: searchAtom.Get(), Link: linkAtom.Get(), LinkID: linkIDAtom.Get(),
		Quick: tasksort.ParseQuickView(quickViewAtom.Get()), TodayISO: today,
	}
	filtered := ui.UseMemo(func() []domain.Task { return taskview.Select(tasks, filter) },
		revKey(app), filter.HideDone, string(filter.Priority), filter.Search, filter.Link, filter.LinkID, string(filter.Quick), today)

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

	// ─── C402: bulk selection over the visible page ──────────────────────────
	//
	// visibleOrder is the on-screen order of the current page, which is what a
	// shift-click range means: the rows between these two, as displayed — not as
	// stored. Select-all likewise means "everything I can see", never the whole
	// unfiltered table, or one click on a filtered view would silently grab rows
	// the filter was hiding.
	visibleOrder := make([]string, 0, len(nodes))
	for _, n := range nodes {
		visibleOrder = append(visibleOrder, n.Task.ID)
	}
	copySel := func() map[string]bool {
		m := selected.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			if v {
				nm[k] = v
			}
		}
		return nm
	}
	toggleSelect := func(taskID string, shift bool) {
		nm := copySel()
		if shift && lastSelID.Get() != "" && lastSelID.Get() != taskID {
			if r := taskbulk.Range(visibleOrder, lastSelID.Get(), taskID); len(r) > 0 {
				for _, x := range r {
					nm[x] = true
				}
				selected.Set(nm)
				lastSelID.Set(taskID)
				return
			}
		}
		if nm[taskID] {
			delete(nm, taskID)
		} else {
			nm[taskID] = true
		}
		selected.Set(nm)
		lastSelID.Set(taskID)
	}
	clearSelection := func() {
		selected.Set(map[string]bool{})
		lastSelID.Set("")
	}
	selectAllVisible := func() {
		nm := map[string]bool{}
		for _, x := range visibleOrder {
			nm[x] = true
		}
		selected.Set(nm)
	}
	exitSelectMode := func() {
		selectMode.Set(false)
		clearSelection()
	}
	enterSelectMode := func() { selectMode.Set(true) }

	// Every bulk write records what it overwrote, so one click can put it back.
	// Bulk edits are the one place where a mis-click costs a dozen rows at once,
	// and an action that expensive has to be reversible.
	remember := func(prior []domain.Task, label string) {
		undoTasks.Set(prior)
		undoLabel.Set(label)
	}
	priorOf := func(ids []string) []domain.Task {
		out := make([]domain.Task, 0, len(ids))
		for _, x := range ids {
			if t, ok := byID[x]; ok {
				out = append(out, t)
			}
		}
		return out
	}
	undoBulk := func() {
		for _, t := range undoTasks.Get() {
			if err := app.PutTask(t); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		n := len(undoTasks.Get())
		undoTasks.Set(nil)
		undoLabel.Set("")
		uistate.PostNotice(uistate.T("todo.bulkUndone", n), false)
		uistate.BumpDataRevision()
	}

	applyBulkEdit := func(e taskbulk.Edit) {
		plan := taskbulk.Plan(tasks, selected.Get(), e, time.Now())
		if len(plan) == 0 {
			// Nothing would change. Saying so beats a silent click and beats a
			// "12 tasks updated" toast for twelve rows that already matched.
			uistate.PostNotice(uistate.T("todo.bulkNoChange"), false)
			return
		}
		prior := make([]domain.Task, 0, len(plan))
		for _, t := range plan {
			if p, ok := byID[t.ID]; ok {
				prior = append(prior, p)
			}
			if err := app.PutTask(t); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		remember(prior, uistate.T("todo.bulkUpdated", len(plan)))
		uistate.PostNotice(uistate.T("todo.bulkUpdated", len(plan)), false)
		uistate.BumpDataRevision()
	}
	bulkComplete := func() {
		ids := taskbulk.Completable(tasks, selected.Get())
		if len(ids) == 0 {
			uistate.PostNotice(uistate.T("todo.bulkNoneOpen"), false)
			return
		}
		prior := priorOf(ids)
		for _, x := range ids {
			// CompleteTask, not a status write: a recurring task must spawn its
			// successor here exactly as it does from the single-row checkbox.
			if err := app.CompleteTask(x, id.New(), time.Now()); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		remember(prior, uistate.T("todo.bulkCompleted", len(ids)))
		uistate.PostNotice(uistate.T("todo.bulkCompleted", len(ids)), false)
		clearSelection()
		uistate.BumpDataRevision()
	}
	bulkDelete := func() {
		ids := taskbulk.Deletable(tasks, selected.Get())
		if len(ids) == 0 {
			return
		}
		// Count the whole blast radius, sub-trees included: deleting a parent takes
		// its children, and a confirm that undercounts is worse than none.
		doomed := map[string]bool{}
		for _, x := range ids {
			doomed[x] = true
			for _, dsc := range tasktree.Descendants(tasks, x) {
				doomed[dsc] = true
			}
		}
		uistate.ConfirmModal(uistate.T("todo.bulkDeleteConfirm", len(doomed)), true, func(ok bool) {
			if !ok {
				return
			}
			prior := make([]domain.Task, 0, len(doomed))
			for _, t := range tasks {
				if doomed[t.ID] {
					prior = append(prior, t)
				}
			}
			for x := range doomed {
				_ = app.DeleteTask(x)
			}
			remember(prior, uistate.T("todo.bulkDeleted", len(doomed)))
			uistate.PostNotice(uistate.T("todo.bulkDeleted", len(doomed)), false)
			clearSelection()
			uistate.BumpDataRevision()
		})
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
				linkName, linkFound := snapshot.EntityName(n.Task.RelatedType, n.Task.RelatedID)
				memberName, memberFound := snapshot.MemberNames[n.Task.MemberID]
				return ui.CreateElement(TaskRow, taskRowProps{
					Task: n.Task, Depth: n.Depth,
					OnToggle: toggleTask, OnDelete: deleteTask, OnAddSub: addSub,
					LinkName: linkName, LinkFound: linkFound, MemberName: memberName, MemberFound: memberFound,
					ChildTotal: st.Total, ChildDone: st.Done,
					Collapsed: collapsedSet[n.Task.ID], OnToggleCollapse: toggleCollapse,
					Draggable:   manualMode,
					OnDragStart: func() { dragSrc.Set(n.Task.ID) },
					OnDrop:      func() { reorderTask(n.Task.ID) },
					Selectable:  selectMode.Get(),
					Selected:    selected.Get()[n.Task.ID],
					OnSelect:    toggleSelect,
				})
			},
		)
		listBody = Div(css.Class("rows"), rows)
	}

	// When hide-done is active and there ARE completed tasks, acknowledge the hidden
	// work so it doesn't feel lost (mirrors the Transactions filter-summary pattern).
	var hiddenDoneNote ui.Node = Fragment()
	if hideDone.Get() {
		if counts.Done > 0 {
			hiddenDoneNote = P(css.Class("empty", tw.TextDim), uistate.T("todo.hiddenDone", counts.Done))
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

	// The selection bar and the undo banner live in their own component: their
	// buttons need hooks, and this function returns early for the board and
	// calendar views, so hooks declared down here would shift position whenever the
	// user switched view. A child component owns its own hook list and simply is
	// not rendered in those views.
	bulkBar := ui.CreateElement(todoBulkBar, todoBulkBarProps{
		Active:      selectMode.Get(),
		Count:       len(selected.Get()),
		HasRows:     len(nodes) > 0,
		Members:     members,
		UndoCount:   len(undoTasks.Get()),
		UndoLabel:   undoLabel.Get(),
		OnEnter:     enterSelectMode,
		OnExit:      exitSelectMode,
		OnSelectAll: selectAllVisible,
		OnClear:     clearSelection,
		OnAssign:    func(memberID string) { applyBulkEdit(taskbulk.Edit{MemberID: memberID}) },
		OnReschedule: func(shift taskbulk.DueShift) {
			applyBulkEdit(taskbulk.Edit{Due: shift})
		},
		OnComplete: bulkComplete,
		OnDelete:   bulkDelete,
		OnUndo:     undoBulk,
	})

	body := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("todo.listTitle"),
		Body: Fragment(
			If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
			// C402: selection is opt-in — the bar is a small "Select several" button
			// until you ask for it.
			bulkBar,
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
