// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "github.com/monstercameron/CashFlux/internal/uistate"

func budgetAttentionLabel(count int) string {
	return uistate.TN("budgets.attention.one", "budgets.attention.many", count)
}
