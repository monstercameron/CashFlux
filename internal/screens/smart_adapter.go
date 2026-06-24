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

// aiProviderConfigured reports whether the user has an inference provider set up
// (a stored OpenAI key, or the hosted backend AI). It drives the "needs a
// provider" hint on AI features so the cost story stays honest.
func aiProviderConfigured(app *appstate.App, backendAI bool) bool {
	if backendAI {
		return true
	}
	return app.Settings().OpenAIKey != ""
}
