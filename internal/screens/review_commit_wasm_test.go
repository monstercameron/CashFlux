// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
)

// The Review commit path, exercised against a real appstate.
//
//	C616 — one-at-a-time confirm passed the RAW payee where a normalized
//	       reviewqueue.MerchantKey was required, so it matched zero rows, wrote
//	       nothing, and (correctly, after C553) reported "That didn't save".
//	C617 — a confirmed decision left `Reviewed` false, so the row left the queue
//	       while the ledger still badged it "Needs review".
//
// Both are about what the STORE ends up holding, so they assert on the stored
// transaction rather than on any rendering.

// reviewTestApp installs an appstate holding one uncategorized charge whose payee
// is deliberately NOT already normalized — upper case with a trailing reference
// number, i.e. what an import actually produces. A payee that happened to be
// lowercase would have passed even with the C616 bug.
func reviewTestApp(t *testing.T) (*appstate.App, domain.Transaction) {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	if err := app.PutAccount(domain.Account{
		ID: "acct-1", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "cat-food", Name: "Groceries", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	txn := domain.Transaction{
		ID: "txn-1", AccountID: "acct-1", Date: time.Now(),
		Payee: "VENMO PAYMENT 1042778120", Desc: "VENMO PAYMENT 1042778120",
		Amount: money.New(-3200, "USD"),
	}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	prev := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = prev })
	return app, txn
}

func storedTxn(t *testing.T, app *appstate.App, id string) domain.Transaction {
	t.Helper()
	for _, x := range app.Transactions() {
		if x.ID == id {
			return x
		}
	}
	t.Fatalf("transaction %s not found", id)
	return domain.Transaction{}
}

// TestApplyReviewChoiceBatchWritesWithARawPayee is the C616 guard. The batch
// branch is what BOTH the single-card confirm and the merchant-group confirm go
// through, so a key mismatch there breaks the guided flow entirely.
func TestApplyReviewChoiceBatchWritesWithARawPayee(t *testing.T) {
	app, txn := reviewTestApp(t)

	// Precondition: the raw payee and the queue's key genuinely differ, or this
	// test would pass against the bug.
	if reviewqueue.MerchantKey(txn) == txn.Payee {
		t.Fatalf("test setup is toothless: MerchantKey == raw payee (%q)", txn.Payee)
	}

	if ok := applyReviewChoice(app, txn, "cat-food", true); !ok {
		t.Fatal("applyReviewChoice reported no write — the merchant key never matched the row it came from (C616)")
	}
	got := storedTxn(t, app, txn.ID)
	if got.CategoryID != "cat-food" {
		t.Errorf("CategoryID = %q, want %q", got.CategoryID, "cat-food")
	}
}

// TestApplyReviewChoiceSingleWrites covers the non-batch branch with the same
// awkward payee, so neither path can regress independently.
func TestApplyReviewChoiceSingleWrites(t *testing.T) {
	app, txn := reviewTestApp(t)
	if ok := applyReviewChoice(app, txn, "cat-food", false); !ok {
		t.Fatal("applyReviewChoice(batch=false) reported no write")
	}
	if got := storedTxn(t, app, txn.ID); got.CategoryID != "cat-food" {
		t.Errorf("CategoryID = %q, want %q", got.CategoryID, "cat-food")
	}
}

// TestConfirmingAReviewLeavesTheQueueAndTheLedgerAgreeing is the C617 guard: a
// confirmed row must be out of the queue AND no longer read as awaiting review.
// Before this, it did the first and not the second — the app contradicting
// itself about one transaction.
func TestConfirmingAReviewLeavesTheQueueAndTheLedgerAgreeing(t *testing.T) {
	app, txn := reviewTestApp(t)

	if !reviewqueue.Needs(storedTxn(t, app, txn.ID)) {
		t.Fatal("setup: the charge should start in the review queue")
	}
	for _, batch := range []bool{true, false} {
		app, txn = reviewTestApp(t)
		if ok := applyReviewChoice(app, txn, "cat-food", batch); !ok {
			t.Fatalf("batch=%v: write did not land", batch)
		}
		got := storedTxn(t, app, txn.ID)
		if reviewqueue.Needs(got) {
			t.Errorf("batch=%v: still queued after confirmation", batch)
		}
		if !got.Reviewed {
			t.Errorf("batch=%v: Reviewed is false after an explicit confirmation — "+
				"the row leaves the inbox while the ledger still badges it 'Needs review' (C617)", batch)
		}
	}
}
