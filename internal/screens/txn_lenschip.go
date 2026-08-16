// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// lensChipKey marks the transactions toolbar chip that represents the TOP BAR's
// member perspective rather than one of the page's own filters.
//
// The distinction is the whole point of C574. Every other chip is a value inside
// the persisted filter, so its ✕ removes that value. The perspective is not in the
// filter at all — it is ambient app state layered on at render time — so a ✕ that
// went through the filter's RemoveValue removed nothing and left the chip sitting
// there. Keying it apart lets the one handler route each chip to the state that
// actually owns it.
const lensChipKey = "lens"

// clearAllLabel is the text for the filter summary's reset, or "" to hide it.
//
// The count is the number of the PAGE's own filters, which is not always the
// number of chips on screen: the member-lens chip is cleared from the top bar, so
// with a lens on and no filters set the reset would read "Clear all 0 filters" and
// do nothing when pressed. A control with nothing to do should not be there.
func clearAllLabel(n int) string {
	if n == 0 {
		return ""
	}
	return uistate.T("transactions.clearAllFiltersN", plural(n, "filter"))
}

// customFieldLabel resolves a transaction custom-field key to the label the user
// gave it, falling back to the raw key so a chip never renders as a bare value
// with no field name attached.
func customFieldLabel(defs []customfields.Def, key string) string {
	for _, d := range defs {
		if d.Key == key {
			return d.Label
		}
	}
	return key
}
