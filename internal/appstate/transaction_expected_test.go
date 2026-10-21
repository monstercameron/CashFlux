// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func expectedTxn(id, desc string) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "checking", Date: time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		Desc: desc, Payee: desc, Amount: money.New(-1299, "USD")}
}

func TestMergeDuplicateTransactionsExpectedCommitsCanonicalMerge(t *testing.T) {
	a := newApp(t, false)
	survivor := expectedTxn("a", "Coffee")
	survivor.Tags = []string{"work"}
	duplicate := expectedTxn("b", "coffee")
	duplicate.Tags = []string{"Lunch"}
	for _, row := range []domain.Transaction{survivor, duplicate} {
		if err := a.Store().PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.MergeDuplicateTransactionsExpected([]domain.Transaction{duplicate, survivor}, survivor.ID); err != nil {
		t.Fatal(err)
	}
	rows := a.Transactions()
	if len(rows) != 1 || rows[0].ID != survivor.ID || len(rows[0].Tags) != 2 {
		t.Fatalf("merged rows = %+v", rows)
	}
}

func TestMergeDuplicateTransactionsExpectedRefusesStaleOrIncompleteGroup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*App) error
		rows   []domain.Transaction
	}{
		{name: "edited member", mutate: func(a *App) error {
			row := expectedTxn("b", "Coffee")
			row.Note = "changed after preview"
			return a.Store().PutTransaction(row)
		}, rows: []domain.Transaction{expectedTxn("a", "Coffee"), expectedTxn("b", "Coffee")}},
		{name: "new member joins", mutate: func(a *App) error {
			return a.Store().PutTransaction(expectedTxn("c", "Coffee"))
		}, rows: []domain.Transaction{expectedTxn("a", "Coffee"), expectedTxn("b", "Coffee")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			for _, row := range []domain.Transaction{expectedTxn("a", "Coffee"), expectedTxn("b", "Coffee")} {
				if err := a.Store().PutTransaction(row); err != nil {
					t.Fatal(err)
				}
			}
			if err := tc.mutate(a); err != nil {
				t.Fatal(err)
			}
			if err := a.MergeDuplicateTransactionsExpected(tc.rows, "a"); err == nil {
				t.Fatal("stale or incomplete preview unexpectedly merged")
			}
			if got := len(a.Transactions()); got != 2 && tc.name == "edited member" {
				t.Fatalf("stale merge changed ledger; rows=%d", got)
			}
		})
	}
}

func TestDeleteTransactionExpectedRequiresPreviewedTransferPartner(t *testing.T) {
	a := newApp(t, false)
	when := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	left := domain.Transaction{ID: "out", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "pair", Date: when, Desc: "Transfer", Amount: money.New(-5000, "USD")}
	right := domain.Transaction{ID: "in", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "pair", Date: when, Desc: "Transfer", Amount: money.New(5000, "USD")}
	for _, row := range []domain.Transaction{left, right} {
		if err := a.Store().PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.DeleteTransactionExpected(left.ID, []domain.Transaction{left}); err == nil {
		t.Fatal("delete without previewing paired leg unexpectedly succeeded")
	}
	if got := len(a.Transactions()); got != 2 {
		t.Fatalf("refused delete changed ledger; rows=%d", got)
	}
	if err := a.DeleteTransactionExpected(left.ID, []domain.Transaction{left, right}); err != nil {
		t.Fatal(err)
	}
	if got := len(a.Transactions()); got != 0 {
		t.Fatalf("paired deletion left %d rows", got)
	}
}
