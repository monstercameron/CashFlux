// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens — spotlight_host.go draws the ring the assistant points with
// (PS8).
package screens

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// SpotlightHost highlights whatever the assistant is pointing at. Mounted once in
// the shell; it renders nothing until something is pointed at.
//
// It rings the control in place rather than dimming the page around it. A modal
// spotlight would block the very interaction it is describing — the whole point is
// that the person can now DO the thing, and an overlay they have to dismiss first
// puts a step back in front of them. It also means the highlight can simply sit
// there while they work, which is what somebody following an instruction actually
// wants.
func SpotlightHost() ui.Node {
	spot := uistate.UseSpotlight().Get()
	ui.UseEffect(func() func() {
		clearSpotlightMarks()
		if spot.Active() && spot.TestID != "" {
			markSpotlight(spot.TestID)
		}
		// The mark is removed when the highlight changes or the host unmounts, so
		// a ring never outlives the instruction that put it there.
		return clearSpotlightMarks
	}, spot.Nonce)

	if !spot.Active() || spot.What == "" {
		return Fragment()
	}
	// The label is a status rather than a dialog: it announces itself once to a
	// screen reader and never steals focus, because focus belongs on the control
	// being pointed at.
	return Div(css.Class("spot-note"), Attr("data-testid", "spotlight-note"),
		Attr("role", "status"), Attr("aria-live", "polite"),
		uistate.T("spotlight.pointing", spot.What))
}

// spotlightClass is the class added to the highlighted element. It is added to the
// real control rather than to a copy so the ring tracks the element through
// scrolling, resizing and re-layout for free.
const spotlightClass = "is-spotlit"

// markSpotlight rings the control with the given test id and scrolls it into view.
// A control that is off-screen is not being pointed at in any useful sense.
func markSpotlight(testID string) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	el := doc.Call("querySelector", "[data-testid=\""+testID+"\"]")
	if !el.Truthy() {
		return
	}
	el.Get("classList").Call("add", spotlightClass)
	el.Call("scrollIntoView", map[string]any{"block": "center", "behavior": "smooth"})
}

// clearSpotlightMarks removes every ring. It clears ALL of them rather than the
// one it added, so a highlight can never be orphaned by a re-render that happened
// between marking and clearing.
func clearSpotlightMarks() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	marked := doc.Call("querySelectorAll", "."+spotlightClass)
	for i := 0; i < marked.Length(); i++ {
		marked.Index(i).Get("classList").Call("remove", spotlightClass)
	}
}
