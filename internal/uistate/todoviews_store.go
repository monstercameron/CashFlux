// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/todoview"

// todoViewsKey is the SettingKV key for the household's saved to-do views (C404).
//
// It lives in the PRESERVED settings KV, alongside the rebalance targets and the
// learned-correction tally, for the same reason: a saved view is a statement
// about how someone works, not transaction data, and losing it on a dataset wipe
// would be a small betrayal of effort the user put in by hand.
const todoViewsKey = "cashflux:todo-views"

// LoadTodoViews reads the saved to-do views. A missing or malformed value yields
// an empty store — see todoview.Unmarshal for why that is not an error.
func LoadTodoViews() todoview.Store {
	return todoview.Unmarshal(SettingKVGet(todoViewsKey))
}

// SaveTodoViews persists the store, clearing the key entirely when it is empty
// so a household that deletes its last view leaves no leftover blob behind.
func SaveTodoViews(s todoview.Store) {
	if len(s.Views) == 0 {
		SettingKVSet(todoViewsKey, "")
		return
	}
	SettingKVSet(todoViewsKey, s.Marshal())
}
