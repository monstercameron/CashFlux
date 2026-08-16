// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsSpotlight(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/spotlight"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// agToolsSpotlight lets the assistant POINT at a control instead of describing one
// (PS8).
//
// "Click the button in the top right of the accounts page" is the weakest possible
// answer to "how do I add an account?", because the person is looking at the app
// right now and a paragraph about a button is strictly worse than the button. This
// takes them to the screen and rings the control.
//
// The destinations are a curated list rather than the live DOM. A model given the
// whole page will confidently point at the wrong thing; a model given fourteen
// named destinations either finds one or says it cannot — and the second failure is
// recoverable while the first is not.
func agToolsSpotlight(app *appstate.App, base string, rates currency.Rates) []chatTool {
	_ = app
	_ = base
	_ = rates
	return []chatTool{
		{
			spec: ai.FunctionTool("show_me",
				"Take the user to a control and highlight it, instead of describing where it is. Use "+
					"whenever they ask how or where to do something the app supports. Pass their own words; "+
					"if nothing matches, say so plainly rather than describing a button that may not exist. "+
					"Known destinations: "+strings.Join(spotlight.Names(), ", ")+".",
				json.RawMessage(`{"type":"object","properties":{"what":{"type":"string","description":"what they want to do, in their words, or one of the known destination names"}},"required":["what"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					What string `json:"what"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read what to point at."
				}
				want := strings.TrimSpace(a.What)
				if want == "" {
					return "Point at what?"
				}
				target, ok := spotlight.Get(want)
				if !ok {
					target, ok = spotlight.Find(want)
				}
				if !ok {
					return "There's no control I can point at for that. Say so plainly and explain it in words instead — " +
						"don't describe a button you aren't sure exists."
				}
				navigateTo(target.Route)
				uistate.PointAt(target.TestID, target.What)
				if target.TestID == "" {
					return "Taken them to " + target.Route + ", which is where they " + target.What +
						". Tell them what they're looking at in one sentence."
				}
				return "Taken them to " + target.Route + " and highlighted the control to " + target.What +
					". Tell them it's highlighted and what to do next, in one sentence."
			},
		},
	}
}

// navigateTo moves the app to a route from outside a component render. It uses the
// history API and a popstate event, which is the same path the app's own links
// take, so the router reacts identically whether a person or the assistant asked.
func navigateTo(route string) {
	path := uistate.RoutePath(route)
	history := js.Global().Get("history")
	if !history.Truthy() {
		return
	}
	history.Call("pushState", nil, "", path)
	ev := js.Global().Get("PopStateEvent").New("popstate")
	js.Global().Call("dispatchEvent", ev)
}
