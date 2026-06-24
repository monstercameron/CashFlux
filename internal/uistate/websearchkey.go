// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "syscall/js"

// webSearchKeyStore holds an optional API key for the chat's web_search tool, for
// users who want paid/higher-limit search access. Kept on-device, separate from the
// dataset; empty means use the free keyless endpoint.
const webSearchKeyStore = "cashflux:websearch-key"

// PersistWebSearchKey saves (or clears, when blank) the web-search API key.
func PersistWebSearchKey(key string) {
	if key == "" {
		js.Global().Get("localStorage").Call("removeItem", webSearchKeyStore)
		return
	}
	js.Global().Get("localStorage").Call("setItem", webSearchKeyStore, key)
}

// LoadWebSearchKey reads the web-search API key, or "" when none is set.
func LoadWebSearchKey() string {
	v := js.Global().Get("localStorage").Call("getItem", webSearchKeyStore)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}
