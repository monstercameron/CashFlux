// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"time"

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

// SetSmartCadence sets a feature's run schedule (when it runs) and persists.
func SetSmartCadence(code string, c smart.Cadence) smart.Settings {
	s := LoadSmartSettings().SetCadence(code, c)
	SaveSmartSettings(s)
	return s
}

// SetSmartMuted snoozes/un-snoozes a feature (without changing its opt-in or
// schedule) and persists.
func SetSmartMuted(code string, on bool) smart.Settings {
	s := LoadSmartSettings().SetMuted(code, on)
	SaveSmartSettings(s)
	return s
}

// MarkSmartRun stamps a feature's last-run time (after a manual or scheduled run)
// and persists, so cadence due/freshness survives reloads.
func MarkSmartRun(code string, now time.Time) smart.Settings {
	s := LoadSmartSettings().MarkRun(code, now)
	SaveSmartSettings(s)
	return s
}

// SetSmartResult caches an AI feature's produced text (and stamps its run time)
// and persists, so a scheduled/manual AI result shows between renders without
// re-spending.
func SetSmartResult(code, text string, now time.Time) smart.Settings {
	s := LoadSmartSettings().SetResult(code, text).MarkRun(code, now)
	SaveSmartSettings(s)
	return s
}

// SetSmartDensity sets the global "how much smart weaves into the app" dial and
// persists.
func SetSmartDensity(d smart.Density) smart.Settings {
	s := LoadSmartSettings().SetDensity(d)
	SaveSmartSettings(s)
	return s
}

// EnableAllSmart opts into every catalog feature at once and persists.
func EnableAllSmart() smart.Settings {
	s := LoadSmartSettings().EnableAll()
	SaveSmartSettings(s)
	return s
}

// DisableAllSmart opts out of every feature at once and persists (keeping
// schedules/mutes so re-enabling restores intent).
func DisableAllSmart() smart.Settings {
	s := LoadSmartSettings().DisableAll()
	SaveSmartSettings(s)
	return s
}
