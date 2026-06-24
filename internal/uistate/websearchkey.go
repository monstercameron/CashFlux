// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/browserstore"

// webSearchKeyStore holds an optional API key for the chat's web_search tool, for
// users who want paid/higher-limit search access. Kept on-device in the browser
// store (IndexedDB), separate from the dataset; empty means use the free keyless
// endpoint.
const webSearchKeyStore = "cashflux:websearch-key"

// PersistWebSearchKey saves (or clears, when blank) the web-search API key.
func PersistWebSearchKey(key string) {
	if key == "" {
		browserstore.Remove(webSearchKeyStore)
		return
	}
	browserstore.Set(webSearchKeyStore, key)
}

// LoadWebSearchKey reads the web-search API key, or "" when none is set.
func LoadWebSearchKey() string { return browserstore.GetString(webSearchKeyStore) }
