// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

// subsDetectPrefsKey is the settings KV key for subscription detection preferences.
// Stored in the PRESERVED settings KV so it survives dataset wipes (the same
// bucket as theme, language, smart settings, etc.).
const subsDetectPrefsKey = "cashflux:subs-detect-prefs"

// LoadSubsDetectPrefs reads the persisted detection preferences. Returns a
// zero SubsDetectPrefs (no ignored categories/types) when not yet set.
func LoadSubsDetectPrefs() SubsDetectPrefs {
	p, err := ParseSubsDetectPrefs(SettingKVGet(subsDetectPrefsKey))
	if err != nil {
		return SubsDetectPrefs{}
	}
	return p
}

// SaveSubsDetectPrefs persists the detection preferences. Persisting via
// SettingKVSet bumps the store mutation revision so dependant views recompute.
func SaveSubsDetectPrefs(p SubsDetectPrefs) {
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	SettingKVSet(subsDetectPrefsKey, string(b))
}
