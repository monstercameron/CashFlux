// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"testing"
	"time"
)

func TestR1ADefaultTransactionFilterUsesCurrentMonth(t *testing.T) {
	now := time.Date(2026, time.October, 18, 17, 30, 0, 0, time.FixedZone("local", -4*60*60))
	got := defaultTxFilterAt(now)
	if got.From != "2026-10-01" || got.To != "2026-10-31" {
		t.Fatalf("default period = %s..%s, want 2026-10-01..2026-10-31", got.From, got.To)
	}
	if got.Page != 1 || got.PageSize != 25 || got.Sort != "date" || got.Dir != "desc" {
		t.Fatalf("default filter normalization = %+v, want newest-first first page", got)
	}
}
