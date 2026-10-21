// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func transactionWriteTestApp(t *testing.T, rows ...domain.Transaction) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	for _, row := range rows {
		if err := app.Store().PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func transactionWriteTestRow(id, desc string) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "checking", Date: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		Desc: desc, Payee: desc, Amount: money.New(-1299, "USD")}
}

func transactionWriteTestTool(t *testing.T, app *appstate.App, name string) chatTool {
	t.Helper()
	for _, tool := range agToolsTransactionWrites(app) {
		if tool.spec.Function.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return chatTool{}
}

func TestTransactionWriteArgsAreStrictAndExact(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"target":"coffee","unexpected":true}`, `{"target":"coffee","date":"2026-2-03"}`,
		`{"target":"coffee","date":"2026-02-30"}`, `{"target":"coffee","amount":12.99}`,
		`{"target":"coffee","amount":"12.9999999"}`, `{"target":"coffee","amount":"-12.99"}`,
		`{"target":"coffee"} {"target":"other"}`,
	} {
		if _, err := parseTransactionWriteArgs(json.RawMessage(raw)); err == nil {
			t.Errorf("parseTransactionWriteArgs(%s) unexpectedly succeeded", raw)
		}
	}
	if _, err := parseTransactionWriteArgs(json.RawMessage(`{"target":" coffee ","amount":"12.99","date":"2026-02-03"}`)); err != nil {
		t.Fatalf("valid exact args rejected: %v", err)
	}
	args, err := parseTransactionWriteArgs(json.RawMessage(`{"target":"coffee","amount":"12.999"}`))
	if err != nil {
		t.Fatalf("currency-precision check should wait for candidate currency: %v", err)
	}
	if transactionMatchesWriteArgs(transactionWriteTestRow("usd", "Coffee"), args) {
		t.Fatal("three decimal places unexpectedly matched a two-decimal USD record")
	}
}

func TestDeleteTransactionToolRefusesAmbiguousMatches(t *testing.T) {
	a := transactionWriteTestApp(t, transactionWriteTestRow("a", "Coffee shop"), transactionWriteTestRow("b", "Coffee shop"))
	tool := transactionWriteTestTool(t, a, "delete_transaction")
	raw := json.RawMessage(`{"target":"coffee"}`)
	preview := tool.preview(raw)
	if !strings.Contains(preview, "2 transactions match") {
		t.Fatalf("ambiguous preview = %q", preview)
	}
	if out := tool.run(raw); !strings.Contains(out, "2 transactions match") || len(a.Transactions()) != 2 {
		t.Fatalf("ambiguous call changed data: %q; rows=%d", out, len(a.Transactions()))
	}
	// An exact transaction id narrows the same target to one row.
	narrow := json.RawMessage(`{"target":"coffee","transaction_id":"a"}`)
	preview = tool.preview(narrow)
	if !strings.Contains(preview, "ID a") || strings.Contains(preview, "ID b") {
		t.Fatalf("narrow preview = %q", preview)
	}
	if out := tool.run(narrow); !strings.Contains(out, "Deleted") || len(a.Transactions()) != 1 {
		t.Fatalf("exact delete = %q; rows=%d", out, len(a.Transactions()))
	}
}

func TestDeleteTransactionToolRefusesAfterPreviewEditAndIsIdempotent(t *testing.T) {
	row := transactionWriteTestRow("a", "Coffee shop")
	a := transactionWriteTestApp(t, row)
	tool := transactionWriteTestTool(t, a, "delete_transaction")
	raw := json.RawMessage(`{"target":"coffee","amount":"12.99","date":"2026-04-03"}`)
	preview := tool.preview(raw)
	for _, want := range []string{"ID a", "2026-04-03", "12.99"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview %q lacks %q", preview, want)
		}
	}
	changed := row
	changed.Note = "edited while approval was open"
	if err := a.Store().PutTransaction(changed); err != nil {
		t.Fatal(err)
	}
	if out := tool.run(raw); !strings.Contains(out, "changed after approval") || len(a.Transactions()) != 1 {
		t.Fatalf("stale delete = %q; rows=%d", out, len(a.Transactions()))
	}

	// A fresh approval applies once; replaying the same invocation does not write again.
	tool.preview(raw)
	if out := tool.run(raw); !strings.Contains(out, "Deleted") || len(a.Transactions()) != 0 {
		t.Fatalf("approved delete = %q; rows=%d", out, len(a.Transactions()))
	}
	revision := a.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already processed") || a.Rev() != revision {
		t.Fatalf("repeated invocation was not idempotent: %q", out)
	}
}

func TestMergeDuplicateToolShowsSurvivorAndUsesCanonicalMerge(t *testing.T) {
	survivor := transactionWriteTestRow("a", "Coffee shop")
	survivor.Tags = []string{"work"}
	duplicate := transactionWriteTestRow("b", "coffee shop")
	duplicate.Tags = []string{"lunch"}
	a := transactionWriteTestApp(t, duplicate, survivor)
	tool := transactionWriteTestTool(t, a, "merge_duplicate_transactions")
	raw := json.RawMessage(`{"target":"coffee","amount":"12.99","date":"2026-04-03"}`)
	preview := tool.preview(raw)
	for _, want := range []string{"2 exact duplicate transactions", "survivor a", "ID a", "ID b"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("merge preview %q lacks %q", preview, want)
		}
	}
	if out := tool.run(raw); !strings.Contains(out, "keeping transaction a") {
		t.Fatalf("merge result = %q", out)
	}
	rows := a.Transactions()
	if len(rows) != 1 || rows[0].ID != "a" || len(rows[0].Tags) != 2 {
		t.Fatalf("canonical merge result = %+v", rows)
	}
}

func TestTransactionWriteToolsRequirePreviewAndExcludeTransfers(t *testing.T) {
	row := transactionWriteTestRow("a", "Coffee shop")
	transfer := transactionWriteTestRow("transfer", "Coffee transfer")
	transfer.TransferAccountID = "savings"
	a := transactionWriteTestApp(t, row, transfer)
	tool := transactionWriteTestTool(t, a, "delete_transaction")
	raw := json.RawMessage(`{"target":"coffee","transaction_id":"transfer"}`)
	if out := tool.run(raw); !strings.Contains(out, "no matching approval preview") || len(a.Transactions()) != 2 {
		t.Fatalf("run without preview = %q; rows=%d", out, len(a.Transactions()))
	}
	preview := tool.preview(raw)
	if !strings.Contains(preview, "no non-transfer transaction") || len(a.Transactions()) != 2 {
		t.Fatalf("transfer preview = %q", preview)
	}
	if out := tool.run(raw); !strings.Contains(out, "no non-transfer transaction") || len(a.Transactions()) != 2 {
		t.Fatalf("transfer deletion = %q; rows=%d", out, len(a.Transactions()))
	}
}
