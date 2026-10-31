// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
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
var smartInputCache = newViewCache[smartengine.Input](1)

// Free findings are consumed by both the anomaly and next-action sections. Share
// their engine pass; each caller still applies the current dismissals immediately.
var freeFindingsCache = newViewCache[[]smart.Insight](1)

// accountsPageInsightsCache memoizes the Accounts page insight pass. The pass is the costly part
// of an Accounts list render, and its answer depends only on the same context as the input plus
// the Smart settings, so a filter change or hover does not run the engine again.
var accountsPageInsightsCache = newViewCache[[]smart.Insight](1)

func smartContextKey(app *appstate.App, weekStart time.Weekday, now time.Time) string {
	return revKey(app) + "|" + strconv.Itoa(int(weekStart)) + "|" + strconv.FormatInt(now.Unix()/60, 10)
}

// buildSmartInput gathers the live dataset into the pure smartengine.Input the
// rule engines compute over. It reads only through the appstate accessors (no
// hooks), so it is safe to call anywhere in a render. weekStart is passed in by
// the caller (which reads it from the prefs hook at a stable render position).
func buildSmartInput(app *appstate.App, weekStart time.Weekday) smartengine.Input {
	return buildSmartInputAt(app, weekStart, time.Now())
}

func buildSmartInputAt(app *appstate.App, weekStart time.Weekday, now time.Time) smartengine.Input {
	return memoByRev(smartInputCache, smartContextKey(app, weekStart, now), func() smartengine.Input {
		return buildSmartInputRawAt(app, weekStart, now)
	})
}

// buildSmartInputRaw is the uncached seam used by engine parity tests.
func buildSmartInputRaw(app *appstate.App, weekStart time.Weekday) smartengine.Input {
	return buildSmartInputRawAt(app, weekStart, time.Now())
}

func buildSmartInputRawAt(app *appstate.App, weekStart time.Weekday, now time.Time) smartengine.Input {
	s := app.Settings()
	base := s.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return smartengine.Input{
		Ledger:        budgetLedgerForApp(app),
		Now:           now,
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

func freeSmartFindings(app *appstate.App, weekStart time.Weekday, now time.Time) []smart.Insight {
	key := smartContextKey(app, weekStart, now) + "|" + string(uistate.ActiveLanguage())
	return memoByRev(freeFindingsCache, key, func() []smart.Insight {
		return smartengine.Run(buildSmartInputAt(app, weekStart, now), smart.EnableFreeOnly(smart.Settings{}))
	})
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
	all := freeSmartFindings(app, weekStart, time.Now())
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

// runActionableFindings returns every free-tier finding the user has not
// dismissed, for surfaces that rank ACTIONS rather than flag anomalies (WF-SM3).
//
// It is deliberately separate from runAnomalyDetectors: that one filters to the
// four anomaly codes because a "flagged activity" list is about things that look
// wrong, while a "what to do next" list is about things worth doing, and most of
// those are not anomalies at all — a subscription that crept up in price is not
// an anomaly, it is a decision.
func runActionableFindings(app *appstate.App, weekStart time.Weekday) []smart.Insight {
	all := freeSmartFindings(app, weekStart, time.Now())
	dismissed := uistate.LoadSmartSettings()
	out := make([]smart.Insight, 0, len(all))
	for _, ins := range all {
		if !dismissed.IsDismissed(ins.Key) {
			out = append(out, ins)
		}
	}
	return out
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

// accountsPageInsights runs the Accounts page's insight pass once per data revision, minute,
// week start and Smart settings, rather than on every list render.
// accountsPageInsights runs the Accounts page's insight pass once per data revision, minute,
// week start and Smart settings, rather than on every list render.
func accountsPageInsights(app *appstate.App, in smartengine.Input, s smart.Settings, weekStart time.Weekday) []smart.Insight {
	key := smartContextKey(app, weekStart, time.Now()) + "|" + fmt.Sprint(s)
	return memoByRev(accountsPageInsightsCache, key, func() []smart.Insight {
		return smartengine.RunPage(in, s, smart.PageAccounts)
	})
}
