// SPDX-License-Identifier: MIT

package smart

import "strings"

// Migrate upgrades a Settings value loaded from the KV store to the current
// schema, preserving all explicit user choices. It is a pure function: it does
// not persist anything — the migrated value is written on the next normal
// SaveSmartSettings call so the read path stays free of side-effects.
//
// Migration policy (conservative):
//
//   - Version 0 receives the legacy Free-feature default migration; versions
//     below the current schema also receive feature-identity migrations.
//   - For each Free-tier feature that has
//     NO explicit state in the row (neither in Enabled nor in ExplicitOff), add it
//     to Enabled so that the free-on default is recorded explicitly rather than
//     being inferred from the tier each time. This fills in the C254 "free features
//     on by default" intent without overriding anything the user already chose.
//   - A Free feature already in ExplicitOff (user said "off") is left alone.
//   - A Free feature already in Enabled (user said "on") is left alone.
//   - AI features are never touched: they stay opt-in regardless of schema version.
//   - Version is bumped to CurrentSettingsVersion when migration runs.
func Migrate(s Settings) Settings {
	if s.Version >= CurrentSettingsVersion {
		return s
	}
	if s.Version < 2 {
		migrateT6ToUnusual(&s)
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

// migrateT6ToUnusual carries the former spending-spike preference to the new
// Unusual family. Dismissals are deliberately dropped because the evidence and
// signal identity have changed.
func migrateT6ToUnusual(s *Settings) {
	const oldCode, newCode = "SMART-T6", "SMART-U1"
	if s.Enabled[oldCode] {
		if s.Enabled == nil {
			s.Enabled = map[string]bool{}
		}
		s.Enabled[newCode] = true
		delete(s.ExplicitOff, newCode)
	} else if s.ExplicitOff[oldCode] {
		delete(s.Enabled, newCode)
		if s.ExplicitOff == nil {
			s.ExplicitOff = map[string]bool{}
		}
		s.ExplicitOff[newCode] = true
	} else if s.Version == 0 {
		// Legacy zero-version rows store only explicit choices. The old Free
		// feature's implicit default was on, and remains on for Unusual.
		if s.Enabled == nil {
			s.Enabled = map[string]bool{}
		}
		s.Enabled[newCode] = true
	}
	if s.Muted[oldCode] {
		if s.Muted == nil {
			s.Muted = map[string]bool{}
		}
		s.Muted[newCode] = true
	}
	delete(s.Enabled, oldCode)
	delete(s.ExplicitOff, oldCode)
	delete(s.Muted, oldCode)
	delete(s.Schedules, oldCode)
	delete(s.LastRun, oldCode)
	delete(s.Results, oldCode)
	for key := range s.Dismissed {
		if strings.HasPrefix(key, oldCode+":") {
			delete(s.Dismissed, key)
		}
	}
}
