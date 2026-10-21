// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/monstercameron/GoWebComponents/v5/state"
)

// UseRoundUpConfigOpen returns the shared atom controlling whether the "Round-ups"
// config flip modal is open. The goals toolbar button sets it; the goals surface
// renders the modal from its root when it is true.
func UseRoundUpConfigOpen() state.Atom[bool] {
	return state.UseAtom("goals:roundup-config-open", false)
}

const (
	roundUpConfigKey      = "cashflux:goals:roundupconfig"
	roundUpPromptStateKey = "cashflux:goals:rounduprompt"
)

// RoundUpConfigGet returns the effective round-up config: the persisted override,
// or a disabled default (weekly cadence) when nothing is stored yet.
func RoundUpConfigGet() RoundUpConfig {
	raw := kvGet(roundUpConfigKey)
	// Legacy JSON null decoded into the zero config; retain that UI contract.
	if strings.TrimSpace(raw) == "null" {
		return RoundUpConfig{}
	}
	cfg, err := ParseRoundUpConfig(raw)
	if err != nil {
		return RoundUpConfig{Cadence: RoundUpCadenceWeekly}
	}
	return cfg
}

// SetRoundUpConfig persists the round-up config.
func SetRoundUpConfig(cfg RoundUpConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(roundUpConfigKey, string(data))
	}
}

// StampRoundUpSweep records now as the last-sweep time (advancing the accrual
// window) and persists the config. Called on approval.
func StampRoundUpSweep(now time.Time) {
	cfg := RoundUpConfigGet()
	cfg.LastSweepStamp = now.Format(time.RFC3339)
	SetRoundUpConfig(cfg)
}

// roundUpPromptState records which cadence period the round-up card was last
// acted on (swept or dismissed), so the card never re-nags for a period already
// handled.
type roundUpPromptState struct {
	HandledPeriod string `json:"handledPeriod"`
}

// RoundUpPromptHandledPeriod returns the period key the user last swept or
// dismissed, or "" if none.
func RoundUpPromptHandledPeriod() string {
	raw := kvGet(roundUpPromptStateKey)
	if raw == "" {
		return ""
	}
	var st roundUpPromptState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return ""
	}
	return st.HandledPeriod
}

// MarkRoundUpPromptHandled records that the round-up card for the given cadence
// period key has been acted on, so it will not reappear for that period.
func MarkRoundUpPromptHandled(periodKey string) {
	if data, err := json.Marshal(roundUpPromptState{HandledPeriod: periodKey}); err == nil {
		kvSet(roundUpPromptStateKey, string(data))
	}
}
