//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/state"
)

// TxFilter holds the transaction list's filter and sort selections. It persists
// to localStorage so the user's view is restored on reload.
type TxFilter struct {
	Text     string `json:"text,omitempty"`
	Account  string `json:"account,omitempty"`
	Category string `json:"category,omitempty"`
	Member   string `json:"member,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Sort     string `json:"sort,omitempty"`
}

// Normalize fills in defaults (the sort defaults to newest-first by date).
func (f TxFilter) Normalize() TxFilter {
	if f.Sort == "" {
		f.Sort = "date"
	}
	return f
}

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
