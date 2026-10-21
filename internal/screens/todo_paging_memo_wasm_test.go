// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	"github.com/monstercameron/CashFlux/internal/tasktree"
	"github.com/monstercameron/CashFlux/internal/taskview"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type todoPagingControls struct {
	View, Sort, Search, Priority, Link, LinkID, Quick state.Atom[string]
	Page, Size                                        state.Atom[int]
	HideDone                                          state.Atom[bool]
	Collapsed                                         state.Atom[map[string]bool]
	Pick                                              func(ui.Event)
}

type todoPagingProps struct {
	App      *appstate.App
	Controls *todoPagingControls
}

func todoPagingHost(p todoPagingProps) ui.Node {
	c := p.Controls
	c.View, c.Sort, c.Search = uistate.UseTodoView(), uistate.UseTodoSortMode(), uistate.UseTodoSearch()
	c.Priority, c.Link, c.LinkID = uistate.UseTodoFilterPrio(), uistate.UseTodoFilterLink(), uistate.UseTodoFilterLinkID()
	c.Quick, c.HideDone = uistate.UseTodoQuickView(), uistate.UseTodoHideDone()
	c.Page, c.Size, c.Collapsed = uistate.UseTodoPage(), uistate.UseTodoPageSize(), uistate.UseTodoCollapsed()
	// render.Event omits mouse modifier fields. Forward a browser-shaped event
	// through the actual row handler without changing production selection code.
	pick := ui.UseEvent(func(e ui.Event) {
		e.JSValue().Set("shiftKey", false)
		if c.Pick != nil {
			c.Pick(e)
		}
	})
	return Fragment(Button(Attr("data-testid", "todo-paging-pick"), OnClick(pick)),
		ui.CreateElement(todoListWidget, todoListProps{App: p.App}))
}

func todoPagingFixture(t *testing.T) (*appstate.App, *render.Fixture, *todoPagingControls, *int) {
	t.Helper()
	a := primaryTodoNotificationsApp(t)
	today := dateutil.CalendarDayUTC(time.Now())
	for _, task := range []domain.Task{
		{ID: "page-alpha", Title: "Alpha", Priority: domain.PriorityMedium, Status: domain.StatusOpen, Due: today},
		{ID: "page-child", ParentID: "page-alpha", Title: "Child needle", Priority: domain.PriorityHigh, Status: domain.StatusOpen, Due: today},
		{ID: "page-beta", Title: "Beta", Priority: domain.PriorityLow, Status: domain.StatusOpen, Due: today.AddDate(0, 0, 1)},
		{ID: "page-gamma", Title: "Gamma", Priority: domain.PriorityHigh, Status: domain.StatusDone, Due: today.AddDate(0, 0, -1)},
		{ID: "page-delta", Title: "Delta", Priority: domain.PriorityMedium, Status: domain.StatusOpen, RelatedType: domain.RelatedGoal, RelatedID: "goal"},
	} {
		if err := a.PutTask(task); err != nil {
			t.Fatal(err)
		}
	}
	builds := new(int)
	previous := todoPageBuildObserver
	todoPageBuildObserver = func() { *builds++ }
	t.Cleanup(func() { todoPageBuildObserver = previous })
	c := &todoPagingControls{}
	f := render.New(t, render.WithQueuedScheduler())
	t.Cleanup(f.Cleanup)
	f.Render(ui.CreateElement(todoPagingHost, todoPagingProps{App: a, Controls: c}))
	f.Stabilize()
	if *builds != 1 {
		t.Fatalf("cold List built the native page %d times, want once", *builds)
	}
	return a, f, c, builds
}

func assertTodoNativePage(t *testing.T, a *appstate.App, f *render.Fixture, c *todoPagingControls) {
	t.Helper()
	filter := taskview.Filter{HideDone: c.HideDone.Get(), Priority: domain.TaskPriority(c.Priority.Get()),
		Search: c.Search.Get(), Link: c.Link.Get(), LinkID: c.LinkID.Get(), Quick: tasksort.ParseQuickView(c.Quick.Get()), TodayISO: dateutil.FormatDate(time.Now())}
	nodes, _ := tasktree.Page(taskview.Select(a.Tasks(), filter), tasksort.ParseMode(c.Sort.Get()), c.Page.Get(), c.Size.Get(), c.Collapsed.Get())
	want := make([]string, 0, len(nodes))
	for _, node := range nodes {
		want = append(want, node.Task.ID)
	}
	got := make([]string, 0, len(nodes))
	for _, button := range f.AllByTag("button") {
		if id := button.Attr("data-testid"); strings.HasPrefix(id, "task-check-") {
			got = append(got, strings.TrimPrefix(id, "task-check-"))
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rendered page order %v, want native root/subtree page %v", got, want)
	}
}

func TestTodoPagingMemoReusesBulkSelectionAndErrorRenders(t *testing.T) {
	a, f, c, builds := todoPagingFixture(t)
	assertTodoNativePage(t, a, f, c)
	primaryClick(t, f, "todo-bulk-enter")
	selection := byTestID(f, "input", "task-select-page-alpha")
	if selection == nil {
		t.Fatal("actual selection checkbox did not mount")
	}
	var ok bool
	c.Pick, ok = selection.Property("onclick").(func(ui.Event))
	if !ok {
		t.Fatalf("actual row selection handler has unexpected type %T", selection.Property("onclick"))
	}
	primaryClick(t, f, "todo-paging-pick")
	if count := byTestID(f, "span", "todo-bulk-count"); count == nil || count.Text() != uistate.T("todo.bulkSelected", 1) {
		t.Fatal("row selection callback did not update the selected count")
	}
	for _, action := range []string{"todo-bulk-all", "todo-bulk-clear", "todo-bulk-exit", "todo-bulk-enter", "todo-bulk-exit"} {
		primaryClick(t, f, action)
	}
	a.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	primaryClick(t, f, "task-check-page-alpha")
	if f.ByRole("alert", "") == nil || a.Tasks()[0].Status == domain.StatusDone {
		t.Fatal("read-only row action did not retain its error")
	}
	if *builds != 1 {
		t.Fatalf("presentation and error renders rebuilt the native page %d times", *builds)
	}
	a.SetActiveRoleFunc(nil)
	primaryClick(t, f, "task-check-page-alpha")
	if *builds != 2 {
		t.Fatalf("real task write did not invalidate the native page: %d builds", *builds)
	}
	assertTodoNativePage(t, a, f, c)
}

func TestTodoPagingMemoInvalidatesInputsAndGatesAlternateViews(t *testing.T) {
	a, f, c, builds := todoPagingFixture(t)
	step := func(name string, change func()) {
		t.Helper()
		before := *builds
		change()
		f.Stabilize()
		if *builds != before+1 {
			t.Fatalf("%s built %d native pages, want one", name, *builds-before)
		}
		assertTodoNativePage(t, a, f, c)
	}
	step("root page size and sort", func() { c.Size.Set(2); c.Sort.Set("az") })
	step("next root page", func() { primaryClick(t, f, "todo-next") })
	step("out-of-range clamping", func() { c.Page.Set(99) })
	step("first root page", func() { c.Page.Set(1) })
	step("collapse", func() { primaryClick(t, f, "task-collapse-page-alpha") })
	step("expand", func() { primaryClick(t, f, "task-collapse-page-alpha") })
	step("sort", func() { c.Sort.Set("priority") })
	step("child search without parent", func() { c.Search.Set("needle") })
	step("clear search", func() { c.Search.Set("") })
	step("priority", func() { c.Priority.Set(string(domain.PriorityHigh)) })
	step("hide done", func() { c.HideDone.Set(true) })
	step("quick view", func() { c.Quick.Set("today") })
	step("reset filters", func() { c.Priority.Set(""); c.HideDone.Set(false); c.Quick.Set("all") })
	step("link type", func() { c.Link.Set(string(domain.RelatedGoal)) })
	step("specific linked goal", func() { c.LinkID.Set("other") })
	step("clear link", func() { c.Link.Set(""); c.LinkID.Set("") })
	step("UI revision", func() { uistate.BumpDataRevision() })

	warm := *builds
	for _, view := range []string{uistate.TodoViewBoard, uistate.TodoViewCalendar, uistate.TodoViewBoard} {
		c.View.Set(view)
		f.Stabilize()
		c.Search.Set(view)
		c.Page.Set(4)
		f.Stabilize()
		if *builds != warm {
			t.Fatalf("%s view built an unused List page", view)
		}
	}
	c.Search.Set("")
	c.Page.Set(1)
	step("return to List", func() { c.View.Set(uistate.TodoViewList) })
	primaryClick(t, f, "task-collapse-page-alpha")
	primaryClick(t, f, "todo-bulk-enter")
	if byTestID(f, "input", "task-select-page-alpha") == nil {
		t.Fatal("List callback hooks lost their positions after alternate views")
	}
}
