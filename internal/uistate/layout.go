//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	layoutAtomID  = "dashboard:layout"
	layoutStoreID = "cashflux:layout"
)

// UseLayoutItems returns the shared bento layout atom: the ordered widget items
// (id + intrinsic spans) that dashlayout.Pack flows into the grid. Seeded from
// localStorage so a rearranged dashboard survives reloads. The Widget shell Packs
// it to place each widget; drag-reorder (Move) and resize (ResizeItem) rewrite it
// and persist via PersistItems.
func UseLayoutItems() state.Atom[[]dashlayout.Item] {
	return state.UseAtom(layoutAtomID, loadItems())
}

// PersistItems saves the ordered items to localStorage. Call it after writing the
// atom so the arrangement is remembered across reloads.
func PersistItems(items []dashlayout.Item) {
	data, err := json.Marshal(items)
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", layoutStoreID, string(data))
}

// loadItems reads the saved items from localStorage, falling back to the default
// arrangement when absent or invalid. The legacy []Placement format migrates for
// free: unmarshaling it into []Item picks up id + spans and ignores col/row.
func loadItems() []dashlayout.Item {
	v := js.Global().Get("localStorage").Call("getItem", layoutStoreID)
	if v.IsNull() || v.IsUndefined() {
		return dashlayout.DefaultItems()
	}
	var items []dashlayout.Item
	if err := json.Unmarshal([]byte(v.String()), &items); err != nil || len(items) == 0 || items[0].ID == "" {
		return dashlayout.DefaultItems()
	}
	return items
}

const dragSrcAtomID = "dashboard:drag-source"

// UseDragSource returns the shared atom holding the id of the widget currently
// being dragged ("" when none), so the drop target knows what to swap with.
func UseDragSource() state.Atom[string] {
	return state.UseAtom(dragSrcAtomID, "")
}
