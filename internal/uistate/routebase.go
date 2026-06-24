// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/routebase"
)

// routeBase is the URL sub-path the app is served under (e.g. "/CashFlux" on a
// GitHub Pages project site), or "" at the server root. It is read once at boot
// from the document's <base href> (set by index.html) so it always matches the
// prefix assets resolve against.
var routeBase string

// InitRouteBase reads the document's <base href> and derives the route prefix.
// Call once before registering routes. With no <base> element (or a root base)
// the prefix is "", making RoutePath a no-op — local dev and custom domains are
// unaffected.
func InitRouteBase() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	base := doc.Call("querySelector", "base")
	if !base.Truthy() {
		return
	}
	// getAttribute returns the raw href as authored ("/CashFlux/" or "/"), not the
	// resolved absolute URL, so it is already the path prefix.
	raw := base.Call("getAttribute", "href")
	if raw.Type() != js.TypeString {
		return
	}
	routeBase = routebase.Normalize(raw.String())
}

// RouteBase returns the active route prefix ("" at the server root).
func RouteBase() string { return routeBase }

// RoutePath prefixes a route path with the base so it matches under a sub-path
// deployment. An empty base returns the path unchanged. Used for both route
// registration and every Navigate target.
func RoutePath(path string) string { return routebase.Join(routeBase, path) }

// LogicalPath strips the base prefix from a raw current pathname so it can be
// compared against unprefixed route literals (active-link checks, breadcrumb
// "are we home", period-aware screen lookup). An empty base is a no-op.
func LogicalPath(rawPath string) string { return routebase.Strip(routeBase, rawPath) }
