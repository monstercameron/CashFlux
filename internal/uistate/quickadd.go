//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

const quickAddAtomID = "quickadd:open"

// UseQuickAdd returns the shared atom tracking whether the quick-add transaction
// panel is open. The top bar's "+ Add" button sets it true; the quick-add host
// (at the shell root) reads it to render the flip panel.
func UseQuickAdd() state.Atom[bool] {
	return state.UseAtom(quickAddAtomID, false)
}
