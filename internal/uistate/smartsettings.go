// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/smart"
)

// smartSettingsKey holds the user's SMART-series opt-in state (which features are
// enabled, which insights are dismissed) in the PRESERVED settings KV — these are
// preferences, so they survive a dataset wipe like theme/language/prefs do.
const smartSettingsKey = "cashflux:smart-settings"

// LoadSmartSettings reads the persisted SMART opt-in settings. A missing or
// unparseable value yields the zero Settings (everything OFF — the safe default,
// since the series is strictly opt-in).
func LoadSmartSettings() smart.Settings {
	raw := SettingKVGet(smartSettingsKey)
	if raw == "" {
		return smart.Settings{}
	}
	var s smart.Settings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return smart.Settings{}
	}
	return s
}

// SaveSmartSettings persists the SMART opt-in settings. Persisting bumps the
// store mutation revision (via SettingKVSet), so memoized views recompute and the
// freshly toggled feature surfaces or disappears without a manual reload.
func SaveSmartSettings(s smart.Settings) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	SettingKVSet(smartSettingsKey, string(b))
}

// SetSmartFeatureEnabled opts a single feature in or out and persists, returning
// the updated settings for the caller to render against immediately.
func SetSmartFeatureEnabled(code string, on bool) smart.Settings {
	s := LoadSmartSettings().SetEnabled(code, on)
	SaveSmartSettings(s)
	return s
}

// DismissSmartInsight records that an insight was dismissed and persists.
func DismissSmartInsight(key string) smart.Settings {
	s := LoadSmartSettings().Dismiss(key)
	SaveSmartSettings(s)
	return s
}

// RestoreSmartInsight un-dismisses an insight (the "show dismissed again"
// affordance) and persists.
func RestoreSmartInsight(key string) smart.Settings {
	s := LoadSmartSettings().Restore(key)
	SaveSmartSettings(s)
	return s
}
