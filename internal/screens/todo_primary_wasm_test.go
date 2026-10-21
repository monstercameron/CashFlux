// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type primaryTodoControls struct{ View, Sort, Search state.Atom[string] }
type primaryTodoProps struct {
	App      *appstate.App
	Controls *primaryTodoControls
}

func primaryTodoFixture(props primaryTodoProps) ui.Node {
	props.Controls.View = uistate.UseTodoView()
	props.Controls.Sort = uistate.UseTodoSortMode()
	props.Controls.Search = uistate.UseTodoSearch()
	return Div(ui.CreateElement(todoSummaryWidget, todoSummaryProps{App: props.App}),
		ui.CreateElement(todoListWidget, todoListProps{App: props.App}))
}

func TestPrimaryTodoEmptyViewsAndKeyedRowState(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	controls := &primaryTodoControls{}
	f := render.New(t)
	root := ui.CreateElement(primaryTodoFixture, primaryTodoProps{App: app, Controls: controls})
	f.Render(root)
	tasks := []domain.Task{
		{ID: "a", Title: "Alpha", Notes: strings.Repeat("A long note ", 7), Status: domain.StatusOpen, Priority: domain.PriorityMedium, Order: 0, Due: time.Now()},
		{ID: "b", Title: "Beta", Notes: strings.Repeat("B long note ", 7), Status: domain.StatusOpen, Priority: domain.PriorityMedium, Order: 1, Due: time.Now()},
	}
	for _, task := range tasks {
		if err := app.PutTask(task); err != nil {
			t.Fatal(err)
		}
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	primaryClick(t, f, "task-note-a")
	controls.Sort.Set("manual")
	tasks[0].Order, tasks[1].Order = 2, 0
	for _, task := range tasks {
		if err := app.PutTask(task); err != nil {
			t.Fatal(err)
		}
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if got := byTestID(f, "button", "task-note-a"); got == nil || got.Attr("aria-expanded") != "true" {
		t.Fatal("task row expansion did not follow its ID after reorder")
	}
	if got := byTestID(f, "button", "task-note-b"); got == nil || got.Attr("aria-expanded") != "false" {
		t.Fatal("task row expansion migrated to the next row")
	}
	controls.Search.Set("no matching task")
	f.Stabilize()
	if byTestID(f, "button", "todo-add-from-search") == nil {
		t.Fatal("filtered empty list lost the add-from-search action")
	}
	controls.Search.Set("")
	for _, view := range []string{uistate.TodoViewBoard, uistate.TodoViewCalendar, uistate.TodoViewList} {
		controls.View.Set(view)
		f.Stabilize()
	}
	primaryClick(t, f, "task-check-a")
	for _, task := range app.Tasks() {
		if task.ID == "a" && task.Status != domain.StatusDone {
			t.Fatal("task action lost its hook after empty/list/board/calendar transitions")
		}
	}
}

type primaryTodoSuggestionsProps struct {
	App *appstate.App
	Now time.Time
}

func primaryTodoSuggestionsFixture(props primaryTodoSuggestionsProps) ui.Node {
	return todoSuggestStripAt(props.App, props.Now)
}

func TestPrimaryTodoSuggestionsRefreshAtFreshnessDeadlineWithoutWrite(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	deadline := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	account := domain.Account{ID: "checking", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, BalanceAsOf: deadline.AddDate(0, 0, -30)}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	props := primaryTodoSuggestionsProps{App: app, Now: deadline.Add(-time.Nanosecond)}
	f := render.New(t)
	f.Render(ui.CreateElement(primaryTodoSuggestionsFixture, props))
	if byTestID(f, "div", "todo-suggest-strip") != nil {
		t.Fatal("fresh account suggested a stale-balance task")
	}
	revision := app.Rev()
	props.Now = deadline.Add(time.Nanosecond)
	f.Render(ui.CreateElement(primaryTodoSuggestionsFixture, props))
	if app.Rev() != revision {
		t.Fatal("fixture changed stored data instead of crossing only the clock boundary")
	}
	if byTestID(f, "div", "todo-suggest-strip") == nil {
		t.Fatal("same-day freshness deadline did not invalidate suggestions")
	}
}
