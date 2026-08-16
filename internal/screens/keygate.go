// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// KeyGateMark labels a control as needing an API key, BEFORE it is pressed (R24).
//
// Every AI-gated control in the app already refused politely when pressed without
// a key. That is the wrong moment: the person has decided to use the feature,
// clicked, and only then been told it is unavailable — which reads as the app
// changing its mind. The audit's rule is that an AI-gated control announces itself
// as one, so the choice to press it is informed.
//
// It renders NOTHING when a key is configured. The mark is information about a
// missing prerequisite, not a permanent badge, and leaving it on afterwards would
// make it wallpaper on the surfaces that use AI most.
func KeyGateMark(hasKey bool) ui.Node {
	if hasKey {
		return Fragment()
	}
	return Span(css.Class("key-gate-mark"), Attr("data-testid", "key-gate-mark"),
		Title(uistate.T("keygate.needsKeyHint")),
		uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
		Span(uistate.T("keygate.needsKey")),
	)
}

// AIKeyConfigured reports whether anything is set up that could answer: a direct
// key, or a backend to proxy through. It exists so the ten surfaces that ask this
// question ask it the same way — the two-part condition was written out by hand at
// each of them, and one of the ten checking only half of it is the sort of bug
// nobody finds by reading.
func AIKeyConfigured() bool {
	app := appstate.Default
	if app == nil {
		return false
	}
	return app.Settings().OpenAIKey != "" || uistate.LoadPrefs().Normalize().BackendActive()
}
