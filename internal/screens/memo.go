// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// revKey returns the store's mutation revision as a string — the base cache key for a
// derived view that depends only on stored data. Append any non-store inputs (member,
// date window) to it before passing to memoByRev.
func revKey(app *appstate.App) string { return strconv.FormatUint(app.Rev(), 10) }

// useAfterSettle returns false on a component's first paint and true ~300ms later (past
// the 160ms route cross-fade), via a Go timer. Gate a page's heavy secondary / below-
// the-fold content on it so the primary content paints immediately on mount and the
// rest fills in once the page has settled — keeping expensive work (full-transaction
// scans, chart builds, long lists) off the mount critical path. `key` just names the
// deferral for the effect's dependency; hooks stay unconditional either way.
func useAfterSettle(key string) bool {
	_ = key // descriptive only; the effect re-arms via the ready state below
	ready := ui.UseState(false)
	// Depend on ready itself so the effect fires on mount (ready=false → schedule the
	// flip) AND re-arms on every re-mount (navigating away and back gives a fresh
	// ready=false), then no-ops once flipped. A constant dep could fire only once and
	// leave deferred content missing on a return visit.
	ui.UseEffect(func() func() {
		if !ready.Get() {
			time.AfterFunc(300*time.Millisecond, func() { ready.Set(true) })
		}
		return nil
	}, ready.Get())
	return ready.Get()
}

// memoByRev caches a derived view by a string key. The key MUST include the store's
// mutation revision (app.Rev()) so an entry is dropped the moment any data changes;
// include any other inputs the view depends on that aren't reflected in the store
// (e.g. the active member, a date window). It keeps a handful of recent keys — enough
// to cover the several identical recomputations a single render triggers across
// separate tile components — and resets once it grows past the cap so stale-revision
// keys don't accumulate. Single-threaded wasm, so no locking.
//
// This exists because the compute*View functions each aggregate over the full ledger,
// yet a single surface render calls them once per tile (investments has ~6 tiles, each
// calling computeInvestView). Memoizing collapses those to one computation per frame.
func memoByRev[T any](cache map[string]T, key string, compute func() T) T {
	if v, ok := cache[key]; ok {
		return v
	}
	if len(cache) > 8 {
		clear(cache)
	}
	v := compute()
	cache[key] = v
	return v
}
