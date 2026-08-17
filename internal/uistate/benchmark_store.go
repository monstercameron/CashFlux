// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/benchseries"
)

// benchmarkKey is the SettingKV key for the user-imported comparison series
// (C380).
//
// It lives in the PRESERVED settings KV rather than the dataset: a benchmark is
// reference data the user went and fetched by hand, not household financial
// data, and losing it on a wipe would cost effort the app cannot recreate — it
// has no market feed to re-fetch from, which is the whole reason the import
// exists.
const benchmarkKey = "cashflux:benchmark-series"

// LoadBenchmark reads the imported comparison series. A missing or malformed
// value yields an empty series, which every caller reads as "no comparison" —
// the same as never having imported one.
func LoadBenchmark() benchseries.Series {
	raw := SettingKVGet(benchmarkKey)
	if raw == "" {
		return benchseries.Series{}
	}
	var s benchseries.Series
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return benchseries.Series{}
	}
	return s
}

// SaveBenchmark persists a comparison series, clearing the key entirely when the
// series is empty so removing a benchmark leaves nothing behind.
func SaveBenchmark(s benchseries.Series) {
	if s.Empty() {
		SettingKVSet(benchmarkKey, "")
		BumpDataRevision()
		return
	}
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	SettingKVSet(benchmarkKey, string(b))
	RequestPersist()
	BumpDataRevision()
}
