// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate provides UI state atoms and KV persistence helpers.
// This file wires the learntally package into the KV store so payee→category
// correction tallies survive page reloads (C33) and a user-tunable threshold
// survives as well (C35).
package uistate

import (
	"encoding/json"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/learntally"
)

const (
	// learnTallyKey is the SettingKV key for the persisted correction tally.
	// Lives in the PRESERVED settings KV so it survives a dataset wipe — the
	// tally is learned behaviour, not transaction data.
	learnTallyKey = "cashflux:learn-tally"

	// learnThresholdKey stores the user's chosen suggestion threshold.
	learnThresholdKey = "cashflux:learn-threshold"
)

// LoadLearnTally deserializes the correction tally from the preserved KV store.
// Returns an empty (non-nil) Tally when the key is absent or unparseable.
func LoadLearnTally() learntally.Tally {
	raw := SettingKVGet(learnTallyKey)
	if raw == "" {
		return learntally.Tally{}
	}
	var t learntally.Tally
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return learntally.Tally{}
	}
	return t
}

// SaveLearnTally serializes the tally and writes it to the preserved KV store.
// Silent on marshal error (no tally to persist then).
func SaveLearnTally(t learntally.Tally) {
	b, err := json.Marshal(t)
	if err != nil {
		return
	}
	SettingKVSet(learnTallyKey, string(b))
}

// IncrementLearnTally loads the tally, records one correction of payee→categoryID,
// and persists. A no-op when either field is empty (after normalization).
func IncrementLearnTally(payee, categoryID string) {
	if learntally.NormalizePayee(payee) == "" || categoryID == "" {
		return
	}
	t := LoadLearnTally()
	t.Increment(payee, categoryID)
	SaveLearnTally(t)
}

// LoadLearnThreshold returns the user's persisted suggestion threshold.
// Falls back to learntally.DefaultMinCount when no override is stored.
func LoadLearnThreshold() int {
	raw := SettingKVGet(learnThresholdKey)
	if raw == "" {
		return learntally.DefaultMinCount
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return learntally.DefaultMinCount
	}
	return n
}

// SaveLearnThreshold persists the user's chosen suggestion threshold.
// Values below 1 are clamped to 1.
func SaveLearnThreshold(n int) {
	if n < 1 {
		n = 1
	}
	SettingKVSet(learnThresholdKey, strconv.Itoa(n))
}
