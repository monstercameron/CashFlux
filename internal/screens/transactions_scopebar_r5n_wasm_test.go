// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnscope"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestTxnScopePresetSelectionsCarryExplicitKeys(t *testing.T) {
	now := time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name, selection, key string
	}{
		{"this month", "this-month", txnfilter.PeriodThisMonth},
		{"last month", "last-month", txnfilter.PeriodLastMonth},
		{"last three months", "last-three-months", txnfilter.PeriodLastThreeMonths},
		{"this year", "this-year", txnfilter.PeriodThisYear},
		{"all dates", "all-dates", txnfilter.PeriodAllDates},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, key, ok := txnScopePresetRange(tt.selection, now)
			if !ok || key != tt.key {
				t.Fatalf("preset selection key = %q, %v; want %q", key, ok, tt.key)
			}
		})
	}
}

func TestTxnScopeDatePickerRangeStaysCustomWhenItMatchesPreset(t *testing.T) {
	now := time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC)
	monthFrom, monthTo := txnscope.MonthBounds(now)
	from, to, key := txnScopeCustomRange(monthFrom, monthTo)
	if key != txnfilter.PeriodCustom {
		t.Fatalf("date picker range key = %q, want custom", key)
	}
	filter := uistate.TxFilter{From: from, To: to, PeriodPreset: key}
	if got := txnScopePreset(filter, now); got != "custom" {
		t.Fatalf("matching custom range displays preset %q, want custom", got)
	}
}

func TestTxnScopeMonthArrowsCarryStableMonthKey(t *testing.T) {
	now := time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC)
	scope := txnscope.Of("2026-10-01", "2026-10-31", now)
	from, to, key := txnScopeStepRange(scope, -1)
	if from != "2026-09-01" || to != "2026-09-30" || key != txnfilter.MonthPeriodPreset(2026, time.September) {
		t.Fatalf("previous month = %q..%q (%q), want September 2026 stable month key", from, to, key)
	}
}
