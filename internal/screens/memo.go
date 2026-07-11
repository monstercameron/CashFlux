// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/appstate"
)

// revKey returns the store's mutation revision as a string — the base cache key for a
// derived view that depends only on stored data. Append any non-store inputs (member,
// date window) to it before passing to memoByRev.
func revKey(app *appstate.App) string { return strconv.FormatUint(app.Rev(), 10) }

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
