// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

// C677: the transfer RELATIONSHIP and the payment INTENT are two different
// claims, and the second has to survive a round trip through the store — a
// checkbox that forgets itself on reopen is indistinguishable from one that never
// applied.

func seedCardPayment(t *testing.T) (*App, domain.Transaction, domain.Account) {
	t.Helper()
	a := newApp(t, false)
	chk := domain.Account{
		ID: "chk", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}
	card := domain.Account{
		ID: "card", Name: "Rewards Card", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
	}
	if err := a.PutAccount(chk); err != nil {
		t.Fatalf("PutAccount chk: %v", err)
	}
	if err := a.PutAccount(card); err != nil {
		t.Fatalf("PutAccount card: %v", err)
	}
	tx := domain.Transaction{
		ID: "pay", AccountID: chk.ID, Amount: money.New(-20000, "USD"),
		Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Desc: "Card payment",
	}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	return a, tx, card
}

func TestPaymentIntentSurvivesSaveAndReopen(t *testing.T) {
	a, tx, card := seedCardPayment(t)

	classified, err := txnclassify.Apply(tx, card.ID, true, a.Accounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := a.PutTransaction(classified); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reopen: read it back the way the edit form does.
	got, ok := legByID(a, tx.ID)
	if !ok {
		t.Fatal("transaction missing after save")
	}
	if got.TransferAccountID != card.ID {
		t.Errorf("counterparty = %q, want %q", got.TransferAccountID, card.ID)
	}
	if got.BillAccountID != card.ID {
		t.Errorf("payment intent = %q, want %q — the choice did not survive the round trip",
			got.BillAccountID, card.ID)
	}
	if txnclassify.KindOf(got, a.Accounts()) != txnclassify.KindDebt {
		t.Error("the reopened row no longer reads as a debt payment")
	}
}

// The relationship without the intent: naming a card says the money stayed in the
// household, which is a smaller claim than "this settles that debt". Both must
// persist independently.
func TestTransferWithoutPaymentIntentPersistsAsSuch(t *testing.T) {
	a, tx, card := seedCardPayment(t)

	classified, err := txnclassify.Apply(tx, card.ID, false, a.Accounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := a.PutTransaction(classified); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, _ := legByID(a, tx.ID)
	if got.TransferAccountID != card.ID {
		t.Errorf("counterparty = %q, want the card", got.TransferAccountID)
	}
	if got.BillAccountID != "" {
		t.Errorf("payment intent = %q, want empty — it was never claimed", got.BillAccountID)
	}
	// Still out of spending either way: that is the relationship's doing.
	if got.IsIncome() {
		t.Error("a transfer leg read as income")
	}
}

// Re-pointing at an asset must drop the intent, and must do so THROUGH the store,
// not just in memory — a stale BillAccountID pointing at a checking account is a
// debt payment toward something that cannot hold debt.
func TestPaymentIntentIsClearedWhenTheCounterpartyStopsBeingADebt(t *testing.T) {
	a, tx, card := seedCardPayment(t)
	sav := domain.Account{
		ID: "sav", Name: "Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
	}
	if err := a.PutAccount(sav); err != nil {
		t.Fatalf("PutAccount sav: %v", err)
	}

	paid, err := txnclassify.Apply(tx, card.ID, true, a.Accounts())
	if err != nil {
		t.Fatalf("Apply card: %v", err)
	}
	if err := a.PutTransaction(paid); err != nil {
		t.Fatalf("save card: %v", err)
	}

	moved, err := txnclassify.Apply(paid, sav.ID, false, a.Accounts())
	if err != nil {
		t.Fatalf("Apply savings: %v", err)
	}
	if err := a.PutTransaction(moved); err != nil {
		t.Fatalf("save savings: %v", err)
	}

	got, _ := legByID(a, tx.ID)
	if got.TransferAccountID != sav.ID {
		t.Errorf("counterparty = %q, want savings", got.TransferAccountID)
	}
	if got.BillAccountID != "" {
		t.Errorf("payment intent = %q — it still names a debt after moving to an asset",
			got.BillAccountID)
	}
}

// The intent is not offerable for an asset at all, and the refusal is an error
// rather than a silent drop: half-applying what the caller asked for is how two
// surfaces end up disagreeing about one row.
func TestPaymentIntentIsRefusedForAnAssetCounterparty(t *testing.T) {
	a, tx, _ := seedCardPayment(t)
	sav := domain.Account{
		ID: "sav", Name: "Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
	}
	if err := a.PutAccount(sav); err != nil {
		t.Fatalf("PutAccount sav: %v", err)
	}
	if _, err := txnclassify.Apply(tx, sav.ID, true, a.Accounts()); err == nil {
		t.Fatal("a payment toward a savings account was allowed")
	}
}
