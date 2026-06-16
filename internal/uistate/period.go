//go:build js && wasm

// Package uistate holds small pieces of shared, cross-screen UI state backed by
// framework atoms (e.g. the dashboard time window). It bridges the pure logic
// packages (internal/period) and the view layer without either the app shell or
// the screens owning the state, so they stay in sync through one source of truth.
package uistate

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/GoWebComponents/state"
)

const periodAtomID = "dashboard:period"

// defaultWindow is the initial dashboard selection: the current month.
func defaultWindow() period.Window {
	return period.NewWindow(period.Month, time.Now(), time.Monday)
}

// UsePeriod returns the shared dashboard time-window atom. Every component that
// reads it re-renders when the window changes, so the top-bar control and the
// dashboard widgets stay in lockstep.
func UsePeriod() state.Atom[period.Window] {
	return state.UseAtom(periodAtomID, defaultWindow())
}
