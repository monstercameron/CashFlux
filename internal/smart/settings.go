// SPDX-License-Identifier: MIT

package smart

// Settings is the user's opt-in state for the SMART series. The series is
// strictly opt-in: every feature is OFF until the user enables it, so nothing in
// here can block, gate, or slow a core flow until the user has asked for it.
//
// Settings holds two independent sets:
//   - Enabled features (by Code). A feature does nothing — no compute, no
//     insights, no AI calls — unless enabled.
//   - Dismissed insight keys. A dismissed insight stays hidden across recomputes
//     and reloads until its underlying condition changes enough to mint a new Key.
//
// Settings is a plain value type so it serializes to the dataset/KV and unit-
// tests trivially. The zero value is valid: nothing enabled, nothing dismissed.
type Settings struct {
	Enabled   map[string]bool `json:"enabled,omitempty"`
	Dismissed map[string]bool `json:"dismissed,omitempty"`
}

// IsEnabled reports whether the feature code is opted in. Unknown codes are off.
func (s Settings) IsEnabled(code string) bool {
	return s.Enabled[code]
}

// SetEnabled turns a feature on or off. It is a no-op for unknown codes so a
// stale UI cannot enable a removed feature. Returns the updated Settings (maps
// are lazily allocated, so callers should use the returned value).
func (s Settings) SetEnabled(code string, on bool) Settings {
	if _, ok := byCode[code]; !ok {
		return s
	}
	if s.Enabled == nil {
		s.Enabled = map[string]bool{}
	}
	if on {
		s.Enabled[code] = true
	} else {
		delete(s.Enabled, code)
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

// EnabledCodes returns the enabled feature codes that still exist in the
// catalog, in catalog order (stable for display), dropping any stale codes.
func (s Settings) EnabledCodes() []string {
	var out []string
	for _, f := range catalog {
		if s.Enabled[f.Code] {
			out = append(out, f.Code)
		}
	}
	return out
}

// EnabledFeaturesForPage returns the catalog features on a page that are enabled,
// in catalog order. Engines use this to skip work for features the user hasn't
// turned on.
func (s Settings) EnabledFeaturesForPage(p Page) []Feature {
	var out []Feature
	for _, f := range catalog {
		if f.Page == p && s.Enabled[f.Code] {
			out = append(out, f)
		}
	}
	return out
}

// AnyAIEnabled reports whether the user has opted into at least one AI feature —
// the signal the UI uses to decide whether to nudge about configuring a provider.
func (s Settings) AnyAIEnabled() bool {
	for code := range s.Enabled {
		if f, ok := byCode[code]; ok && f.Tier == TierAI {
			return true
		}
	}
	return false
}

// Active filters a fresh batch of insights to the ones that should be shown:
// their feature is still enabled and they are not dismissed. It does not mutate
// the input. The result keeps the input order (engines sort before display).
func (s Settings) Active(in []Insight) []Insight {
	out := in[:0:0] // new backing array; never alias the caller's slice
	for _, ins := range in {
		if !s.Enabled[ins.Feature] {
			continue
		}
		if s.Dismissed[ins.Key] {
			continue
		}
		out = append(out, ins)
	}
	return out
}
