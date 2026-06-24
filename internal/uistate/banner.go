// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/theme"
)

const bannerStoreID = "cashflux:banner"

// LoadBanner returns the saved dashboard banner (kept in its own localStorage
// slot, separate from the theme, since an uploaded image can be large). An empty
// or unreadable slot yields no banner.
func LoadBanner() theme.Banner {
	v := js.Global().Get("localStorage").Call("getItem", bannerStoreID)
	if v.IsNull() || v.IsUndefined() {
		return theme.Banner{}
	}
	var b theme.Banner
	if err := json.Unmarshal([]byte(v.String()), &b); err != nil {
		return theme.Banner{}
	}
	return b
}

// PersistBanner saves the dashboard banner to localStorage.
func PersistBanner(b theme.Banner) {
	data, err := json.Marshal(b)
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", bannerStoreID, string(data))
}

// ApplyBanner reflects the banner onto the document root: it sets the
// --banner-bg custom property to the CSS background value and toggles a
// data-banner="on"/"off" attribute the stylesheet uses to show or collapse the
// dashboard banner band. Call it at boot and whenever the banner changes; the
// band repaints with no component re-render.
func ApplyBanner(b theme.Banner) {
	root := js.Global().Get("document").Get("documentElement")
	if root.IsNull() || root.IsUndefined() {
		return
	}
	if b.None() {
		root.Call("setAttribute", "data-banner", "off")
		return
	}
	root.Get("style").Call("setProperty", "--banner-bg", b.CSS())
	root.Call("setAttribute", "data-banner", "on")
}
