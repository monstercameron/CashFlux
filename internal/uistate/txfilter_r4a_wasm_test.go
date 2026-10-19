// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestR4ADefaultFilterStoresRollingMonthPreset(t *testing.T) {
	got := defaultTxFilterAt(time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC))
	if got.PeriodPreset != txnfilter.PeriodThisMonth || got.From != "2026-11-01" || got.To != "2026-11-30" {
		t.Fatalf("default filter = %+v, want this-month resolved to November", got)
	}
}

func TestR4APersistPresetOmitsAbsoluteDatesAndMigratesLegacyMonth(t *testing.T) {
	defer kvDelete(txFilterStoreID)
	now := time.Now()
	preset := defaultTxFilterAt(now)
	PersistTxFilter(preset)
	var stored txnfilter.Criteria
	if err := json.Unmarshal([]byte(kvGet(txFilterStoreID)), &stored); err != nil {
		t.Fatalf("decode persisted preset: %v", err)
	}
	if stored.PeriodPreset != txnfilter.PeriodThisMonth || stored.From != "" || stored.To != "" {
		t.Fatalf("stored rolling preset = %+v, want key only", stored)
	}
	if loaded := loadTxFilter(); loaded.From != preset.From || loaded.To != preset.To {
		t.Fatalf("loaded preset dates = %s..%s, want %s..%s", loaded.From, loaded.To, preset.From, preset.To)
	}

	legacy := txnfilter.Criteria{From: preset.From, To: preset.To}
	data, _ := json.Marshal(legacy)
	kvSet(txFilterStoreID, string(data))
	loaded := loadTxFilter()
	if loaded.PeriodPreset != txnfilter.PeriodThisMonth || loaded.From != preset.From || loaded.To != preset.To {
		t.Fatalf("legacy range did not migrate: %+v", loaded)
	}
	if err := json.Unmarshal([]byte(kvGet(txFilterStoreID)), &stored); err != nil {
		t.Fatalf("decode migrated preset: %v", err)
	}
	if stored.PeriodPreset != txnfilter.PeriodThisMonth || stored.From != "" || stored.To != "" {
		t.Fatalf("migrated stored filter = %+v, want key only", stored)
	}
}
