// SPDX-License-Identifier: MIT

// Pure helpers for keeping this device's server credentials out of anything
// that leaves it. No build tag and no syscall/js, so they unit-test natively —
// the project rule is that logic lives in platform-independent code and the
// wasm layer is a thin shell over it (see CLAUDE.md "Clean architecture").
package app

import (
	"encoding/json"
	"strings"
)

// connectionPrefKeys are the fields of cashflux:prefs that describe THIS
// device's connection to a server rather than anything about the workspace:
// the endpoint, the bearer token, and the CSRF token.
//
// They exist as a named list because prefs travels — it is one of
// perWorkspaceKeys, so it is bundled into every workspace blob and into every
// exported workspace file. That meant an export written to the user's Downloads
// carried a live access token in plain text (wsExport's own comment claimed the
// file "carries no secrets"), and importing somebody else's file silently
// repointed the app at their server under their credential. Neither belongs in
// a file whose whole purpose is to be moved between machines and shared.
var connectionPrefKeys = []string{"serverUrl", "serverToken", "serverCsrf"}

// stripConnectionPrefs returns bundle with the connection fields removed from
// its prefs entry, leaving every other preference intact.
//
// It edits the JSON generically rather than round-tripping through prefs.Prefs
// so a preference this package does not know about cannot be silently dropped
// on the way through — losing a user's settings to a security fix would be its
// own bug. A bundle with no prefs, or prefs that will not parse, is returned
// unchanged except that unparseable prefs are dropped entirely: something we
// cannot read is something we cannot promise is clean.
func stripConnectionPrefs(bundle map[string]string) map[string]string {
	out := make(map[string]string, len(bundle))
	for k, v := range bundle {
		out[k] = v
	}
	raw, ok := out[prefsStoreKey]
	if !ok || strings.TrimSpace(raw) == "" {
		return out
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		delete(out, prefsStoreKey)
		return out
	}
	for _, key := range connectionPrefKeys {
		delete(fields, key)
	}
	cleaned, err := json.Marshal(fields)
	if err != nil {
		delete(out, prefsStoreKey)
		return out
	}
	out[prefsStoreKey] = string(cleaned)
	return out
}

// prefsStoreKey is the canonical localStorage key holding this device's prefs.
const prefsStoreKey = "cashflux:prefs"

// stripConnectionPrefsJSON removes the connection fields from a raw prefs JSON
// value, for the callers that hold the prefs string on its own rather than
// inside a workspace bundle (the full backup envelope).
//
// Empty in, empty out. Unparseable in, EMPTY out: a prefs value we cannot read
// is one we cannot promise is clean, and losing appearance settings is a far
// smaller harm than writing an unexamined credential into a file.
func stripConnectionPrefsJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return stripConnectionPrefs(map[string]string{prefsStoreKey: raw})[prefsStoreKey]
}
