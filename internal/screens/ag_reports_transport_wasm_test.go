// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAgRenderTxnsBoundsWholeResponseAndNamesOmittedRows(t *testing.T) {
	ctx := agRptCtx{
		rates:    currency.Rates{Base: "USD"},
		base:     "USD",
		label:    "this month",
		fmtM:     func(n int64) string { return fmtMoney(money.New(n, "USD")) },
		nameOf:   func(string) string { return "Groceries" },
		memberOf: func(string) string { return "Household" },
	}
	rows := make([]domain.Transaction, 100)
	for i := range rows {
		rows[i] = domain.Transaction{
			ID: "txn-" + fmt.Sprintf("%03d", i), Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			Payee: strings.Repeat("商店", 200), Amount: money.New(-1234, "USD"),
		}
	}
	out := agRenderTxns(ctx, rows, "the selected window", 100)
	if len(out) > agTxnToolOutputLimit {
		t.Fatalf("encoded response is %d bytes, limit is %d", len(out), agTxnToolOutputLimit)
	}
	if !strings.Contains(out, "omitted from this response") || !strings.Contains(out, "Omitted transaction IDs:") {
		t.Fatalf("oversized result did not explain omitted rows: %s", out[len(out)-min(500, len(out)):])
	}
	if !strings.Contains(out, `"txn-099"`) || !strings.Contains(out, "txn-000") {
		t.Fatalf("omitted IDs are not exact or displayed rows disappeared")
	}
	if !strings.Contains(out, "query_data") || !strings.Contains(out, "read_record") {
		t.Fatalf("missing recovery instructions: %s", out[len(out)-min(500, len(out)):])
	}
}
