// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

// RuleDraft carries prefill data for the Rules add-form. It is set by the
// "Always categorize like this" action on a transaction row and consumed
// (then cleared) when the Rules screen mounts.
type RuleDraft struct {
	Match      string
	CategoryID string
}

// UseRuleDraft returns the shared atom holding the pending rule prefill
// (nil when there is nothing to prefill).
func UseRuleDraft() state.Atom[*RuleDraft] {
	return state.UseAtom("app:ruleDraft", (*RuleDraft)(nil))
}

// ruleDraftAtom is captured from a host component's render (the same host
// that captures the dialog atom) so that SetRuleDraft / ClearRuleDraft notify
// the correct subscriber.
var (
	ruleDraftAtom  state.Atom[*RuleDraft]
	ruleDraftReady bool
)

// CaptureRuleDraft registers the rule-draft atom the host renders with.
// Call from the host component each render (mirrors CaptureDialog).
func CaptureRuleDraft(a state.Atom[*RuleDraft]) {
	ruleDraftAtom, ruleDraftReady = a, true
}

// SetRuleDraft stores a prefill draft so the Rules screen can read and consume
// it on mount. match is the payee/description phrase; categoryID is the
// transaction's current category.
func SetRuleDraft(match, categoryID string) {
	if !ruleDraftReady {
		return
	}
	ruleDraftAtom.Set(&RuleDraft{Match: match, CategoryID: categoryID})
}

// ClearRuleDraft removes any pending prefill draft (call from the Rules screen
// after consuming it so a later visit starts blank).
func ClearRuleDraft() {
	if !ruleDraftReady {
		return
	}
	ruleDraftAtom.Set(nil)
}
