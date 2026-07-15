// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"time"

	"github.com/monstercameron/GoWebComponents/v4/state"
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
	RoundUpCadenceWeekly  = "weekly"
	RoundUpCadenceMonthly = "monthly"
)

// RoundUpConfig is the persisted virtual round-up policy (TX11): whether the
// round-up jar is on, the goal the jar sweeps into, the sweep cadence, which
// accounts participate, and the timestamp of the last approved sweep (the
// accrual window starts there). Stored as config so the ritual is data, not code.
type RoundUpConfig struct {
	// Enabled turns the round-up jar + sweep card on.
	Enabled bool `json:"enabled"`
	// TargetGoalID is the goal the swept jar is earmarked toward.
	TargetGoalID string `json:"targetGoalId"`
	// Cadence is RoundUpCadenceWeekly or RoundUpCadenceMonthly.
	Cadence string `json:"cadence"`
	// AccountIDs are the participating accounts; empty means every account.
	AccountIDs []string `json:"accountIds"`
	// LastSweepStamp is the RFC3339 time the jar was last swept, the exclusive
	// lower bound of the next accrual window. Empty means never swept.
	LastSweepStamp string `json:"lastSweepStamp"`
}

// EffectiveCadence returns the cadence, defaulting to weekly.
func (c RoundUpConfig) EffectiveCadence() string {
	if c.Cadence == RoundUpCadenceMonthly {
		return RoundUpCadenceMonthly
	}
	return RoundUpCadenceWeekly
}

// SinceOr returns the accrual window's start: the parsed LastSweepStamp, or the
// supplied fallback when no sweep has been recorded yet.
func (c RoundUpConfig) SinceOr(fallback time.Time) time.Time {
	if c.LastSweepStamp == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, c.LastSweepStamp)
	if err != nil {
		return fallback
	}
	return t
}

// ParticipatingSet returns the account-id set (empty map when all participate).
func (c RoundUpConfig) ParticipatingSet() map[string]bool {
	if len(c.AccountIDs) == 0 {
		return nil
	}
	m := make(map[string]bool, len(c.AccountIDs))
	for _, id := range c.AccountIDs {
		m[id] = true
	}
	return m
}

// RoundUpConfigGet returns the effective round-up config: the persisted override,
// or a disabled default (weekly cadence) when nothing is stored yet.
func RoundUpConfigGet() RoundUpConfig {
	raw := kvGet(roundUpConfigKey)
	if raw == "" {
		return RoundUpConfig{Cadence: RoundUpCadenceWeekly}
	}
	var cfg RoundUpConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
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
