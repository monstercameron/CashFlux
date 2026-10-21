// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashboardview"
)

var dashboardGoalFactsCache = newViewCache[dashboardview.GoalFacts]()

// dashboardGoalFacts shares one task index and goal evaluation between dashboard
// tiles. Store/browser revisions cover goals, tasks and FX; the day covers timing.
func dashboardGoalFacts(app *appstate.App, now time.Time) dashboardview.GoalFacts {
	return memoByRev(dashboardGoalFactsCache, revKey(app)+"|"+now.Format("2006-01-02"), func() dashboardview.GoalFacts {
		settings := app.Settings()
		base := settings.BaseCurrency
		if base == "" {
			base = "USD"
		}
		return dashboardview.DeriveGoals(app.Goals(), app.Tasks(), currency.Rates{Base: base, Rates: settings.FXRates}, now)
	})
}
