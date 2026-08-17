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
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// The Review commit path, exercised against a real appstate.
//
//	C616 — one-at-a-time confirm passed the RAW payee where a normalized
//	       reviewqueue.MerchantKey was required, so it matched zero rows, wrote
//	       nothing, and (correctly, after C553) reported "That didn't save".
//	C617 — a confirmed decision left `Reviewed` false, so the row left the queue
//	       while the ledger still badged it "Needs review".
//	C653 — a card depicting ONE charge wrote every charge from the merchant, 122
//	       of them, under a button reading "Categorize & next".
//	C554 — Review read the whole ledger, so a queue opened from a filtered view
//	       still worked through unrelated merchants and dates.
//
// All of them are about what the STORE ends up holding, so they assert on stored
// transactions rather than on any rendering.

// reviewTestApp installs an appstate holding one uncategorized charge whose payee
// is deliberately NOT already normalized — upper case with a trailing reference
// number, i.e. what an import actually produces. A payee that happened to be
// lowercase would have passed even with the C616 bug.
func reviewTestApp(t *testing.T) (*appstate.App, domain.Transaction) {
	t.Helper()
	app := newReviewApp(t)
	txn := domain.Transaction{
		ID: "txn-1", AccountID: "acct-1", Date: time.Now(),
		Payee: "VENMO PAYMENT 1042778120", Desc: "VENMO PAYMENT 1042778120",
		Amount: money.New(-3200, "USD"),
	}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	return app, txn
}

// newReviewApp is reviewTestApp's fixture without any transactions, for the tests
// that need to shape their own ledger.
func newReviewApp(t *testing.T) *appstate.App {
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
	prev := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = prev })
	return app
}

// putCharge adds one uncategorized charge from a merchant, so a test can build a
// group of siblings the way an import does.
func putCharge(t *testing.T, app *appstate.App, id, payee string, day int) domain.Transaction {
	t.Helper()
	txn := domain.Transaction{
		ID: id, AccountID: "acct-1",
		Date:  time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC),
		Payee: payee, Desc: payee, Amount: money.New(-1200, "USD"),
	}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatalf("PutTransaction %s: %v", id, err)
	}
	return txn
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

// TestApplyReviewChoiceWritesWithARawPayee is the C616 guard, kept after the
// C653 refactor removed its original cause. The confirm path no longer re-derives
// a merchant key at all, so a raw descriptor cannot mismatch a normalized one —
// but the payee shape that exposed it is still the one imports produce, and a
// future keyed shortcut must fail here rather than in the queue.
func TestApplyReviewChoiceWritesWithARawPayee(t *testing.T) {
	app, txn := reviewTestApp(t)

	// Precondition: the raw payee and the queue's key genuinely differ, or this
	// test would pass against the bug.
	if reviewqueue.MerchantKey(txn) == txn.Payee {
		t.Fatalf("test setup is toothless: MerchantKey == raw payee (%q)", txn.Payee)
	}

	if ok := applyReviewChoice(app, []string{txn.ID}, "cat-food"); !ok {
		t.Fatal("applyReviewChoice reported no write (C616)")
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
	if ok := applyReviewChoice(app, []string{txn.ID}, "cat-food"); !ok {
		t.Fatal("write did not land")
	}
	got := storedTxn(t, app, txn.ID)
	if reviewqueue.Needs(got) {
		t.Error("still queued after confirmation")
	}
	if !got.Reviewed {
		t.Error("Reviewed is false after an explicit confirmation — the row leaves the " +
			"inbox while the ledger still badges it 'Needs review' (C617)")
	}
}

// TestConfirmingOneChargeLeavesItsSiblingsAlone is the C653 guard, and the whole
// point of the refactor: the surface hands over the ids it drew, so a card
// showing one charge writes one charge no matter how large the merchant group is.
func TestConfirmingOneChargeLeavesItsSiblingsAlone(t *testing.T) {
	app := newReviewApp(t)
	one := putCharge(t, app, "bb-1", "BLUE BOTTLE COFFEE #221", 3)
	putCharge(t, app, "bb-2", "BLUE BOTTLE COFFEE #488", 5)
	putCharge(t, app, "bb-3", "BLUE BOTTLE COFFEE #902", 9)

	targets := reviewscope.Targets(reviewscope.CommitCharge, one.ID, []string{"bb-1", "bb-2", "bb-3"})
	if len(targets) != 1 {
		t.Fatalf("Targets(charge) = %v, want exactly the card's own charge", targets)
	}
	if ok := applyReviewChoice(app, targets, "cat-food"); !ok {
		t.Fatal("write did not land")
	}
	if got := storedTxn(t, app, "bb-1"); got.CategoryID != "cat-food" {
		t.Errorf("bb-1 CategoryID = %q, want cat-food", got.CategoryID)
	}
	for _, id := range []string{"bb-2", "bb-3"} {
		got := storedTxn(t, app, id)
		if got.CategoryID != "" || got.Reviewed {
			t.Errorf("%s was categorized by a confirmation on a card showing bb-1 only "+
				"(category=%q reviewed=%v) — one click, 3 rows (C653)", id, got.CategoryID, got.Reviewed)
		}
		if !reviewqueue.Needs(got) {
			t.Errorf("%s left the review queue without being confirmed (C653)", id)
		}
	}
}

// TestConfirmingTheMerchantScopeWritesExactlyTheGroup is the other half: when the
// user opts into the merchant scope, every charge the card counted is written and
// nothing else is.
func TestConfirmingTheMerchantScopeWritesExactlyTheGroup(t *testing.T) {
	app := newReviewApp(t)
	one := putCharge(t, app, "bb-1", "BLUE BOTTLE COFFEE #221", 3)
	putCharge(t, app, "bb-2", "BLUE BOTTLE COFFEE #488", 5)
	putCharge(t, app, "other-1", "SHELL 4471", 6)

	targets := reviewscope.Targets(reviewscope.CommitMerchant, one.ID, []string{"bb-1", "bb-2"})
	if ok := applyReviewChoice(app, targets, "cat-food"); !ok {
		t.Fatal("write did not land")
	}
	for _, id := range []string{"bb-1", "bb-2"} {
		if got := storedTxn(t, app, id); got.CategoryID != "cat-food" || !got.Reviewed {
			t.Errorf("%s = (%q, reviewed=%v), want the merchant-wide confirmation to cover it",
				id, got.CategoryID, got.Reviewed)
		}
	}
	if got := storedTxn(t, app, "other-1"); got.CategoryID != "" {
		t.Errorf("a different merchant's charge was written by the group confirmation: %q", got.CategoryID)
	}
}

// TestAConfirmationNeverResurrectsAResolvedCharge covers the stale-card race the
// id list opens up: the surface drew the group a moment ago, and one of those
// charges has since been categorized elsewhere. It must not be re-written with
// the card's category.
func TestAConfirmationNeverResurrectsAResolvedCharge(t *testing.T) {
	app := newReviewApp(t)
	putCharge(t, app, "bb-1", "BLUE BOTTLE COFFEE #221", 3)
	settled := putCharge(t, app, "bb-2", "BLUE BOTTLE COFFEE #488", 5)
	settled.CategoryID = "cat-food"
	settled.Reviewed = true
	if err := app.PutTransaction(settled); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "cat-other", Name: "Fun", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}

	if n := assignReviewToCharges(app, []string{"bb-1", "bb-2"}, "cat-other"); n != 1 {
		t.Errorf("wrote %d charges, want 1 — a charge that already left the queue must not be re-categorized", n)
	}
	if got := storedTxn(t, app, "bb-2"); got.CategoryID != "cat-food" {
		t.Errorf("bb-2 CategoryID = %q, want cat-food (its settled value)", got.CategoryID)
	}
}

// TestReviewScopeFollowsTheLedgerFilter is the C554 guard: with the ledger
// searched to one merchant, the "current view" scope queues only that merchant,
// while the entire-queue denominator still reports everything.
func TestReviewScopeFollowsTheLedgerFilter(t *testing.T) {
	app := newReviewApp(t)
	putCharge(t, app, "bb-1", "BLUE BOTTLE COFFEE #221", 3)
	putCharge(t, app, "bb-2", "BLUE BOTTLE COFFEE #488", 5)
	putCharge(t, app, "shell-1", "SHELL 4471", 6)

	ledger := txnfilter.Criteria{Text: "blue bottle"}

	view := buildReviewIndex(app, reviewscope.ScopeCurrentView, ledger)
	if view.Total != 2 {
		t.Errorf("current-view Total = %d, want 2 — Review must inherit the ledger's search (C554)", view.Total)
	}
	if view.QueueTotal != 3 {
		t.Errorf("QueueTotal = %d, want 3 — the other scope's denominator must stay honest", view.QueueTotal)
	}
	if view.ViewTotal != 2 {
		t.Errorf("ViewTotal = %d, want 2", view.ViewTotal)
	}
	for _, r := range view.Rows {
		for _, item := range r.Group.Items {
			if item.ID == "shell-1" {
				t.Error("a charge outside the ledger's filter is in the current-view queue (C554)")
			}
		}
	}

	all := buildReviewIndex(app, reviewscope.ScopeEntireQueue, ledger)
	if all.Total != 3 {
		t.Errorf("entire-queue Total = %d, want 3 — switching scope must widen it back", all.Total)
	}
	if all.ViewTotal != 2 {
		t.Errorf("entire-queue ViewTotal = %d, want 2 — the strip states BOTH counts in either scope", all.ViewTotal)
	}
}

// TestReviewScopeIgnoresLedgerPagination pins the deliberate decision in
// ledgerVisibleIDs: paging is a property of the ledger's screen, not of the
// working set. Reviewing "what I am looking at" after filtering to 3 charges must
// not mean "the 1 on page two".
func TestReviewScopeIgnoresLedgerPagination(t *testing.T) {
	app := newReviewApp(t)
	putCharge(t, app, "bb-1", "BLUE BOTTLE COFFEE #221", 3)
	putCharge(t, app, "bb-2", "BLUE BOTTLE COFFEE #488", 5)
	putCharge(t, app, "bb-3", "BLUE BOTTLE COFFEE #902", 9)

	paged := txnfilter.Criteria{Text: "blue bottle", Page: 2, PageSize: 1}
	if got := buildReviewIndex(app, reviewscope.ScopeCurrentView, paged).Total; got != 3 {
		t.Errorf("current-view Total = %d, want 3 — the ledger's page size must not narrow the queue", got)
	}
}

// TestAFilterThatMatchesNothingReviewsNothing guards the failure mode that would
// make the scope feel random: an empty visible set falling back to the whole
// queue, so "current view" would silently mean "everything".
func TestAFilterThatMatchesNothingReviewsNothing(t *testing.T) {
	app := newReviewApp(t)
	putCharge(t, app, "bb-1", "BLUE BOTTLE COFFEE #221", 3)

	idx := buildReviewIndex(app, reviewscope.ScopeCurrentView, txnfilter.Criteria{Text: "nothing matches this"})
	if idx.Total != 0 {
		t.Errorf("Total = %d, want 0", idx.Total)
	}
	if idx.QueueTotal != 1 {
		t.Errorf("QueueTotal = %d, want 1 — the empty state has to be able to say what is waiting outside the view", idx.QueueTotal)
	}
}
