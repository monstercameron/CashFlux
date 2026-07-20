// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/recurdiscover"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// The Bills & recurring surface is ONE page split into several components so
// their hooks stay isolated — but that split made each of them re-derive the same
// model from the same store on the same frame. A single render ran
// buildDiscoverTxns three times, recurdiscover.Discover twice and computeRecurView
// twice, each a full sweep of the transaction history, all producing byte-identical
// results. These caches collapse that to once per frame.
//
// Keying follows the memoByRev contract: the store revision, PLUS the inputs that
// live outside the store. Discovery reads the day (its liveness/expiry windows) and
// the user's discovery pins (persisted in prefs, invisible to app.Rev()), so both
// ride in the key — a pin change must invalidate immediately or "Not recurring"
// would appear to do nothing until the next data write.
var (
	discoverTxnCache = map[string][]recurdiscover.Txn{}
	commitmentCache  = map[string][]recurdiscover.Commitment{}
	discoverCache    = map[string]recurdiscover.Result{}
	recurViewCache   = map[string]recurView{}
)

// dayKey renders an instant at day granularity — the resolution every window on
// this surface actually uses.
func dayKey(now time.Time) string { return now.Format("2006-01-02") }

// pinsKey fingerprints the persisted discovery pins so a pin edit drops the
// discovery caches even though nothing in the store changed.
func pinsKey(p uistate.RecurPins) string {
	var b strings.Builder
	b.WriteString(strings.Join(p.Suppressed, ","))
	b.WriteByte('|')
	for _, pr := range p.NeverMerge {
		b.WriteString(pr[0] + ">" + pr[1] + ";")
	}
	b.WriteByte('|')
	for _, pr := range p.ForceMerge {
		b.WriteString(pr[0] + ">" + pr[1] + ";")
	}
	return b.String()
}

// discoverTxns is buildDiscoverTxns memoized for the frame.
func discoverTxns(app *appstate.App, rates currency.Rates) []recurdiscover.Txn {
	return memoByRev(discoverTxnCache, revKey(app)+"|"+rates.Base, func() []recurdiscover.Txn {
		return buildDiscoverTxns(app, rates)
	})
}

// discoverCommitments is rhyCommitments memoized for the frame.
func discoverCommitments(app *appstate.App, rates currency.Rates) []recurdiscover.Commitment {
	return memoByRev(commitmentCache, revKey(app)+"|"+rates.Base, func() []recurdiscover.Commitment {
		return rhyCommitments(app, rates)
	})
}

// rhyDiscover runs the deterministic discovery pipeline ONCE per frame. The hero,
// the review strip and the findings strip all need it; before this they each ran
// their own full sweep of the transaction history and got the same answer.
func rhyDiscover(app *appstate.App, rates currency.Rates, now time.Time) recurdiscover.Result {
	pins := uistate.LoadRecurPins()
	key := revKey(app) + "|" + rates.Base + "|" + dayKey(now) + "|" + pinsKey(pins)
	return memoByRev(discoverCache, key, func() recurdiscover.Result {
		return recurdiscover.Discover(discoverTxns(app, rates), discoverCommitments(app, rates),
			loadRecurPins(), recurdiscover.Options{Now: now})
	})
}

// recurViewOf is computeRecurView memoized for the frame — the page shell and the
// roster both need the shared model, and it sweeps the whole ledger to build it.
func recurViewOf(app *appstate.App, now time.Time) recurView {
	return memoByRev(recurViewCache, revKey(app)+"|"+dayKey(now), func() recurView {
		return computeRecurView(app, now)
	})
}
