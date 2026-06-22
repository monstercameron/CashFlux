//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// wireOnlineStatus seeds the shared online state from navigator.onLine and keeps
// it current by listening for the window "online"/"offline" events, so the top-bar
// OfflineIndicator reflects connectivity live. Boot-safe: a missing navigator just
// leaves the default (online).
func wireOnlineStatus() {
	win := js.Global().Get("window")
	if !win.Truthy() {
		return
	}
	if nav := js.Global().Get("navigator"); nav.Truthy() {
		uistate.SetOnline(nav.Get("onLine").Truthy())
	}
	onOnline := js.FuncOf(func(js.Value, []js.Value) any { uistate.SetOnline(true); return nil })
	onOffline := js.FuncOf(func(js.Value, []js.Value) any { uistate.SetOnline(false); return nil })
	win.Call("addEventListener", "online", onOnline)
	win.Call("addEventListener", "offline", onOffline)
}
