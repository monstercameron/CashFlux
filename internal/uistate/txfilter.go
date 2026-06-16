//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/GoWebComponents/state"
)

// TxFilter is the transaction list's filter/sort selection. It aliases the pure
// txnfilter.Criteria (where the apply logic lives and is tested); this file only
// persists it to localStorage so the user's view survives reloads.
type TxFilter = txnfilter.Criteria

const (
	txFilterAtomID  = "transactions:filter"
	txFilterStoreID = "cashflux:tx-filter"
)

// UseTxFilter returns the shared transaction-filter atom, seeded from
// localStorage so the last filter/sort is restored on reload.
func UseTxFilter() state.Atom[TxFilter] {
	return state.UseAtom(txFilterAtomID, loadTxFilter())
}

// PersistTxFilter saves the filter to localStorage.
func PersistTxFilter(f TxFilter) {
	data, err := json.Marshal(f.Normalize())
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", txFilterStoreID, string(data))
}

// loadTxFilter reads the saved filter from localStorage, defaulting to an empty
// (newest-first) filter when absent or invalid. Always normalized.
func loadTxFilter() TxFilter {
	v := js.Global().Get("localStorage").Call("getItem", txFilterStoreID)
	if v.IsNull() || v.IsUndefined() {
		return TxFilter{}.Normalize()
	}
	var f TxFilter
	if err := json.Unmarshal([]byte(v.String()), &f); err != nil {
		return TxFilter{}.Normalize()
	}
	return f.Normalize()
}
