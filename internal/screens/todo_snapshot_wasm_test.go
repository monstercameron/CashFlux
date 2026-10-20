// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
	"testing"
	"time"
)

func TestPrimaryTaskSnapshotIsolatesAppsAndRefreshesDatesAndNames(t *testing.T) {
	apps := []*appstate.App{primaryTodoNotificationsApp(t), primaryTodoNotificationsApp(t)}
	for i, app := range apps {
		name := []string{"First checking", "Other checking"}[i]
		if err := app.PutAccount(domain.Account{ID: "a", Name: name, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
			t.Fatal(err)
		}
		if err := app.PutTask(domain.Task{ID: "t", Title: "Review", Status: domain.StatusOpen, Priority: domain.PriorityMedium,
			Due: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), RelatedType: domain.RelatedAccount, RelatedID: "a"}); err != nil {
			t.Fatal(err)
		}
	}
	if apps[0].Rev() != apps[1].Rev() {
		t.Fatal("fixture needs equal revisions on distinct apps")
	}
	for i, app := range apps {
		snapshot := taskSnapshot(app, "2026-10-20")
		name, found := snapshot.EntityName(domain.RelatedAccount, "a")
		if !found || name != []string{"First checking", "Other checking"}[i] || snapshot.Counts.Today != 1 {
			t.Fatalf("app %d snapshot got name %q, found %t, counts %+v", i, name, found, snapshot.Counts)
		}
	}
	if got := taskSnapshot(apps[0], "2026-10-21").Counts; got.Today != 0 || got.Overdue != 1 {
		t.Fatalf("day boundary did not refresh task counts: %+v", got)
	}
	account, ok := domain.AccountByID(apps[0].Accounts(), "a")
	if !ok {
		t.Fatal("fixture account missing")
	}
	account.Name = "Updated checking"
	if err := apps[0].PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if name, _ := taskSnapshot(apps[0], "2026-10-21").EntityName(domain.RelatedAccount, "a"); name != "Updated checking" {
		t.Fatal("entity revision did not refresh resolved task link names")
	}
}

type primarySegmentControls struct {
	View, Quick state.Atom[string]
	Page        state.Atom[int]
}

type primarySegmentToolbarProps struct {
	App      *appstate.App
	Controls *primarySegmentControls
}

func primarySegmentToolbarFixture(props primarySegmentToolbarProps) ui.Node {
	props.Controls.View = uistate.UseTodoView()
	props.Controls.Quick = uistate.UseTodoQuickView()
	props.Controls.Page = uistate.UseTodoPage()
	return ui.CreateElement(todoToolbarWidget, todoToolbarProps{App: props.App})
}

func TestPrimaryTodoSharedSegmentsKeepViewAndQuickFilterActions(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	if err := app.PutTask(domain.Task{ID: "t", Title: "Today", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: time.Now()}); err != nil {
		t.Fatal(err)
	}
	controls := &primarySegmentControls{}
	f := render.New(t)
	f.Render(ui.CreateElement(primarySegmentToolbarFixture, primarySegmentToolbarProps{App: app, Controls: controls}))
	controls.Page.Set(3)
	f.Stabilize()
	primaryClick(t, f, "todo-quick-today")
	if controls.Quick.Get() != "today" || controls.Page.Get() != 1 || byTestID(f, "button", "todo-quick-today").Attr("aria-pressed") != "true" {
		t.Fatal("quick segment did not select its lens and reset pagination")
	}
	for _, view := range []string{uistate.TodoViewBoard, uistate.TodoViewCalendar, uistate.TodoViewList} {
		primaryClick(t, f, "todo-view-"+view)
		if controls.View.Get() != view || byTestID(f, "button", "todo-view-"+view).Attr("aria-pressed") != "true" {
			t.Fatalf("view segment %q did not retain its action/pressed state", view)
		}
	}
}
