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
	// SuppressedNames maps a suppressed signature to the display name the user
	// actually rejected ("Xbox Game Pass" rather than "MSFT XBOX GAME PASS #").
	// Additive and optional: pins written before this existed simply fall back to
	// showing the signature, which is the descriptor the bank prints anyway.
	//
	// It exists because a rejection has to be undoable, and an undo list the user
	// cannot read is not an undo list.
	SuppressedNames map[string]string `json:"suppressedNames,omitempty"`
}

// SuppressedEntry is one rejected signature paired with a readable name, for the
// un-suppress list in Detection preferences.
type SuppressedEntry struct {
	Signature string
	Name      string
}

// SuppressedList returns the rejected signatures with their display names, in the
// order they were rejected. A signature with no recorded name shows as itself.
func (p RecurPins) SuppressedList() []SuppressedEntry {
	out := make([]SuppressedEntry, 0, len(p.Suppressed))
	for _, s := range p.Suppressed {
		name := p.SuppressedNames[s]
		if name == "" {
			name = s
		}
		out = append(out, SuppressedEntry{Signature: s, Name: name})
	}
	return out
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
// recurring") set and persists it, recording name so the rejection can be shown
// back to the user in a form they recognise. A duplicate refreshes the name only.
func SuppressSignature(sig, name string) {
	if sig == "" {
		return
	}
	p := LoadRecurPins()
	if p.SuppressedNames == nil {
		p.SuppressedNames = map[string]string{}
	}
	if name != "" {
		p.SuppressedNames[sig] = name
	}
	for _, s := range p.Suppressed {
		if s == sig {
			SaveRecurPins(p)
			return
		}
	}
	p.Suppressed = append(p.Suppressed, sig)
	SaveRecurPins(p)
}

// UnsuppressSignature removes a signature from the suppressed set, so discovery
// may propose it again. It is the way back out of "Not recurring": a one-click
// reject that cannot be undone can permanently hide a real commitment, and the
// user would have no route back and no way to know something was missing.
func UnsuppressSignature(sig string) {
	p := LoadRecurPins()
	kept := make([]string, 0, len(p.Suppressed))
	for _, s := range p.Suppressed {
		if s != sig {
			kept = append(kept, s)
		}
	}
	p.Suppressed = kept
	delete(p.SuppressedNames, sig)
	SaveRecurPins(p)
}
