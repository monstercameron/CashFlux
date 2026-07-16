// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// smartInputCache memoizes buildSmartInput. Gathering the Input copies nine dataset
// slices (including every transaction) on every render, and the Smart strip renders on
// ~10 pages plus the dashboard, anomaly detectors, and digest driver — all with the
// same (data, weekStart). The rule engines read the Input strictly read-only (verified:
// no engine mutates in.<slice>), so sharing one instance is safe. Keyed on the store
// revision (covers all data + base-currency/FX), the week start, and a 1-minute bucket
// for the wall-clock Now — the same 60s staleness the dashboard engine memo accepts for
// time-relative figures. wasm is single-threaded, so no lock is needed.
var smartInputCache = map[string]smartengine.Input{}

// buildSmartInput gathers the live dataset into the pure smartengine.Input the
// rule engines compute over. It reads only through the appstate accessors (no
// hooks), so it is safe to call anywhere in a render. weekStart is passed in by
// the caller (which reads it from the prefs hook at a stable render position).
func buildSmartInput(app *appstate.App, weekStart time.Weekday) smartengine.Input {
	key := strconv.FormatUint(app.Rev(), 10) + "|" + strconv.Itoa(int(weekStart)) + "|" + strconv.FormatInt(time.Now().Unix()/60, 10)
	if v, ok := smartInputCache[key]; ok {
		return v
	}
	if len(smartInputCache) > 4 {
		smartInputCache = map[string]smartengine.Input{}
	}
	v := buildSmartInputRaw(app, weekStart)
	smartInputCache[key] = v
	return v
}

func buildSmartInputRaw(app *appstate.App, weekStart time.Weekday) smartengine.Input {
	s := app.Settings()
	base := s.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return smartengine.Input{
		Now:           time.Now(),
		Base:          base,
		Rates:         currency.Rates{Base: base, Rates: s.FXRates},
		WeekStart:     weekStart,
		Accounts:      app.Accounts(),
		Transactions:  app.Transactions(),
		Categories:    app.Categories(),
		Budgets:       app.Budgets(),
		Goals:         app.Goals(),
		Recurring:     app.Recurring(),
		Members:       app.Members(),
		Tasks:         app.Tasks(),
		Subscriptions: app.Cancellations(),
		// TX9: matched bill occurrences read as PAID, so the missing-transaction
		// detector skips what a bill-match link already settled.
		PaidOccurrences: app.BillMatchPaidOccurrences(),
	}
}

// runSmart computes the active insights (enabled features, not dismissed) over
// the live data for the given settings, sorted for display.
func runSmart(app *appstate.App, weekStart time.Weekday, s smart.Settings) []smart.Insight {
	return smartengine.Run(buildSmartInput(app, weekStart), s)
}

// runAnomalyDetectors runs the four SMART anomaly detectors (A1/T2/T6/T7) with
// all Free features force-enabled, so they always fire regardless of the user's
// per-feature opt-in state. It is the shared compute kernel used by both the
// Insights screen (smartAnomalyHighlights) and the Anomaly Hub dashboard widget
// (anomalyHubWidget) — only the row renderer differs between the two call sites.
func runAnomalyDetectors(app *appstate.App, weekStart time.Weekday) []smart.Insight {
	in := buildSmartInput(app, weekStart)
	freeSettings := smart.EnableFreeOnly(smart.Settings{})
	all := smartengine.Run(in, freeSettings)
	anomalyCodes := map[string]bool{
		"SMART-A1": true,
		"SMART-T2": true,
		"SMART-T6": true,
		"SMART-T7": true,
	}
	// Honour dismissed flags: the detectors run unconditionally (no opt-in gate), but a
	// flag the user (or the agent, on request) has dismissed should stay hidden until its
	// situation changes. Dismissals persist in the Smart settings, keyed by insight key.
	dismissed := uistate.LoadSmartSettings()
	var flagged []smart.Insight
	for _, ins := range all {
		if anomalyCodes[ins.Feature] && !dismissed.IsDismissed(ins.Key) {
			flagged = append(flagged, ins)
		}
	}
	return flagged
}

// aiProviderConfigured reports whether the user has an inference provider set up
// (a stored OpenAI key, or the hosted backend AI). It drives the "needs a
// provider" hint on AI features so the cost story stays honest.
func aiProviderConfigured(app *appstate.App, backendAI bool) bool {
	if backendAI {
		return true
	}
	return app.Settings().OpenAIKey != ""
}
