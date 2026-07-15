// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/GoWebComponents/v4/state"
)

// UseFlexSheetOpen returns the shared atom that controls the flex-budgeting
// category-assignment sheet (BG2): the one-time modal where each category is
// classified as flex, fixed, or non-monthly. Open it from the flex-view tile.
func UseFlexSheetOpen() state.Atom[bool] { return state.UseAtom("budgets:flexSheet", false) }
