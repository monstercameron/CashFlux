// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/txnscope"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestR1DScopePresetClassification(t *testing.T) {
	now := time.Date(2026, time.October, 18, 12, 0, 0, 0, time.Local)
	monthFrom, monthTo := txnscope.MonthBounds(now)
	lastFrom, lastTo := txnscope.MonthBounds(now.AddDate(0, -1, 0))
	threeFrom, _ := txnscope.MonthBounds(now.AddDate(0, -2, 0))
	yearFrom := "2026-01-01"
	yearTo := "2026-12-31"
	tests := []struct {
		name string
		from string
		to   string
		want string
	}{
		{name: "current month", from: monthFrom, to: monthTo, want: "this-month"},
		{name: "last month", from: lastFrom, to: lastTo, want: "last-month"},
		{name: "last three months", from: threeFrom, to: monthTo, want: "last-three-months"},
		{name: "this year", from: yearFrom, to: yearTo, want: "this-year"},
		{name: "stepped future month", from: "2026-11-01", to: "2026-11-30", want: "shown-month"},
		{name: "all dates", want: "all-dates"},
		{name: "custom range", from: "2026-02-03", to: "2026-02-17", want: "custom"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := txnScopePreset(uistate.TxFilter{From: tc.from, To: tc.to}, now); got != tc.want {
				t.Fatalf("txnScopePreset(%q, %q) = %q, want %q", tc.from, tc.to, got, tc.want)
			}
		})
	}
}
