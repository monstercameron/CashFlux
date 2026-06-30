// SPDX-License-Identifier: MIT

package smart

import "time"

// CurrentSettingsVersion is the current schema version for Settings. A stored
// row with Version==0 is a legacy pre-C254 record and will be upgraded by
// Migrate the next time it is loaded.
const CurrentSettingsVersion = 1

// Settings is the user's preference state for the SMART series. Free
// (deterministic, on-device) features are enabled by default so users get value
// immediately; AI features stay opt-in so no spend happens without consent.
//
// An explicit user choice — toggling a feature on or off — always wins and is
// preserved across reloads. The default only applies when the user has never
// touched a feature's toggle.
//
// Settings holds two independent sets:
//   - Enabled tracks features the user has explicitly turned ON (any tier).
//   - ExplicitOff tracks features the user has explicitly turned OFF. The
//     combination of the two is what distinguishes "never touched" (tier default
//     applies) from "user said no" (honor it).
//   - Dismissed insight keys: a dismissed insight stays hidden across recomputes
//     and reloads until its underlying condition changes enough to mint a new Key.
//
// Settings is a plain value type so it serializes to the dataset/KV and unit-
// tests trivially. The zero value is valid: Free features on by tier default, AI
// features off by tier default, nothing dismissed.
type Settings struct {
	// Version is a schema marker used for stale-state migration. Zero means a
	// legacy pre-C254 row; CurrentSettingsVersion means fully migrated.
	// json:"version,omitempty" omits the field when zero so existing zero-version
	// rows round-trip without spurious JSON changes.
	Version     int             `json:"version,omitempty"`
	Enabled     map[string]bool `json:"enabled,omitempty"`
	ExplicitOff map[string]bool `json:"explicitOff,omitempty"`
	Dismissed   map[string]bool `json:"dismissed,omitempty"`

	// Schedules is the per-feature run cadence (when it runs). A missing entry
	// uses the tier default (Free→Live, AI→Manual). See schedule.go.
	Schedules map[string]Cadence `json:"schedules,omitempty"`
	// Muted features are snoozed: enabled but not surfaced (a per-feature "not
	// now" that's reversible without losing the opt-in or its schedule).
	Muted map[string]bool `json:"muted,omitempty"`
	// LastRun records the unix-second timestamp of each feature's last run, so
	// cadence Due/freshness checks and AI result caching work across reloads.
	LastRun map[string]int64 `json:"lastRun,omitempty"`
	// Results caches each AI feature's last produced text, shown between runs so a
	// scheduled/manual AI result persists without re-spending on every render.
	Results map[string]string `json:"results,omitempty"`
	// Density is the global "how much smart weaves into the app" dial (see
	// density.go). The empty value means the Standard default.
	Density Density `json:"density,omitempty"`
	// QuoteTheme is the stylistic theme for the SMART-QUOTE daily quote (e.g.
	// "Stoic", "Playful"). Empty means the default theme. It is a preference, so
	// ClearGenerated keeps it.
	QuoteTheme string `json:"quoteTheme,omitempty"`
	// QuoteUseContext opts the daily quote into personalization: when true, a
	// snapshot of the user's financial situation (goals, flows) is sent so the
	// model picks a quote relevant to it. Off by default — no figures leave the
	// device for the quote unless the user turns this on. A preference (kept by
	// ClearGenerated).
	QuoteUseContext bool `json:"quoteUseContext,omitempty"`
}

// DefaultQuoteTheme is the fallback theme for the daily quote when the user
// hasn't picked one.
const DefaultQuoteTheme = "Stoic"

// QuoteThemeOr returns the chosen quote theme, or DefaultQuoteTheme when unset.
func (s Settings) QuoteThemeOr() string {
	if s.QuoteTheme == "" {
		return DefaultQuoteTheme
	}
	return s.QuoteTheme
}

// ClearGenerated returns a copy of s with all DATA-DERIVED smart state removed —
// dismissed insight keys, per-feature last-run timestamps, and cached AI result
// text (the "smart messages") — while KEEPING the user's preferences: which
// features are on/off (Enabled/ExplicitOff), their schedules, mutes, and the
// density dial. A data wipe uses this: cached messages and dismissals describe
// transactions/accounts that no longer exist, so they must not survive a wipe,
// but the user's opt-ins should. s is a value; the original is not mutated.
func (s Settings) ClearGenerated() Settings {
	s.Dismissed = nil
	s.LastRun = nil
	s.Results = nil
	return s
}

// IsEnabled reports whether a feature is effectively on, applying the tier-based
// default for features the user has never explicitly toggled:
//
//   - TierFree  → on by default (deterministic, no cost, no network).
//   - TierAI    → off by default (requires a configured provider and spends money).
//
// An explicit user choice recorded via SetEnabled always wins over the default:
// a Free feature turned off stays off; an AI feature turned on stays on.
// Unknown codes always return false.
func (s Settings) IsEnabled(code string) bool {
	f, ok := byCode[code]
	if !ok {
		return false
	}
	if s.Enabled[code] {
		return true
	}
	if s.ExplicitOff[code] {
		return false
	}
	// No explicit choice: apply the tier default.
	return f.Tier == TierFree
}

// SetEnabled records an explicit user choice to turn a feature on or off. It is
// a no-op for unknown codes so a stale UI cannot enable a removed feature.
// Returns the updated Settings (maps are lazily allocated, so callers must use
// the returned value).
//
// Turning a feature on clears any prior explicit-off record and sets it in
// Enabled. Turning a feature off clears Enabled and records an explicit-off so
// that IsEnabled knows "user said no" rather than "never asked".
func (s Settings) SetEnabled(code string, on bool) Settings {
	if _, ok := byCode[code]; !ok {
		return s
	}
	if on {
		if s.Enabled == nil {
			s.Enabled = map[string]bool{}
		}
		s.Enabled[code] = true
		delete(s.ExplicitOff, code)
	} else {
		delete(s.Enabled, code)
		if s.ExplicitOff == nil {
			s.ExplicitOff = map[string]bool{}
		}
		s.ExplicitOff[code] = true
	}
	return s
}

// Dismiss hides the insight with the given Key.
func (s Settings) Dismiss(key string) Settings {
	if key == "" {
		return s
	}
	if s.Dismissed == nil {
		s.Dismissed = map[string]bool{}
	}
	s.Dismissed[key] = true
	return s
}

// Restore un-dismisses an insight key (the "show dismissed again" affordance).
func (s Settings) Restore(key string) Settings {
	delete(s.Dismissed, key)
	return s
}

// IsDismissed reports whether the insight key has been dismissed.
func (s Settings) IsDismissed(key string) bool {
	return s.Dismissed[key]
}

// EnabledCodes returns the effectively-enabled feature codes that still exist in
// the catalog, in catalog order (stable for display), dropping any stale codes.
func (s Settings) EnabledCodes() []string {
	var out []string
	for _, f := range catalog {
		if s.IsEnabled(f.Code) {
			out = append(out, f.Code)
		}
	}
	return out
}

// EnabledFeaturesForPage returns the catalog features on a page that are
// effectively enabled, in catalog order. Engines use this to skip work for
// features that are off.
func (s Settings) EnabledFeaturesForPage(p Page) []Feature {
	var out []Feature
	for _, f := range catalog {
		if f.Page == p && s.IsEnabled(f.Code) {
			out = append(out, f)
		}
	}
	return out
}

// AnyAIEnabled reports whether the user has opted into at least one AI feature —
// the signal the UI uses to decide whether to nudge about configuring a provider.
// AI features are off by default, so this is true only when the user has
// explicitly enabled at least one.
func (s Settings) AnyAIEnabled() bool {
	for code := range s.Enabled {
		if f, ok := byCode[code]; ok && f.Tier == TierAI {
			return true
		}
	}
	return false
}

// Active filters a fresh batch of insights to the ones that should be shown:
// their feature is effectively enabled, not muted, and the insight is not
// dismissed. It does not mutate the input. The result keeps the input order
// (engines sort before display).
func (s Settings) Active(in []Insight) []Insight {
	out := in[:0:0] // new backing array; never alias the caller's slice
	for _, ins := range in {
		if !s.IsEnabled(ins.Feature) || s.Muted[ins.Feature] {
			continue
		}
		if s.Dismissed[ins.Key] {
			continue
		}
		out = append(out, ins)
	}
	return out
}

// --- scheduling, mute, run-tracking, and AI result cache ------------------

// CadenceFor returns the feature's run cadence: the user's choice, or the tier
// default (Free→Live, AI→Manual) when unset or unknown.
func (s Settings) CadenceFor(code string) Cadence {
	if c, ok := s.Schedules[code]; ok && c.Valid() {
		return c
	}
	if f, ok := byCode[code]; ok {
		return DefaultCadence(f.Tier)
	}
	return CadenceLive
}

// SetCadence sets a feature's run cadence. Setting it to the tier default clears
// the override so the stored map stays small. No-op for unknown codes/cadences.
func (s Settings) SetCadence(code string, c Cadence) Settings {
	f, ok := byCode[code]
	if !ok || !c.Valid() {
		return s
	}
	if c == DefaultCadence(f.Tier) {
		delete(s.Schedules, code)
		return s
	}
	if s.Schedules == nil {
		s.Schedules = map[string]Cadence{}
	}
	s.Schedules[code] = c
	return s
}

// IsMuted reports whether the feature is snoozed.
func (s Settings) IsMuted(code string) bool { return s.Muted[code] }

// SetMuted snoozes or un-snoozes a feature (without changing its opt-in/schedule).
func (s Settings) SetMuted(code string, on bool) Settings {
	if _, ok := byCode[code]; !ok {
		return s
	}
	if !on {
		delete(s.Muted, code)
		return s
	}
	if s.Muted == nil {
		s.Muted = map[string]bool{}
	}
	s.Muted[code] = true
	return s
}

// LastRunAt returns when the feature last ran (zero if never).
func (s Settings) LastRunAt(code string) time.Time {
	if ts, ok := s.LastRun[code]; ok && ts > 0 {
		return time.Unix(ts, 0)
	}
	return time.Time{}
}

// MarkRun stamps a feature's last-run time (used by the cadence Due/freshness
// checks and to gate the next scheduled AI call).
func (s Settings) MarkRun(code string, now time.Time) Settings {
	if _, ok := byCode[code]; !ok {
		return s
	}
	if s.LastRun == nil {
		s.LastRun = map[string]int64{}
	}
	s.LastRun[code] = now.Unix()
	return s
}

// ResultFor returns the cached AI result text for a feature (empty if none).
func (s Settings) ResultFor(code string) string { return s.Results[code] }

// SetResult caches an AI feature's produced text so a scheduled/manual result
// persists between renders without re-spending.
func (s Settings) SetResult(code, text string) Settings {
	if _, ok := byCode[code]; !ok {
		return s
	}
	if s.Results == nil {
		s.Results = map[string]string{}
	}
	s.Results[code] = text
	return s
}

// ActiveCodes returns the effectively-enabled, non-muted feature codes in catalog
// order — the set the engine actually runs (an off OR muted feature costs nothing).
func (s Settings) ActiveCodes() []string {
	var out []string
	for _, f := range catalog {
		if s.IsEnabled(f.Code) && !s.Muted[f.Code] {
			out = append(out, f.Code)
		}
	}
	return out
}

// --- global density + bulk enable/disable ---------------------------------

// DensityOrDefault returns the configured density, defaulting to Standard when
// unset or invalid. Density gates which KINDS of affordance show (see density.go);
// callers AND it with the per-feature enabled/mute check.
func (s Settings) DensityOrDefault() Density {
	if s.Density.Valid() {
		return s.Density
	}
	return DensityStandard
}

// SetDensity sets the global density dial. An invalid value is ignored.
func (s Settings) SetDensity(d Density) Settings {
	if d.Valid() {
		s.Density = d
	}
	return s
}

// ShowsAffordance reports whether a feature's affordance of the given kind should
// render right now: the feature is effectively enabled and not muted, AND the
// global density permits that affordance kind. This is the single gate every
// inline smart surface checks.
func (s Settings) ShowsAffordance(code string, a Affordance) bool {
	if !s.IsEnabled(code) || s.Muted[code] {
		return false
	}
	return s.DensityOrDefault().Shows(a)
}

// EnableAll opts into every catalog feature at once (the hub "Enable all"). It
// clears all explicit-off records and marks every feature in Enabled, so all
// features are on regardless of tier. Schedules/mutes/dismissals are untouched.
func (s Settings) EnableAll() Settings {
	s.ExplicitOff = nil
	if s.Enabled == nil {
		s.Enabled = map[string]bool{}
	}
	for _, f := range catalog {
		s.Enabled[f.Code] = true
	}
	return s
}

// DisableAll opts out of every feature at once (the hub "Disable all"). It
// clears Enabled and records every feature as explicitly off, so the Free-tier
// default does not re-enable them. Schedules/mutes are kept so re-enabling
// restores the prior intent.
func (s Settings) DisableAll() Settings {
	s.Enabled = nil
	if s.ExplicitOff == nil {
		s.ExplicitOff = map[string]bool{}
	}
	for _, f := range catalog {
		s.ExplicitOff[f.Code] = true
	}
	return s
}

// EnabledCount returns how many catalog features are currently effectively
// enabled — used by the hub to label/disable the bulk buttons.
func (s Settings) EnabledCount() int {
	n := 0
	for _, f := range catalog {
		if s.IsEnabled(f.Code) {
			n++
		}
	}
	return n
}
