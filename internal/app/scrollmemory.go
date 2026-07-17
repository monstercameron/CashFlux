// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"

	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// routeScroll remembers each route's last scroll offset in the main pane, and
// backNavPending marks that the next route change came from the browser's
// Back/Forward (popstate) rather than a fresh in-app navigation.
var (
	routeScroll    = map[string]float64{}
	backNavPending = false
)

type scrollMemoryProps struct {
	ActivePath string
}

// scrollMemoryHost implements #60's back-navigation contract: leaving a route
// records where the user was; returning to it via Back/Forward restores that
// offset (re-asserted once after deferred below-the-fold content lands), while
// a fresh forward navigation still starts at the top. Filters and the period
// window already persist via their own atoms — scroll was the missing leg.
// Mounted once in the Shell so its hooks keep a constant depth.
func scrollMemoryHost(props scrollMemoryProps) uic.Node {
	path := props.ActivePath

	// One persistent popstate listener flags Back/Forward navigations. It fires
	// before the router re-renders, so the flag is set by the time the
	// path-change effect below runs for the new route.
	uic.UseEffect(func() func() {
		cb := js.FuncOf(func(js.Value, []js.Value) any {
			// Synthetic popstates (palette/shortcut navigation, settings deep
			// links) are forward navigations — only REAL Back/Forward restores.
			w := js.Global().Get("window")
			if w.Get("__cfSyntheticNav").Truthy() {
				w.Set("__cfSyntheticNav", false)
				return nil
			}
			backNavPending = true
			return nil
		})
		js.Global().Get("window").Call("addEventListener", "popstate", cb)
		return func() {
			js.Global().Get("window").Call("removeEventListener", "popstate", cb)
			cb.Release()
		}
	}, "once")

	uic.UseEffect(func() func() {
		scroller := func() js.Value {
			return js.Global().Get("document").Call("querySelector", "main.cf-scroll")
		}
		if backNavPending {
			backNavPending = false
			if top, ok := routeScroll[path]; ok && top > 0 {
				if sc := scroller(); sc.Truthy() {
					sc.Set("scrollTop", top)
				}
				// Below-the-fold content renders after route settle and can grow
				// the page under us; re-assert once when it has had a beat.
				var again js.Func
				again = js.FuncOf(func(js.Value, []js.Value) any {
					if sc := scroller(); sc.Truthy() {
						sc.Set("scrollTop", top)
					}
					again.Release()
					return nil
				})
				js.Global().Call("setTimeout", again, 250)
			}
		}
		return func() {
			// Leaving this route: remember where the user was.
			if sc := scroller(); sc.Truthy() {
				routeScroll[path] = sc.Get("scrollTop").Float()
			}
		}
	}, path)
	return Fragment()
}
