// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

const insightsPeriodAtomID = "assistant:insightsPeriod"

// UseInsightsPeriod is the Insights briefing's chosen time range (G2-C8). An atom
// rather than panel-local state so the choice survives a trip to a sibling tab and
// back — a selector that silently resets is one people stop trusting they set.
//
// The value is an insightsperiod.Period; it is held as a string here so uistate
// keeps no dependency on the package that interprets it, and an unrecognised value
// resolves to the default rather than erroring.
func UseInsightsPeriod() state.Atom[string] {
	return state.UseAtom(insightsPeriodAtomID, "this-month")
}
