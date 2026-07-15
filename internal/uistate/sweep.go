// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/GoWebComponents/v4/state"
)

// UseSweepConfigOpen returns the shared atom controlling whether the "Sweep
// leftovers" config flip modal is open. The budgets toolbar button sets it; the
// budgets surface renders the modal from its root (outside any tile transform)
// when it is true.
func UseSweepConfigOpen() state.Atom[bool] {
	return state.UseAtom("budgets:sweep-config-open", false)
}

const (
	sweepConfigKey      = "cashflux:budgets:sweepconfig"
	sweepPromptStateKey = "cashflux:budgets:sweepprompt"
)

// SweepConfig is the persisted leftover-sweep policy (XC6): whether the month-
// close sweep ritual is on, which budgets contribute their unspent remainder, and
// the goal the swept total earmarks toward. Stored as config so the ritual is
// data, not code.
type SweepConfig struct {
	// Enabled turns the month-close sweep card + ritual on.
	Enabled bool `json:"enabled"`
	// BudgetIDs are the budgets whose leftovers participate.
	BudgetIDs []string `json:"budgetIds"`
	// TargetGoalID is the goal the swept total is earmarked toward.
	TargetGoalID string `json:"targetGoalId"`
}

// Domain returns the config as the pure-logic budgeting.SweepConfig used by
// ComputeSweep, so the UI and the tested logic share one shape.
func (c SweepConfig) Domain() budgeting.SweepConfig {
	return budgeting.SweepConfig{
		Enabled:      c.Enabled,
		BudgetIDs:    c.BudgetIDs,
		TargetGoalID: c.TargetGoalID,
	}
}

// SweepConfigGet returns the effective sweep config: the persisted override, or a
// disabled default when nothing is stored yet.
func SweepConfigGet() SweepConfig {
	raw := kvGet(sweepConfigKey)
	if raw == "" {
		return SweepConfig{}
	}
	var cfg SweepConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return SweepConfig{}
	}
	return cfg
}

// SetSweepConfig persists the sweep config.
func SetSweepConfig(cfg SweepConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(sweepConfigKey, string(data))
	}
}

// sweepPromptState records which month the sweep card was last acted on (swept or
// dismissed), keyed as "YYYY-MM", so the card never re-nags for a month the user
// has already handled.
type sweepPromptState struct {
	// HandledMonth is the "YYYY-MM" of the closed month the user last swept or
	// dismissed. Empty means never handled.
	HandledMonth string `json:"handledMonth"`
}

// SweepPromptHandledMonth returns the closed month ("YYYY-MM") the user last swept
// or dismissed, or "" if none.
func SweepPromptHandledMonth() string {
	raw := kvGet(sweepPromptStateKey)
	if raw == "" {
		return ""
	}
	var st sweepPromptState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return ""
	}
	return st.HandledMonth
}

// MarkSweepPromptHandled records that the sweep card for the given closed month
// ("YYYY-MM") has been acted on, so it will not appear again for that month.
func MarkSweepPromptHandled(month string) {
	if data, err := json.Marshal(sweepPromptState{HandledMonth: month}); err == nil {
		kvSet(sweepPromptStateKey, string(data))
	}
}

// SweepPromptDue reports whether the sweep card should be shown for the given
// closed month: the config is enabled and that month has not already been swept or
// dismissed.
func SweepPromptDue(closedMonth string, cfg SweepConfig) bool {
	if !cfg.Enabled || closedMonth == "" {
		return false
	}
	return SweepPromptHandledMonth() != closedMonth
}
