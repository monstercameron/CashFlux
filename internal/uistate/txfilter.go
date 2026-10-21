// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// TxFilter is the transaction list's filter/sort selection. It aliases the pure
// txnfilter.Criteria (where the apply logic lives and is tested); this file only
// persists it to localStorage so the user's view survives reloads.
type TxFilter = txnfilter.Criteria

const (
	txFilterAtomID  = "transactions:filter"
	txFilterStoreID = "cashflux:tx-filter"
)

// UseTxFilter returns the shared transaction-filter atom, seeded from the
// persisted filter so the last filter/sort is restored on reload.
func UseTxFilter() state.Atom[TxFilter] {
	return useTxFilterWithLoader(loadTxFilter)
}

// UseAtom only consumes its default when the fixed atom is first created. Keep
// that seed local to the mounted hook so row/menu renders do not reread SQLite
// or decode ignored JSON. A late subscriber/remount still loads a fresh default
// without replacing live atom state. Workspace switches reload the runtime.
func useTxFilterWithLoader(load func() TxFilter) state.Atom[TxFilter] {
	initial := ui.UseMemo(load, appstate.Default)
	return state.UseAtom(txFilterAtomID, initial)
}

// CurrentTxFilter reads the persisted filter WITHOUT subscribing to the atom, for
// callers outside a render — a click handler that is about to navigate away and
// wants to name the working set it is leaving (C581). Inside a render, use
// UseTxFilter: this one will not re-render anything when the filter changes.
func CurrentTxFilter() TxFilter { return loadTxFilter() }

// PersistTxFilter saves the filter to localStorage.
func PersistTxFilter(f TxFilter) {
	now := time.Now()
	var previous TxFilter
	if raw := kvGet(txFilterStoreID); raw != "" {
		_ = json.Unmarshal([]byte(raw), &previous)
	}
	f = f.ResolvePeriodChange(previous, now).Normalize()
	stored := f
	if stored.PeriodPreset != txnfilter.PeriodCustom {
		stored.From, stored.To = "", ""
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return
	}
	kvSet(txFilterStoreID, string(data))
}

// loadTxFilter reads the saved filter from localStorage, defaulting to an empty
// (newest-first) filter when absent or invalid. Always normalized.
func loadTxFilter() TxFilter {
	raw := kvGet(txFilterStoreID)
	if raw == "" {
		return defaultTxFilterAt(time.Now())
	}
	var f TxFilter
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return defaultTxFilterAt(time.Now())
	}
	resolved := f.ResolvePeriod(time.Now()).Normalize()
	if f.PeriodPreset != resolved.PeriodPreset || (resolved.PeriodPreset != txnfilter.PeriodCustom && (f.From != "" || f.To != "")) {
		stored := resolved
		if stored.PeriodPreset != txnfilter.PeriodCustom {
			stored.From, stored.To = "", ""
		}
		if data, err := json.Marshal(stored); err == nil {
			kvSet(txFilterStoreID, string(data))
		}
	}
	return resolved
}

func defaultTxFilterAt(now time.Time) TxFilter {
	start := dateutil.MonthStart(now)
	end := dateutil.AddMonths(start, 1).AddDate(0, 0, -1)
	f := (TxFilter{From: dateutil.FormatDate(start), To: dateutil.FormatDate(end), PeriodPreset: txnfilter.PeriodThisMonth}).Normalize()
	f.PeriodPreset = txnfilter.PeriodThisMonth
	return f
}
