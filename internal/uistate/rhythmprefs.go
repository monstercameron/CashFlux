// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

// This file holds the durable preferences for the unified Bills & Recurring
// ("month's rhythm") surface: the agenda view choice and the discovery pins.
// Both live in the PRESERVED settings KV (the same bucket as theme/language),
// so they survive a dataset wipe; SettingKVSet already flushes a durable persist
// (RequestPersist) on write.

// agendaViewKey is the settings KV key for the up-next agenda's view choice.
const agendaViewKey = "cashflux:rhythm-agenda-view"

// AgendaView is one of the two first-class agenda views.
const (
	AgendaViewCompact  = "compact"
	AgendaViewCalendar = "calendar"
)

// AgendaViewGet returns the persisted agenda view, defaulting to the dense
// compact list.
func AgendaViewGet() string {
	if v := SettingKVGet(agendaViewKey); v == AgendaViewCalendar {
		return AgendaViewCalendar
	}
	return AgendaViewCompact
}

// AgendaViewSet persists the agenda view choice (durably, via SettingKVSet).
func AgendaViewSet(v string) {
	if v != AgendaViewCalendar {
		v = AgendaViewCompact
	}
	SettingKVSet(agendaViewKey, v)
}

// recurPinsKey is the settings KV key for the discovery clustering pins.
const recurPinsKey = "cashflux:rhythm-detect-pins"

// RecurPins are the user's discovery overrides, persisted alongside the
// SubsDetectPrefs. Suppressed canonical signatures are the "not recurring"
// rejects; the pair lists are the "not the same" / "merge these" overrides
// surfaced in Detection preferences. Kept as plain slices so the JSON round-
// trips through the settings KV; the screen converts them to recurdiscover.Pins.
type RecurPins struct {
	Suppressed []string    `json:"suppressed,omitempty"`
	NeverMerge [][2]string `json:"neverMerge,omitempty"`
	ForceMerge [][2]string `json:"forceMerge,omitempty"`
}

// LoadRecurPins reads the persisted discovery pins (empty when unset).
func LoadRecurPins() RecurPins {
	raw := SettingKVGet(recurPinsKey)
	if raw == "" {
		return RecurPins{}
	}
	var p RecurPins
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return RecurPins{}
	}
	return p
}

// SaveRecurPins persists the discovery pins durably.
func SaveRecurPins(p RecurPins) {
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	SettingKVSet(recurPinsKey, string(b))
}

// SuppressSignature adds a canonical signature to the suppressed ("not
// recurring") set and persists it; a duplicate is ignored.
func SuppressSignature(sig string) {
	if sig == "" {
		return
	}
	p := LoadRecurPins()
	for _, s := range p.Suppressed {
		if s == sig {
			return
		}
	}
	p.Suppressed = append(p.Suppressed, sig)
	SaveRecurPins(p)
}
