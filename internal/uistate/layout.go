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
	// Reconcile against the current widget set so a layout saved by an older build
	// gains newly-introduced widgets (e.g. "attention") and sheds retired ones,
	// while keeping the user's order and sizes.
	return dashlayout.Reconcile(items)
}

const (
	layoutModeAtomID = "dashboard:layout-mode"
	layoutModeStore  = "cashflux:layout-mode"
)

// UseLayoutMode returns the shared dashboard layout-mode atom (custom /
// auto-default / auto-importance), seeded from localStorage. Custom is the
// default so an existing, hand-arranged dashboard keeps its order (C24).
func UseLayoutMode() state.Atom[dashlayout.Mode] {
	return state.UseAtom(layoutModeAtomID, loadLayoutMode())
}

// PersistLayoutMode saves the dashboard layout mode across reloads.
func PersistLayoutMode(m dashlayout.Mode) {
	if !m.Valid() {
		return
	}
	js.Global().Get("localStorage").Call("setItem", layoutModeStore, string(m))
}

// loadLayoutMode reads the saved mode, defaulting to Custom when absent/invalid.
func loadLayoutMode() dashlayout.Mode {
	v := js.Global().Get("localStorage").Call("getItem", layoutModeStore)
	if v.IsNull() || v.IsUndefined() {
		return dashlayout.ModeCustom
	}
	if m := dashlayout.Mode(v.String()); m.Valid() {
		return m
	}
	return dashlayout.ModeCustom
}

const dragSrcAtomID = "dashboard:drag-source"

// UseDragSource returns the shared atom holding the id of the widget currently
// being dragged ("" when none), so the drop target knows what to swap with.
func UseDragSource() state.Atom[string] {
	return state.UseAtom(dragSrcAtomID, "")
}

const dragPreviewAtomID = "dashboard:drag-preview"

// UseDragPreview returns the shared atom holding the id of the widget currently
// dragged *over* ("" when none). The render reorders the dragged tile in front
// of it for a live preview, without touching the persisted layout — so the
// reflow shows during the drag and reverts cleanly if the drop is cancelled (B2).
func UseDragPreview() state.Atom[string] {
	return state.UseAtom(dragPreviewAtomID, "")
}
