// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

const assistantTabAtomID = "assistant:tab"

// UseAssistantTab is the /assistant hub's active-tab atom ("ask", "insights",
// "smart"). An atom (not hub-local state) so surfaces inside a tab can move
// the user to a sibling tab — e.g. the Ask rail's "See all in Insights" link.
func UseAssistantTab() state.Atom[string] {
	return state.UseAtom(assistantTabAtomID, "ask")
}
