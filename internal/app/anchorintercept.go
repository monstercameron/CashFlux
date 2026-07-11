// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/router"
)

// wireAnchorInterceptor installs one document-level click handler that turns a same-
// origin route <a href> into a client-side router navigation — so no in-app link ever
// triggers a full page reload. A reload throws away all wasm state and (worse) drops the
// in-memory app-lock passcode, forcing a re-unlock. Doing this once at the root fixes
// every page's links at once, and pages can keep plain <a href="…"> anchors (good for
// accessibility: focus, middle-click, open-in-new-tab).
//
// It defers to the browser for the cases where a full navigation is intended: modified
// clicks (Ctrl/Cmd/Shift/Alt or non-left button), target=_blank, download links,
// mailto:/tel:/external hosts, in-page #fragments, an explicit data-native opt-out, and
// any anchor whose own handler already called preventDefault (e.g. a component using
// Prevent). Only paths under the app's base route prefix are intercepted.
func wireAnchorInterceptor() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	origin := js.Global().Get("location").Get("origin").String()
	// Base route prefix ("" locally, e.g. "/CashFlux" on GitHub Pages) — only paths under
	// it are routes the SPA router owns.
	base := strings.TrimSuffix(uistate.RoutePath("/"), "/")

	cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		e := args[0]
		if e.Get("defaultPrevented").Truthy() {
			return nil // a component handler already took this click
		}
		if e.Get("button").Int() != 0 {
			return nil // middle/right click
		}
		if e.Get("metaKey").Truthy() || e.Get("ctrlKey").Truthy() || e.Get("shiftKey").Truthy() || e.Get("altKey").Truthy() {
			return nil // open-in-new-tab / new-window / etc.
		}
		target := e.Get("target")
		if !target.Truthy() {
			return nil
		}
		a := target.Call("closest", "a")
		if !a.Truthy() {
			return nil
		}
		if a.Call("getAttribute", "target").String() == "_blank" ||
			a.Call("hasAttribute", "download").Truthy() ||
			a.Call("hasAttribute", "data-native").Truthy() {
			return nil
		}
		if !a.Call("hasAttribute", "href").Truthy() {
			return nil
		}
		if strings.HasPrefix(a.Call("getAttribute", "href").String(), "#") {
			return nil // in-page fragment
		}
		if !strings.HasPrefix(a.Get("href").String(), origin) {
			return nil // external host / mailto: / tel:
		}
		path := a.Get("pathname").String()
		if base != "" && !strings.HasPrefix(path, base) {
			return nil // not a route the SPA router owns
		}
		e.Call("preventDefault")
		router.Navigate(path + a.Get("search").String() + a.Get("hash").String())
		return nil
	})
	// Bubbling phase, after the framework's delegated handlers, so a component's own
	// Prevent wins. Lives for the app's lifetime; intentionally never released.
	doc.Call("addEventListener", "click", cb)
}
