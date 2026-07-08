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

// LoadSmartSettings reads the persisted SMART opt-in settings. It is a PURE
// read: it never writes to the KV, eliminating the pre-init clobber race that
// existed when the store may not yet be ready.
//
// Empty KV (first session, or store not yet initialised): returns the
// EnableFreeOnly defaults WITHOUT persisting them. InitSmartSettings is
// responsible for the one-time persist; it must be called from the app boot
// sequence after the store is confirmed ready. Even if InitSmartSettings is
// never reached (e.g. an unusual boot path), the C254 "Free features on"
// contract is satisfied because IsEnabled falls back to the tier default.
//
// Unparseable stored value: returns zero Settings, relying on the tier-default
// logic in IsEnabled (Free → on, AI → off) for the next render, without
// overwriting whatever partial value may be in the KV.
//
// Loaded value: passes through Migrate so that legacy pre-C254 rows are
// transparently upgraded on the next read. The migrated value is NOT persisted
// here; it is written on the next normal SaveSmartSettings call.
func LoadSmartSettings() smart.Settings {
	raw := SettingKVGet(smartSettingsKey)
	if raw == "" {
		// Store not yet initialised or fresh first session. Return free-on
		// defaults without any write so we cannot race with store init.
		return smart.EnableFreeOnly(smart.Settings{})
	}
	var s smart.Settings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return smart.Settings{}
	}
	// Transparently upgrade legacy rows (Version==0) to the current schema.
	// Explicit user choices are preserved; only unset Free features are filled
	// in. The migrated value is persisted on the next normal SaveSmartSettings
	// call, not here, so this path stays free of side-effects.
	return smart.Migrate(s)
}

// InitSmartSettings performs the one-time default persist for new installs. It
// writes the EnableFreeOnly defaults (with the current schema Version) only
// when the KV is still empty, making it idempotent — safe to call on every
// boot. It MUST be called after the store is confirmed ready (e.g. from the
// post-store-ready wiring block in app.Run) so that the write lands in a live
// store and is never lost or premature.
func InitSmartSettings() {
	if SettingKVGet(smartSettingsKey) != "" {
		return // already initialised
	}
	s := smart.EnableFreeOnly(smart.Settings{})
	s.Version = smart.CurrentSettingsVersion
	SaveSmartSettings(s)
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

// DismissAllSmartInsights dismisses every given insight key at once (the panel's
// "dismiss all") and persists.
func DismissAllSmartInsights(keys []string) smart.Settings {
	s := LoadSmartSettings().DismissAll(keys)
	SaveSmartSettings(s)
	return s
}

// SnoozeSmartPanel hides the whole Smart strip until the given time and persists.
func SnoozeSmartPanel(until time.Time) smart.Settings {
	s := LoadSmartSettings().SnoozeUntil(until.Unix())
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

// SetSmartQuoteTheme sets the daily-quote theme and INVALIDATES the cached quote
// (clears its stored result + last-run) so the dashboard regenerates a fresh quote
// in the new theme on the next render. Passing the current theme is the "new
// quote" action — it just clears the cache. Persists.
func SetSmartQuoteTheme(theme string) smart.Settings {
	s := LoadSmartSettings()
	s.QuoteTheme = theme
	delete(s.Results, "SMART-QUOTE") // delete on a nil map is a safe no-op
	delete(s.LastRun, "SMART-QUOTE")
	SaveSmartSettings(s)
	return s
}

// SetSmartQuoteContext toggles personalization of the daily quote (whether the
// user's financial snapshot steers the quote choice) and invalidates the cached
// quote so the next render regenerates with the new setting. Persists.
func SetSmartQuoteContext(on bool) smart.Settings {
	s := LoadSmartSettings()
	s.QuoteUseContext = on
	delete(s.Results, "SMART-QUOTE")
	delete(s.LastRun, "SMART-QUOTE")
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

// ClearSmartGenerated wipes the DATA-DERIVED smart state — dismissed insights,
// last-run stamps, and cached AI result "messages" — plus the digest-delivered
// log, while keeping the user's feature opt-ins/schedules/mutes/density. A data
// wipe calls this: the smart PREFERENCES live in the preserved settings KV (they
// survive a wipe like theme/language), but the generated content describes data
// that no longer exists, so it must not survive. Writing the cleared settings
// before the post-wipe dataset export means the reload re-hydrates the cleared
// state, not the stale one.
func ClearSmartGenerated() {
	SaveSmartSettings(LoadSmartSettings().ClearGenerated())
	SettingKVDelete(digestDeliveredKey)
}

// EnableFreeSmart enables all Free-tier features and persists. AI-tier features
// are left at their current state (explicitly-on AI features stay on; others
// stay at the off-by-default tier default).
func EnableFreeSmart() smart.Settings {
	s := smart.EnableFreeOnly(LoadSmartSettings())
	SaveSmartSettings(s)
	return s
}
