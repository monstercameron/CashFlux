// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "syscall/js"

// NavigateTo performs an SPA navigation from a NON-RENDER context — keyboard
// shortcuts, command-palette actions, other JS callbacks. router.Navigate reads
// the router atom, which is a framework hook and PANICS outside a component
// render ("GoUseAtom called outside component context" — the Ctrl+K palette
// crash). This takes the same path OpenGlobalSettingsAt always has: push the
// history entry, then fire a synthetic popstate so the history router
// re-resolves. The synthetic-nav flag lets popstate listeners that care about
// REAL Back/Forward (the per-route scroll memory) tell the two apart.
func NavigateTo(path string) {
	js.Global().Get("window").Set("__cfSyntheticNav", true)
	js.Global().Get("history").Call("pushState", js.Null(), "", RoutePath(path))
	js.Global().Call("dispatchEvent", js.Global().Get("PopStateEvent").New("popstate"))
}
