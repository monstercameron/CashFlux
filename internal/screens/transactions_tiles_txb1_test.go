// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/nlfilter"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestApplyParsedDateBoundsReplacesOpenEndAndPreservesAbsentClause(t *testing.T) {
	current := uistate.TxFilter{From: "2026-01-01", To: "2026-05-31"}
	since, ok := nlfilter.Parse("since march", nlfilter.Context{Now: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)})
	if !ok {
		t.Fatal("since March was not parsed")
	}
	applyParsedDateBounds(&current, since)
	if current.From != "2026-03-01" || current.To != "" {
		t.Fatalf("since March applied as [%q,%q], want [2026-03-01,open]", current.From, current.To)
	}

	unchanged := uistate.TxFilter{From: "2026-01-01", To: "2026-05-31"}
	applyParsedDateBounds(&unchanged, txnfilter.Criteria{Flow: "out"})
	if unchanged.From != "2026-01-01" || unchanged.To != "2026-05-31" {
		t.Fatalf("date-free parse changed bounds to [%q,%q]", unchanged.From, unchanged.To)
	}
}
