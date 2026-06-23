//go:build js && wasm

// Package uistate holds small pieces of shared, cross-screen UI state backed by
// framework atoms (e.g. the dashboard time window). It bridges the pure logic
// packages (internal/period) and the view layer without either the app shell or
// the screens owning the state, so they stay in sync through one source of truth.
package uistate

import (
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	periodAtomID  = "dashboard:period"
	periodStoreID = "cashflux:period-res"
)

// defaultWindow is the initial dashboard selection. It restores the full
// persisted window (resolution + From/To anchors) when a recent one is stored,
// so /reports reopens on the last-viewed period after a hard reload. When no
// persisted window exists, is stale (>366 days old), or is malformed it falls
// back to the current period at the saved resolution — the prior behaviour.
func defaultWindow() period.Window {
	now := time.Now()
	if w, ok := LoadPeriodWindow(now); ok {
		// Honour the user's week-start preference even if it changed since the
		// window was saved.
		return w.WithWeekStart(loadPrefs().WeekStartWeekday())
	}
	return period.NewWindow(loadResolution(), now, loadPrefs().WeekStartWeekday())
}

// UsePeriod returns the shared dashboard time-window atom. Every component that
// reads it re-renders when the window changes, so the top-bar control and the
// dashboard widgets stay in lockstep.
func UsePeriod() state.Atom[period.Window] {
	return state.UseAtom(periodAtomID, defaultWindow())
}

// PersistResolution saves the dashboard resolution so the user's preferred
// granularity survives reloads.
func PersistResolution(r period.Resolution) {
	if !r.Valid() {
		return
	}
	js.Global().Get("localStorage").Call("setItem", periodStoreID, string(r))
}

// loadResolution reads the saved resolution, defaulting to Month when absent or
// invalid.
func loadResolution() period.Resolution {
	v := js.Global().Get("localStorage").Call("getItem", periodStoreID)
	if v.IsNull() || v.IsUndefined() {
		return period.Month
	}
	if r := period.Resolution(v.String()); r.Valid() {
		return r
	}
	return period.Month
}
