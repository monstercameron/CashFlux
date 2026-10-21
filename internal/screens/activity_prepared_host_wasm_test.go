// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func activityHostApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() {
		appstate.Default = previous
		_ = app.Store().Close()
	})
	return app
}

func setActivityHostFeed(t *testing.T, entries []auditlog.Entry) {
	t.Helper()
	saved := auditview.Feed.Recent(0)
	auditview.Feed.Clear()
	for _, entry := range entries {
		auditview.Feed.Append(entry)
	}
	t.Cleanup(func() {
		auditview.Feed.Clear()
		for i := len(saved) - 1; i >= 0; i-- {
			auditview.Feed.Append(saved[i])
		}
	})
}

func activityHostNode(f *render.Fixture, tag, testID string) *render.QueryNode {
	for _, node := range f.AllByTag(tag) {
		if node.Attr("data-testid") == testID {
			return node
		}
	}
	return nil
}

func activityHostRows(f *render.Fixture) []string {
	var ids []string
	for _, node := range f.AllByTag("div") {
		id := node.Attr("data-testid")
		if strings.HasPrefix(id, "activity-entry-") {
			ids = append(ids, strings.TrimPrefix(id, "activity-entry-"))
		}
	}
	return ids
}

func TestActivityHostFilterPreservesPreparedOrderAndUndoClickUsesLivePreflight(t *testing.T) {
	activityHostApp(t)
	day := time.Date(2026, time.October, 21, 0, 0, 0, 0, time.UTC)
	setActivityHostFeed(t, []auditlog.Entry{
		{ID: "txn-old", At: day.Add(-24 * time.Hour), Actor: "user", Action: "added", EntityType: "transaction", Summary: "Older transaction"},
		{ID: "txn-new", At: day.Add(12 * time.Hour), Actor: "user", Action: "added", EntityType: "transaction", Summary: "Newest transaction"},
		{ID: "acct-newest", At: day.Add(14 * time.Hour), Actor: "system", Action: "updated", EntityType: "account", Summary: "Newest account"},
	})

	previousCanUndo, previousUndo := auditview.CanUndoFunc, auditview.UndoFunc
	liveUndoAllowed := false
	undoCalls, appliedUndos := 0, 0
	auditview.CanUndoFunc = func() bool { return true }
	auditview.UndoFunc = func() bool {
		undoCalls++
		if !liveUndoAllowed { // models the authoritative click-time preflight refusing stale render facts.
			return false
		}
		appliedUndos++
		return true
	}
	t.Cleanup(func() { auditview.CanUndoFunc, auditview.UndoFunc = previousCanUndo, previousUndo })

	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Activity))
	f.Stabilize()
	if got := strings.Join(activityHostRows(f), ","); got != "acct-newest,txn-new,txn-old" {
		t.Fatalf("initial activity row order = %q", got)
	}

	filter := activityHostNode(f, "select", "activity-entity-filter")
	if filter == nil {
		t.Fatal("activity entity filter is missing")
	}
	filter.Change("transaction")
	f.Stabilize()
	if got := strings.Join(activityHostRows(f), ","); got != "txn-new,txn-old" {
		t.Fatalf("filtered activity rows = %q", got)
	}
	undo := activityHostNode(f, "button", "activity-undo-txn-new")
	if undo == nil {
		t.Fatal("newest filtered row does not own the undo affordance")
	}
	undo.Click()
	f.Stabilize()
	if undoCalls != 1 || appliedUndos != 0 {
		t.Fatalf("undo must reach live preflight and honor its refusal: calls=%d applied=%d", undoCalls, appliedUndos)
	}
}

func TestActivityHostReportsUnavailableHousehold(t *testing.T) {
	previous := appstate.Default
	appstate.Default = nil
	t.Cleanup(func() { appstate.Default = previous })
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Activity))
	if !strings.Contains(strings.ToLower(f.Text()), "not ready") {
		t.Fatalf("missing household did not report an unavailable source: %s", f.Text())
	}
}
