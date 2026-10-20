// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func primaryTodoNotificationsApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous; _ = app.Store().Close() })
	return app
}

type primaryNotifyControls struct {
	Feed   state.Atom[[]uistate.FeedItem]
	Member state.Atom[string]
}

type primaryNotifyProps struct {
	App      *appstate.App
	Controls *primaryNotifyControls
}

func primaryNotifyFixture(props primaryNotifyProps) ui.Node {
	props.Controls.Feed = uistate.UseNotifyFeed()
	props.Controls.Member = uistate.UseNotifyMemberFilter()
	return Div(ui.CreateElement(notifSummaryWidget, notifProps{App: props.App}),
		ui.CreateElement(notifListWidget, notifProps{App: props.App}))
}

func primarySetFeed(f *render.Fixture, controls *primaryNotifyControls, items []uistate.FeedItem) {
	uistate.PersistNotifyFeed(items)
	controls.Feed.Set(items)
	f.Stabilize()
}

func primaryClick(t *testing.T, f *render.Fixture, testID string) {
	t.Helper()
	button := byTestID(f, "button", testID)
	if button == nil {
		t.Fatalf("button %q missing in %s", testID, f.Text())
	}
	button.Click()
	f.Stabilize()
}

func TestPrimaryNotificationsEmptyTransitionsAndFinalDismissUndo(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	controls := &primaryNotifyControls{}
	f := render.New(t)
	f.Render(ui.CreateElement(primaryNotifyFixture, primaryNotifyProps{App: app, Controls: controls}))
	primarySetFeed(f, controls, nil)
	item := uistate.FeedItem{ID: "primary-final", Title: "Review this bill", Severity: "warning"}
	for cycle := 0; cycle < 2; cycle++ {
		primarySetFeed(f, controls, []uistate.FeedItem{item})
		if byTestID(f, "button", "notif-mark-all-read") == nil {
			t.Fatal("empty-to-populated summary lost its unread action")
		}
		primaryClick(t, f, "notif-mark-all-read")
		if !controls.Feed.Get()[0].Read {
			t.Fatal("mark-read action did not survive the empty transition")
		}
		uistate.DismissFeedItem(item.ID)
		f.Stabilize()
		if byTestID(f, "div", "notif-"+item.ID) != nil || byTestID(f, "div", "notif-undo-bar") == nil {
			t.Fatal("dismissing the final notification must retain Undo in the empty list")
		}
		primaryClick(t, f, "notif-undo-dismiss")
		if byTestID(f, "div", "notif-"+item.ID) == nil || len(controls.Feed.Get()) != 1 {
			t.Fatal("Undo did not restore the final notification")
		}
		if !controls.Feed.Get()[0].Read {
			t.Fatal("Undo discarded the restored item's read state")
		}
		primarySetFeed(f, controls, nil)
	}
}

func TestPrimaryNotificationGroupsKeepExpansionWithTheirKeys(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	controls := &primaryNotifyControls{}
	f := render.New(t)
	f.Render(ui.CreateElement(primaryNotifyFixture, primaryNotifyProps{App: app, Controls: controls}))
	items := []uistate.FeedItem{
		{ID: "a@1", Title: "A1", Severity: "warning", At: 9},
		{ID: "b@1", Title: "B1", Severity: "warning", At: 8},
		{ID: "a@2", Title: "A2", Severity: "warning", At: 7},
		{ID: "b@2", Title: "B2", Severity: "warning", At: 6},
		{ID: "a@3", Title: "A3", Severity: "warning", At: 5},
		{ID: "b@3", Title: "B3", Severity: "warning", At: 4},
	}
	primarySetFeed(f, controls, items)
	if byTestID(f, "div", "notif-a@1") != nil {
		t.Fatal("collapsed groups constructed visible child rows")
	}
	primaryClick(t, f, "notif-group-toggle-b")
	// Remove the group before B; expansion must stay with B's keyed component.
	primarySetFeed(f, controls, []uistate.FeedItem{items[1], items[3], items[5]})
	if got := byTestID(f, "button", "notif-group-toggle-b"); got == nil || got.Attr("aria-expanded") != "true" {
		t.Fatal("group B lost expansion when the preceding group disappeared")
	}
	if byTestID(f, "div", "notif-b@1") == nil {
		t.Fatal("expanded group lost its children")
	}
	primaryClick(t, f, "notif-group-toggle-b")
	if byTestID(f, "div", "notif-b@1") != nil {
		t.Fatal("closing a group did not remove its child rows")
	}
}

func TestPrimaryNotificationMemberLensKeepsSharedWorkAndResetControl(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	for _, member := range []domain.Member{{ID: "a", Name: "Alex"}, {ID: "b", Name: "Blair"}} {
		if err := app.PutMember(member); err != nil {
			t.Fatal(err)
		}
	}
	controls := &primaryNotifyControls{}
	f := render.New(t)
	f.Render(ui.CreateElement(primaryNotifyFixture, primaryNotifyProps{App: app, Controls: controls}))
	items := []uistate.FeedItem{{ID: "shared", Title: "Shared", Severity: "warning"}, {ID: "other", Title: "Other", Severity: "critical", MemberID: "b"}}
	primarySetFeed(f, controls, items)
	controls.Member.Set("a")
	f.Stabilize()
	if byTestID(f, "div", "notif-shared") == nil || byTestID(f, "div", "notif-other") != nil {
		t.Fatal("member lens did not retain shared work and hide another member's work")
	}
	primarySetFeed(f, controls, items[1:])
	if byTestID(f, "select", "notif-member-lens") == nil {
		t.Fatal("empty member lens hid the control needed to reset it")
	}
	byTestID(f, "select", "notif-member-lens").Change("")
	f.Stabilize()
	if byTestID(f, "div", "notif-other") == nil {
		t.Fatal("resetting an empty lens did not restore its rows")
	}
}

func TestPrimaryNotificationHistoryEmptyToPopulatedRetainsSearch(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	uistate.ClearNotificationHistory()
	t.Cleanup(uistate.ClearNotificationHistory)
	f := render.New(t)
	root := ui.CreateElement(notificationHistoryView, notifHistoryProps{App: app})
	f.Render(root)
	beforeRevision := uistate.CurrentDataRevision()
	uistate.RecordNotification("warning", "Primary archived bill", "", 10)
	items := uistate.ArchiveItems("", "")
	if len(items) != 1 || items[0].Message != "Primary archived bill" {
		t.Fatalf("first history record was not stored: %+v", items)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(f.Text(), "Primary archived bill") {
		t.Fatalf("history did not render its first record: records=%+v revision=%d->%d text=%q", items, beforeRevision, uistate.CurrentDataRevision(), f.Text())
	}
	byTestID(f, "input", "notif-history-search").Input("missing")
	f.Stabilize()
	if strings.Contains(f.Text(), "Primary archived bill") {
		t.Fatal("history search did not filter the populated archive")
	}
	byTestID(f, "input", "notif-history-search").Input("archived")
	f.Stabilize()
	if !strings.Contains(f.Text(), "Primary archived bill") {
		t.Fatal("search hook lost state after the filtered empty result")
	}
	primaryClick(t, f, "notif-history-clear")
	uistate.RecordNotification("warning", "Another archived bill", "", 20)
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(f.Text(), "Another archived bill") {
		t.Fatal("history did not recover after clearing and adding a record")
	}
}

func TestPrimaryNotificationWinsEmptyTransitionsPreserveSeenKeys(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	account := domain.Account{ID: "cash", Name: "Cash", Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "spent", AccountID: account.ID, Date: time.Now().AddDate(0, 0, -1), Amount: money.New(-100, "USD"), Desc: "Yesterday"}); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	root := ui.CreateElement(notifWinsWidget, notifProps{App: app})
	f.Render(root)
	if byTestID(f, "div", "wins-card") != nil {
		t.Fatal("fixture unexpectedly has a milestone")
	}
	goal := domain.Goal{ID: "g", Name: "Emergency reserve", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(100, "USD"), CurrentAmount: money.New(100, "USD")}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "div", "wins-card") == nil || !strings.Contains(uistate.KVGet(winsSeenKey), "goal:"+goal.Name) {
		t.Fatal("first win failed to render or persist its seen identity")
	}
	goal.CurrentAmount = money.New(0, "USD")
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "div", "wins-card") != nil {
		t.Fatal("removed milestone remained visible")
	}
	goal.CurrentAmount = goal.TargetAmount
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	card := byTestID(f, "div", "wins-card")
	if card == nil || strings.Contains(card.Attr("class"), "wins-card-fresh") {
		t.Fatal("returning seen milestone replayed its first-seen celebration")
	}
}
