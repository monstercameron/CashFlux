// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
)

// buildSmartInput gathers the live dataset into the pure smartengine.Input the
// rule engines compute over. It reads only through the appstate accessors (no
// hooks), so it is safe to call anywhere in a render. weekStart is passed in by
// the caller (which reads it from the prefs hook at a stable render position).
func buildSmartInput(app *appstate.App, weekStart time.Weekday) smartengine.Input {
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
	var flagged []smart.Insight
	for _, ins := range all {
		if anomalyCodes[ins.Feature] {
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
