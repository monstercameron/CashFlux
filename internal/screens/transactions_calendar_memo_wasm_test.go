// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/txncalendarview"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type txnCalendarMemoFixtureProps struct {
	Calendar txnCalendarProps
	Filter   txnfilter.Criteria
	Lens     []string
	Labels   txncalendarview.Labels
	Now      time.Time
	Capture  func(*txncalendarview.Month)
}

func txnCalendarMemoFixture(p txnCalendarMemoFixtureProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	month := useTxnCalendarMonth(p.Calendar, p.Filter, p.Lens, p.Labels, p.Now)
	p.Capture(month)
	return P(month.Caption)
}

func TestTxnCalendarMemoTracksScopeDatePreferencesAndFX(t *testing.T) {
	app := newAssistantBriefingApp(t, 1)
	var got *txncalendarview.Month
	props := txnCalendarMemoFixtureProps{Calendar: txnCalendarProps{App: app, Base: "USD", Shown: app.Transactions()},
		Now: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Labels: calendarLabels(prefs.Prefs{DateStyle: prefs.DateISO}),
		Capture: func(value *txncalendarview.Month) { got = value }}
	f := render.New(t)
	defer f.Cleanup()
	renderValue := func() {
		f.Render(ui.CreateElement(txnCalendarMemoFixture, props))
		f.Stabilize()
	}
	renderValue()
	first := got
	if got.Err != nil {
		t.Fatal(got.Err)
	}
	renderValue()
	if got != first {
		t.Fatal("unchanged render rebuilt the calendar projection")
	}
	props.Lens = []string{"A B"}
	renderValue()
	first = got
	props.Lens = []string{"A", "B"}
	renderValue()
	if got == first {
		t.Fatal("quoted member lens keys collided")
	}
	first = got
	props.Labels.Prefs.DateStyle = prefs.DateUS
	renderValue()
	if got == first {
		t.Fatal("date preferences retained formatted calendar labels")
	}
	first = got
	props.Filter.From, props.Filter.To = "2026-10-19", "2026-10-19"
	renderValue()
	if got == first {
		t.Fatal("date filter retained the previous calendar caption")
	}
	first = got
	props.Now = props.Now.AddDate(0, 0, 1)
	renderValue()
	if got == first {
		t.Fatal("calendar day retained yesterday's Today markers")
	}
	settings := app.Settings()
	settings.FXRates = map[string]float64{}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	renderValue()
	if got.Err != nil || got.RateIssues == 0 {
		t.Fatalf("removed FX rate lost the projection's omission warning: err=%v issues=%d", got.Err, got.RateIssues)
	}
	settings.FXRates = map[string]float64{"EUR": 3}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	renderValue()
	if got.Err != nil || got.RateIssues != 0 {
		t.Fatalf("restored FX rate retained an obsolete warning: err=%v issues=%d", got.Err, got.RateIssues)
	}
	first = got
	uistate.BumpDataRevision()
	f.Stabilize()
	if got == first {
		t.Fatal("browser data revision retained stale calendar state")
	}
	// Conversions remain owned by the projection's current app settings.
	for _, week := range got.Weeks {
		for _, day := range week.Days {
			if day.DateKey == "2026-10-19" && day.Rates.Rates["EUR"] != 3 {
				t.Fatal("calendar retained old FX in day props")
			}
		}
	}
}
