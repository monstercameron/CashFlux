// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

// subsDetectPrefsKey is the settings KV key for subscription detection preferences.
// Stored in the PRESERVED settings KV so it survives dataset wipes (the same
// bucket as theme, language, smart settings, etc.).
const subsDetectPrefsKey = "cashflux:subs-detect-prefs"

// SubsDetectPrefs controls which transactions the subscription detector considers.
// A transaction whose spending category is in IgnoredCategoryIDs, or whose source
// account type is in IgnoredAccountTypes, is excluded before detection runs —
// letting the user tune out noise (e.g. ignore "Dining" so restaurant charges
// never appear as subscriptions, or ignore "investment" accounts).
type SubsDetectPrefs struct {
	// IgnoredCategoryIDs is the set of category IDs whose transactions are
	// excluded from subscription detection. Keyed by ID so renames don't break it.
	IgnoredCategoryIDs []string `json:"ignoredCategoryIds,omitempty"`
	// IgnoredAccountTypes is the set of account-type strings (e.g. "checking",
	// "investment") whose transactions are excluded from detection.
	IgnoredAccountTypes []string `json:"ignoredAccountTypes,omitempty"`
	// MinOccurrences is the detection sensitivity: the minimum number of times a
	// charge must repeat before it counts as a subscription. 0 = unset (use the
	// default of 2). Higher = fewer, more-confident matches; lower = catch newer
	// or less-frequent charges. (C166)
	MinOccurrences int `json:"minOccurrences,omitempty"`
}

// defaultSubsMinOccurrences is the detection threshold used when the user hasn't
// chosen one — two sightings is the lightest evidence of a recurring charge.
const defaultSubsMinOccurrences = 2

// MinOccurrencesOrDefault returns the configured detection threshold, or the
// default (2) when unset, clamped to a sane 2..6 range.
func (p SubsDetectPrefs) MinOccurrencesOrDefault() int {
	n := p.MinOccurrences
	if n <= 0 {
		return defaultSubsMinOccurrences
	}
	if n > 6 {
		return 6
	}
	return n
}

// WithMinOccurrences returns a copy of p with the detection threshold set.
func (p SubsDetectPrefs) WithMinOccurrences(n int) SubsDetectPrefs {
	out := SubsDetectPrefs{
		IgnoredCategoryIDs:  append([]string(nil), p.IgnoredCategoryIDs...),
		IgnoredAccountTypes: append([]string(nil), p.IgnoredAccountTypes...),
		MinOccurrences:      n,
	}
	return out
}

// HasIgnoredCategory reports whether id appears in the ignored-category list.
func (p SubsDetectPrefs) HasIgnoredCategory(id string) bool {
	for _, v := range p.IgnoredCategoryIDs {
		if v == id {
			return true
		}
	}
	return false
}

// HasIgnoredAccountType reports whether typ appears in the ignored-account-type list.
func (p SubsDetectPrefs) HasIgnoredAccountType(typ string) bool {
	for _, v := range p.IgnoredAccountTypes {
		if v == typ {
			return true
		}
	}
	return false
}

// WithCategoryToggled returns a copy of p with id added to (or removed from)
// IgnoredCategoryIDs, depending on whether it is currently present.
func (p SubsDetectPrefs) WithCategoryToggled(id string) SubsDetectPrefs {
	out := SubsDetectPrefs{
		IgnoredCategoryIDs:  make([]string, 0, len(p.IgnoredCategoryIDs)),
		IgnoredAccountTypes: append([]string(nil), p.IgnoredAccountTypes...),
		MinOccurrences:      p.MinOccurrences,
	}
	found := false
	for _, v := range p.IgnoredCategoryIDs {
		if v == id {
			found = true
			continue // drop it (toggle off)
		}
		out.IgnoredCategoryIDs = append(out.IgnoredCategoryIDs, v)
	}
	if !found {
		out.IgnoredCategoryIDs = append(out.IgnoredCategoryIDs, id) // toggle on
	}
	return out
}

// WithAccountTypeToggled returns a copy of p with typ added to (or removed from)
// IgnoredAccountTypes, depending on whether it is currently present.
func (p SubsDetectPrefs) WithAccountTypeToggled(typ string) SubsDetectPrefs {
	out := SubsDetectPrefs{
		IgnoredCategoryIDs:  append([]string(nil), p.IgnoredCategoryIDs...),
		IgnoredAccountTypes: make([]string, 0, len(p.IgnoredAccountTypes)),
		MinOccurrences:      p.MinOccurrences,
	}
	found := false
	for _, v := range p.IgnoredAccountTypes {
		if v == typ {
			found = true
			continue // drop it (toggle off)
		}
		out.IgnoredAccountTypes = append(out.IgnoredAccountTypes, v)
	}
	if !found {
		out.IgnoredAccountTypes = append(out.IgnoredAccountTypes, typ) // toggle on
	}
	return out
}

// LoadSubsDetectPrefs reads the persisted detection preferences. Returns a
// zero SubsDetectPrefs (no ignored categories/types) when not yet set.
func LoadSubsDetectPrefs() SubsDetectPrefs {
	raw := SettingKVGet(subsDetectPrefsKey)
	if raw == "" {
		return SubsDetectPrefs{}
	}
	var p SubsDetectPrefs
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
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
