// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

// preflightCycleKey stores the pay-cycle boundary (a "2006-01-02" payday string)
// the payday pre-flight card was last dismissed for, so the ritual regenerates
// once per cycle instead of nagging on every visit (XC9).
const preflightCycleKey = "cashflux:preflight:dismissedCycle"

// PreflightDismissedCycle returns the cycle key the pre-flight card was last
// dismissed for (empty if never).
func PreflightDismissedCycle() string { return kvGet(preflightCycleKey) }

// DismissPreflightCycle records that the user dismissed the pre-flight card for
// the given pay-cycle boundary, so it stays hidden until the next cycle.
func DismissPreflightCycle(cycle string) {
	kvSet(preflightCycleKey, cycle)
	BumpDataRevision()
}
