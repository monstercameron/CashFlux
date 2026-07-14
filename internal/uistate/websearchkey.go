// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

// webSearchKeyStore holds an optional API key for the chat's web_search tool, for
// users who want paid/higher-limit search access. It lives in the SQLite dataset's
// settings KV (preserved across a wipe) like every other setting — a legacy standalone
// browserstore value migrates in on first read. Empty means use the free keyless
// endpoint.
const webSearchKeyStore = "cashflux:websearch-key"

// PersistWebSearchKey saves (or clears, when blank) the web-search API key.
func PersistWebSearchKey(key string) {
	if key == "" {
		SettingKVDelete(webSearchKeyStore)
		return
	}
	SettingKVSet(webSearchKeyStore, key)
}

// LoadWebSearchKey reads the web-search API key, or "" when none is set.
func LoadWebSearchKey() string { return SettingKVGet(webSearchKeyStore) }
