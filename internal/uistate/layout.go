//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/GoWebComponents/state"
)

const layoutAtomID = "dashboard:layout"

// UseLayout returns the shared bento layout atom. The Widget shell reads it to
// place each widget; drag-reorder and resize write swapped/resized layouts back.
func UseLayout() state.Atom[dashlayout.Layout] {
	return state.UseAtom(layoutAtomID, dashlayout.Default())
}
