// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/milestones"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
	"strings"
	"testing"
	"time"
)

func TestPrimaryNotificationRosterWritesRefreshSummaryAndRows(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	for _, member := range []domain.Member{{ID: "a", Name: "Alex"}, {ID: "b", Name: "Blair"}} {
		if err := app.PutMember(member); err != nil {
			t.Fatal(err)
		}
	}
	controls := &primaryNotifyControls{}
	f := render.New(t)
	f.Render(ui.CreateElement(primaryNotifyFixture, primaryNotifyProps{App: app, Controls: controls}))
	primarySetFeed(f, controls, []uistate.FeedItem{{ID: "assigned", Title: "Assigned work", Severity: "warning", MemberID: "a"}})
	if err := app.PutMember(domain.Member{ID: "a", Name: "Alex renamed"}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	chip := byTestID(f, "span", "notif-member-a")
	lens := byTestID(f, "select", "notif-member-lens")
	if chip == nil || chip.Text() != "Alex renamed" || lens == nil || !strings.Contains(lens.Text(), "Alex renamed") {
		t.Fatal("member rename did not refresh both lens options and the existing feed row")
	}
	if err := app.PutMember(domain.Member{ID: "c", Name: "Casey"}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(byTestID(f, "select", "notif-member-lens").Text(), "Casey") {
		t.Fatal("member addition did not refresh the lens")
	}
	if err := app.DeleteMember("c"); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if strings.Contains(byTestID(f, "select", "notif-member-lens").Text(), "Casey") {
		t.Fatal("member deletion left a stale lens option")
	}
}

func TestPrimaryNotificationSharedTabsKeepHistoryAndLiveTransitions(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(notifSurfaceShell, notifSurfaceShellProps{App: app, Live: P("Live feed marker")}))
	for _, view := range []string{"history", "watching", "live"} {
		primaryClick(t, f, "notif-view-"+view)
		if byTestID(f, "button", "notif-view-"+view).Attr("aria-selected") != "true" {
			t.Fatalf("triage tab %q did not become selected", view)
		}
		if view == "history" && byTestID(f, "input", "notif-history-search") == nil {
			t.Fatal("history tab did not mount the archive")
		}
		if view != "history" && !strings.Contains(f.Text(), "Live feed marker") {
			t.Fatal("live triage tab did not restore the provided feed surface")
		}
	}
}

func TestPrimaryNotificationWinsCacheRefreshesOnDayAndDataRevision(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	if err := app.PutAccount(domain.Account{ID: "a", Name: "Cash", Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	spent := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if err := app.PutTransaction(domain.Transaction{ID: "t", AccountID: "a", Desc: "Expense", Date: spent, Amount: money.New(-100, "USD")}); err != nil {
		t.Fatal(err)
	}
	has := func(model notificationWinsSnapshot, kind milestones.Kind) bool {
		for _, win := range model.Found {
			if win.Kind == kind {
				return true
			}
		}
		return false
	}
	if has(winsSnapshot(app, spent.AddDate(0, 0, 3), time.Sunday), milestones.KindNoSpendStreak) {
		t.Fatal("two completed no-spend days reached a three-day milestone")
	}
	if !has(winsSnapshot(app, spent.AddDate(0, 0, 4), time.Sunday), milestones.KindNoSpendStreak) {
		t.Fatal("new clock day did not refresh the no-spend milestone")
	}
	goal := domain.Goal{ID: "g", Name: "Funded", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		CurrentAmount: money.New(100, "USD"), TargetAmount: money.New(100, "USD")}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if !has(winsSnapshot(app, spent.AddDate(0, 0, 4), time.Sunday), milestones.KindGoalReached) {
		t.Fatal("goal data revision did not refresh the same day's milestone snapshot")
	}
}
