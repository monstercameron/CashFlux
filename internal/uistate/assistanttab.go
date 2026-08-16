// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

const assistantTabAtomID = "assistant:tab"

// Assistant tab values. Named constants because three routes and the hub itself
// all have to agree on them, and a typo in a string literal fails silently by
// falling through to the default tab.
const (
	// AssistantTabAsk is the conversation.
	AssistantTabAsk = "ask"
	// AssistantTabInsights is the generated analysis — what the app noticed
	// without being asked.
	AssistantTabInsights = "insights"
	// AssistantTabAutomations is what has been switched on to run by itself.
	AssistantTabAutomations = "automations"
)

// UseAssistantTab is the /assistant hub's active-tab atom. An atom (not hub-local
// state) so surfaces inside a tab can move the user to a sibling tab — e.g. the
// Ask rail's "See all in Insights" link — and so the sibling ROUTES (/insights,
// /smart) can open the hub on the right tab instead of rendering their own copy
// of it (C359, C392).
func UseAssistantTab() state.Atom[string] {
	return state.UseAtom(assistantTabAtomID, AssistantTabAsk)
}
