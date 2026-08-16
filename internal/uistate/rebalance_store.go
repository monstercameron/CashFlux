// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/portfolio"
)

// rebalanceTargetsKey is the SettingKV key for the household's target asset
// allocation (C379).
//
// It lives in the PRESERVED settings KV rather than as a dataset field: a target
// allocation is a statement of intent, not transaction data, and it should
// survive a wipe for the same reason the learned-correction tally does. Keeping
// it here also means no store migration for what is a small JSON blob.
const rebalanceTargetsKey = "cashflux:rebalance-targets"

// LoadRebalanceTargets reads the household's target allocation. Returns nil when
// nothing has been set, which every caller treats as "no targets" rather than as
// "target everything at zero".
func LoadRebalanceTargets() []portfolio.Target {
	raw := SettingKVGet(rebalanceTargetsKey)
	if raw == "" {
		return nil
	}
	var ts []portfolio.Target
	if err := json.Unmarshal([]byte(raw), &ts); err != nil {
		return nil
	}
	return ts
}

// SaveRebalanceTargets persists a target allocation, or clears it when the set
// is empty. It does NOT validate: the caller checks portfolio.TargetsValid and
// decides what to tell the user, so an invalid set is never silently stored and
// never silently discarded either.
func SaveRebalanceTargets(ts []portfolio.Target) {
	if len(ts) == 0 {
		SettingKVSet(rebalanceTargetsKey, "")
		return
	}
	b, err := json.Marshal(ts)
	if err != nil {
		return
	}
	SettingKVSet(rebalanceTargetsKey, string(b))
}
