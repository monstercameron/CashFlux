//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

const addTargetAtomID = "ui:addTarget"

// UseAddTarget returns the shared atom tracking which entity's add form is open.
// An empty string means no modal is open. Known values: "account", "budget",
// "goal" (more added by follow-up work). The top-bar +Add menu sets it; the
// AddHost reads it to render the matching FlipPanel.
//
// Reading the atom here also captures it in the package-level var so
// SetAddTarget can be called from outside a component render (e.g. keyboard
// shortcuts, command-palette actions) without hitting hook-outside-component.
func UseAddTarget() state.Atom[string] {
	a := state.UseAtom(addTargetAtomID, "")
	capturedAddTarget = a
	addTargetCaptured = true
	return a
}

var (
	capturedAddTarget state.Atom[string]
	addTargetCaptured bool
)

// SetAddTarget opens or closes an entity's add-modal from outside a component
// render. Pass one of "account", "budget", "goal" to open, "" to close.
// No-op until AddHost has rendered once (capturing the atom).
func SetAddTarget(target string) {
	if addTargetCaptured {
		capturedAddTarget.Set(target)
	}
}
