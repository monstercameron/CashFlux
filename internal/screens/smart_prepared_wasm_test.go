// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func countPreparedSmartRuns(t *testing.T) *int {
	t.Helper()
	count := new(int)
	previousRun, previousPage := smartPreparedRun, smartPreparedRunPage
	smartPreparedRun = func(in smartengine.Input, settings smart.Settings) []smart.Insight {
		*count++
		return smartengine.Run(in, settings)
	}
	smartPreparedRunPage = func(in smartengine.Input, settings smart.Settings, page smart.Page) []smart.Insight {
		*count++
		return smartengine.RunPage(in, settings, page)
	}
	t.Cleanup(func() { smartPreparedRun, smartPreparedRunPage = previousRun, previousPage })
	return count
}

func TestPreparedSmartResultParityOwnershipAndExactInvalidation(t *testing.T) {
	smartResultsCache.Clear()
	app := primarySmartApp(t, "Cafe")
	other := primarySmartApp(t, "Other")
	count := countPreparedSmartRuns(t)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	settings := smart.EnableFreeOnly(smart.Settings{})
	first, err := preparedSmartInsightsAt(app, time.Monday, settings, "", "en", now)
	if err != nil {
		t.Fatal(err)
	}
	want := smartengine.Run(buildSmartInputRawAt(app, time.Monday, now), settings)
	if !reflect.DeepEqual(first, want) {
		t.Fatal("prepared all-page result differs from engine")
	}
	if len(first) == 0 {
		t.Fatal("fixture produced no SMART findings")
	}
	first[0].Key = "mutated by renderer"
	second, err := preparedSmartInsightsAt(app, time.Monday, settings, "", "en", now.Add(20*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if *count != 1 || !reflect.DeepEqual(second, want) {
		t.Fatalf("warm result was recomputed or exposed renderer mutation: runs=%d", *count)
	}

	checks := []struct {
		name     string
		app      *appstate.App
		week     time.Weekday
		settings smart.Settings
		page     smart.Page
		language string
		now      time.Time
	}{
		{"page", app, time.Monday, settings, smart.PageTransactions, "en", now},
		{"week", app, time.Sunday, settings, "", "en", now},
		{"locale", app, time.Monday, settings, "", "es", now},
		{"time", app, time.Monday, settings, "", "en", now.Add(time.Minute)},
		{"app identity", other, time.Monday, settings, "", "en", now},
	}
	dismissed := settings.Dismiss(want[0].Key)
	checks = append(checks, struct {
		name     string
		app      *appstate.App
		week     time.Weekday
		settings smart.Settings
		page     smart.Page
		language string
		now      time.Time
	}{"dismissal", app, time.Monday, dismissed, "", "en", now})
	for _, check := range checks {
		before := *count
		got, err := preparedSmartInsightsAt(check.app, check.week, check.settings, check.page, check.language, check.now)
		if err != nil {
			t.Fatalf("%s: %v", check.name, err)
		}
		if *count != before+1 {
			t.Fatalf("%s reused a different result: runs=%d", check.name, *count)
		}
		in := buildSmartInputRawAt(check.app, check.week, check.now)
		var expected []smart.Insight
		if check.page == "" {
			expected = smartengine.Run(in, check.settings)
		} else {
			expected = smartengine.RunPage(in, check.settings, check.page)
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("%s differs from engine", check.name)
		}
	}
	txn := app.Transactions()[0]
	txn.Desc = "Changed"
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	before := *count
	if _, err := preparedSmartInsightsAt(app, time.Monday, settings, "", "en", now); err != nil {
		t.Fatal(err)
	}
	if *count != before+1 {
		t.Fatal("data revision reused stale result")
	}
}

func TestPreparedSmartResultRetainsNinePagesAndRejectsClosedStore(t *testing.T) {
	smartResultsCache.Clear()
	app := primarySmartApp(t, "Cafe")
	count := countPreparedSmartRuns(t)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	settings := smart.EnableFreeOnly(smart.Settings{})
	pages := []smart.Page{"", smart.PageAccounts, smart.PageTransactions, smart.PageBudgets,
		smart.PageGoals, smart.PageTodos, smart.PagePlanning, smart.PageAllocate,
		smart.PageSubscriptions, smart.PageBills}
	for _, page := range pages {
		if _, err := preparedSmartInsightsAt(app, time.Monday, settings, page, "en", now); err != nil {
			t.Fatal(err)
		}
	}
	if *count != len(pages) {
		t.Fatalf("first sweep ran %d engines", *count)
	}
	if _, err := preparedSmartInsightsAt(app, time.Monday, settings, "", "en", now); err != nil {
		t.Fatal(err)
	}
	if *count != len(pages) {
		t.Fatal("Dashboard result evicted by one route sweep")
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := preparedSmartInsightsAt(app, time.Monday, settings, "", "en", now); err == nil {
		t.Fatal("warm result concealed closed store")
	}
	if *count != len(pages) {
		t.Fatal("closed store ran engine")
	}
}

func TestMountedSmartPeekAndStripShareOneDashboardEngineRun(t *testing.T) {
	smartResultsCache.Clear()
	app := primarySmartApp(t, "Cafe")
	previous := appstate.Default
	appstate.Default = app
	uistate.SetSmartStripOpen(false)
	count := countPreparedSmartRuns(t)
	f := render.New(t, render.WithQueuedScheduler())
	t.Cleanup(f.Cleanup)
	t.Cleanup(func() { uistate.SetSmartStripOpen(false); appstate.Default = previous })
	t.Cleanup(func() { f.Render(ui.Fragment()); f.Stabilize() })
	f.Render(Fragment(SmartPeekForPath("/"), SmartStripForPath("/")))
	f.Stabilize()
	if *count != 1 {
		t.Fatalf("mounted peek and strip ran engine %d times", *count)
	}
	peek := byTestID(f, "button", "smart-peek-all")
	if peek == nil {
		t.Fatal("mounted peek missing for active findings")
	}
	peek.Click()
	f.Stabilize()
	if byTestID(f, "section", "smart-strip-all") == nil {
		t.Fatal("opening peek did not show strip")
	}
	if *count != 1 {
		t.Fatalf("opening strip reran unchanged engine %d times", *count)
	}
	txn := app.Transactions()[0]
	txn.Desc = "Updated Cafe"
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	// A direct fixture write does not publish the UI event that the real edit
	// flow emits. Drive the same atom the mounted components subscribe to.
	uistate.BumpDataRevision()
	f.Stabilize()
	if *count != 2 {
		t.Fatalf("mounted pair after data edit ran engine %d times, want one fresh pass", *count)
	}
	f.Render(Fragment(SmartPeekForPath("/transactions"), SmartStripForPath("/transactions")))
	f.Stabilize()
	if *count != 3 {
		t.Fatalf("page-specific mounted pair ran engine %d times, want one RunPage pass", *count)
	}
}
