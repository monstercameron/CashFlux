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

const dragSrcAtomID = "dashboard:drag-source"

// UseDragSource returns the shared atom holding the id of the widget currently
// being dragged ("" when none), so the drop target knows what to swap with.
func UseDragSource() state.Atom[string] {
	return state.UseAtom(dragSrcAtomID, "")
}
