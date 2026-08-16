// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate — spotlight.go carries the assistant's "look here" from the chat
// to the screen it is pointing at (PS8).
package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

const spotlightAtomID = "assistant:spotlight"

// Spotlight is what the assistant is currently pointing at.
type Spotlight struct {
	// TestID is the data-testid of the control to ring. Empty means the whole
	// route is the destination and nothing is ringed.
	TestID string
	// Route is where the highlight belongs. The host clears itself when the user
	// navigates elsewhere: a note still claiming to point at a control on another
	// screen is a confidently wrong statement about what somebody is looking at.
	Route string
	// What is the plain-English description, shown beside the ring so the
	// highlight explains itself rather than just glowing.
	What string
	// Nonce changes on every request so pointing at the SAME control twice still
	// re-triggers the highlight. Without it, asking "where is that again?" would
	// silently do nothing.
	Nonce int
}

// Active reports whether anything is being pointed at.
func (s Spotlight) Active() bool { return s.Nonce > 0 }

// UseSpotlight returns the shared atom holding the current highlight.
func UseSpotlight() state.Atom[Spotlight] {
	a := state.UseAtom(spotlightAtomID, Spotlight{})
	capturedSpotlight = a
	spotlightCaptured = true
	return a
}

var (
	capturedSpotlight state.Atom[Spotlight]
	spotlightCaptured bool
	spotlightNonce    int
)

// PointAt asks the UI to highlight a control. Safe to call from outside a render
// (the chat tool loop); a no-op until the host has rendered once.
func PointAt(route, testID, what string) {
	if !spotlightCaptured {
		return
	}
	spotlightNonce++
	capturedSpotlight.Set(Spotlight{Route: route, TestID: testID, What: what, Nonce: spotlightNonce})
}

// ClearSpotlight puts the highlight away.
func ClearSpotlight() {
	if !spotlightCaptured {
		return
	}
	capturedSpotlight.Set(Spotlight{})
}
