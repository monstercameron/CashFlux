// SPDX-License-Identifier: MIT

package smart

// Migrate upgrades a Settings value loaded from the KV store to the current
// schema, preserving all explicit user choices. It is a pure function: it does
// not persist anything — the migrated value is written on the next normal
// SaveSmartSettings call so the read path stays free of side-effects.
//
// Migration policy (conservative):
//
//   - If Version != 0 the row is already at the current schema; return unchanged.
//   - If Version == 0 (legacy pre-C254 row): for each Free-tier feature that has
//     NO explicit state in the row (neither in Enabled nor in ExplicitOff), add it
//     to Enabled so that the free-on default is recorded explicitly rather than
//     being inferred from the tier each time. This fills in the C254 "free features
//     on by default" intent without overriding anything the user already chose.
//   - A Free feature already in ExplicitOff (user said "off") is left alone.
//   - A Free feature already in Enabled (user said "on") is left alone.
//   - AI features are never touched: they stay opt-in regardless of schema version.
//   - Version is bumped to CurrentSettingsVersion when migration runs.
func Migrate(s Settings) Settings {
	if s.Version != 0 {
		return s
	}
	// Lazily allocate the Enabled map — only create it if we actually need to
	// write into it (some legacy rows may already have all features explicit).
	for _, f := range catalog {
		if f.Tier != TierFree {
			continue
		}
		// Skip if the user already recorded an explicit state either way.
		if s.Enabled[f.Code] || s.ExplicitOff[f.Code] {
			continue
		}
		// No explicit state for this Free feature — fill in the free-on default.
		if s.Enabled == nil {
			s.Enabled = map[string]bool{}
		}
		s.Enabled[f.Code] = true
	}
	s.Version = CurrentSettingsVersion
	return s
}
