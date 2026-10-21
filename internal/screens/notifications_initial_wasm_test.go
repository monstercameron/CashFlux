// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestNotificationCenterLazyFeedInitializationPreservesLiveActionsAndRemount(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	primaryTodoNotificationsApp(t)
	uistate.PersistNotifyFeed([]uistate.FeedItem{{ID: "initial-seed", Title: "Seeded alert", Severity: "warning"}})
	f := render.New(t)
	defer f.Cleanup()
	var mount func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		show := ui.UseState(true)
		mount = show.Set
		if show.Get() {
			return ui.CreateElement(NotificationCenter)
		}
		return nil
	}))
	if byTestID(f, "div", "notif-initial-seed") == nil {
		t.Fatal("persisted feed did not initialize real NotificationCenter")
	}
	primaryClick(t, f, "notif-read-initial-seed")
	assertRead := func() {
		t.Helper()
		button := byTestID(f, "button", "notif-read-initial-seed")
		if button == nil || button.Text() != uistate.T("notifications.markUnread") {
			t.Fatalf("live read state was reset by default hydration: %s", f.Text())
		}
	}
	assertRead()
	uistate.PrependNotifyFeed([]uistate.FeedItem{{ID: "initial-new", Title: "New live alert", Severity: "critical"}})
	f.Stabilize()
	if byTestID(f, "div", "notif-initial-new") == nil {
		t.Fatal("boot/live catch-up did not reach production subscribers")
	}
	assertRead()
	primaryClick(t, f, "notif-view-history")
	uistate.PrependNotifyFeed([]uistate.FeedItem{{ID: "initial-hidden", Title: "Arrived in History", Severity: "warning"}})
	f.Stabilize()
	primaryClick(t, f, "notif-view-live")
	if byTestID(f, "div", "notif-initial-hidden") == nil {
		t.Fatal("return from History lost the current atom feed")
	}
	assertRead()
	mount(false)
	f.Stabilize()
	mount(true)
	f.Stabilize()
	if byTestID(f, "div", "notif-initial-hidden") == nil || byTestID(f, "div", "notif-initial-new") == nil {
		t.Fatal("remount reused an earlier feed snapshot")
	}
	assertRead()
	uistate.DismissFeedItem("initial-seed")
	f.Stabilize()
	if byTestID(f, "div", "notif-initial-seed") != nil {
		t.Fatal("dismissed alert reappeared from the persisted default")
	}
	primaryClick(t, f, "notif-undo-dismiss")
	assertRead()
}
