// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/amountmath"
)

// EvalAmountField is the shared blur/Enter handler behaviour for amount inputs
// (TX16): if raw is an arithmetic expression ("45.99*3", "(12+8)*2") it returns
// the computed result formatted to 2 decimals with ok=true; a plain number or
// any parse/eval failure returns ok=false so the caller leaves the field's text
// exactly as the user typed it (no error nag). Wired at every amount-input call
// site — quick-add and the split editor at minimum — so math "just works".
func EvalAmountField(raw string) (string, bool) {
	v, ok := amountmath.EvalAmount(raw)
	if !ok {
		return "", false
	}
	return strconv.FormatFloat(v, 'f', 2, 64), true
}
