// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// seedTransferPair creates two asset accounts and one $77.00 transfer between
// them, returning the app plus both legs as they were written.
func seedTransferPair(t *testing.T) (*App, domain.Transaction, domain.Transaction) {
	t.Helper()
	a := newApp(t, false)
	from := domain.Account{
		ID: "from1", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(100000, "USD"),
	}
	to := domain.Account{
		ID: "to1", Name: "Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
		OpeningBalance: money.New(0, "USD"),
	}
	if err := a.PutAccount(from); err != nil {
		t.Fatalf("PutAccount from: %v", err)
	}
	if err := a.PutAccount(to); err != nil {
		t.Fatalf("PutAccount to: %v", err)
	}
	outID, inID, err := a.CreateTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 7700,
		Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateTransferPair: %v", err)
	}
	var outLeg, inLeg domain.Transaction
	for _, tx := range a.Transactions() {
		switch tx.ID {
		case outID:
			outLeg = tx
		case inID:
			inLeg = tx
		}
	}
	if outLeg.ID == "" || inLeg.ID == "" {
		t.Fatalf("seedTransferPair: legs not found (out=%q in=%q)", outLeg.ID, inLeg.ID)
	}
	return a, outLeg, inLeg
}

// netOf sums every transaction's minor units — for a household whose only
// activity is an internal transfer this must stay zero, because no money entered
// or left. It is the figure the transactions count line reports.
func netOf(a *App) int64 {
	var n int64
	for _, tx := range a.Transactions() {
		n += tx.Amount.Amount
	}
	return n
}

func legByID(a *App, id string) (domain.Transaction, bool) {
	for _, tx := range a.Transactions() {
		if tx.ID == id {
			return tx, true
		}
	}
	return domain.Transaction{}, false
}

// TestEditTransferLegAmountUpdatesBothLegs is the C629 regression: correcting a
// transfer's amount on one leg must move the other leg too, so the pair still
// nets to zero and both balances stay right.
func TestEditTransferLegAmountUpdatesBothLegs(t *testing.T) {
	a, outLeg, inLeg := seedTransferPair(t)
	if got := netOf(a); got != 0 {
		t.Fatalf("net before edit = %d, want 0", got)
	}

	edited := outLeg
	edited.Amount = money.New(-12000, "USD") // $77.00 -> $120.00
	if err := a.PutTransactionWithTransferPair(outLeg, edited); err != nil {
		t.Fatalf("PutTransactionWithTransferPair: %v", err)
	}

	gotOut, ok := legByID(a, outLeg.ID)
	if !ok {
		t.Fatal("out leg missing after edit")
	}
	if gotOut.Amount.Amount != -12000 {
		t.Errorf("out leg = %d, want -12000", gotOut.Amount.Amount)
	}
	gotIn, ok := legByID(a, inLeg.ID)
	if !ok {
		t.Fatal("in leg missing after edit")
	}
	if gotIn.Amount.Amount != 12000 {
		t.Errorf("in leg = %d, want 12000 (mirrored); the pair desynced", gotIn.Amount.Amount)
	}
	if got := netOf(a); got != 0 {
		t.Errorf("net after editing one leg = %d, want 0 — money appeared from nowhere", got)
	}
}

// TestEditTransferLegFromInSideMirrorsOutSide covers editing the POSITIVE leg,
// so the mirroring cannot be hard-coded to one direction.
func TestEditTransferLegFromInSideMirrorsOutSide(t *testing.T) {
	a, outLeg, inLeg := seedTransferPair(t)

	edited := inLeg
	edited.Amount = money.New(5000, "USD")
	if err := a.PutTransactionWithTransferPair(inLeg, edited); err != nil {
		t.Fatalf("PutTransactionWithTransferPair: %v", err)
	}

	gotOut, _ := legByID(a, outLeg.ID)
	if gotOut.Amount.Amount != -5000 {
		t.Errorf("out leg = %d, want -5000", gotOut.Amount.Amount)
	}
	if got := netOf(a); got != 0 {
		t.Errorf("net = %d, want 0", got)
	}
}

// TestEditTransferLegDateUpdatesBothLegs guards the other half of the pairing
// key: isReciprocalTransferLeg matches on equal dates, so a one-sided date edit
// would orphan the pair for every later operation.
func TestEditTransferLegDateUpdatesBothLegs(t *testing.T) {
	a, outLeg, inLeg := seedTransferPair(t)

	newDate := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	edited := outLeg
	edited.Date = newDate
	if err := a.PutTransactionWithTransferPair(outLeg, edited); err != nil {
		t.Fatalf("PutTransactionWithTransferPair: %v", err)
	}

	gotIn, _ := legByID(a, inLeg.ID)
	if !gotIn.Date.Equal(newDate) {
		t.Errorf("in leg date = %s, want %s", gotIn.Date, newDate)
	}
}

// TestEditTransferLegStillPairsForDeleteAfterAmountEdit proves the edit leaves
// the pair recognisable to the delete-both path, which was the second casualty
// of the one-sided write.
func TestEditTransferLegStillPairsForDeleteAfterAmountEdit(t *testing.T) {
	a, outLeg, _ := seedTransferPair(t)

	edited := outLeg
	edited.Amount = money.New(-12000, "USD")
	if err := a.PutTransactionWithTransferPair(outLeg, edited); err != nil {
		t.Fatalf("PutTransactionWithTransferPair: %v", err)
	}
	if err := a.DeleteTransactionWithTransferPair(outLeg.ID); err != nil {
		t.Fatalf("DeleteTransactionWithTransferPair: %v", err)
	}
	if n := len(a.Transactions()); n != 0 {
		t.Errorf("transactions remaining after deleting the pair = %d, want 0 (a leg was orphaned)", n)
	}
}

// TestEditNonTransferIsUnaffected keeps the common path honest: a plain expense
// edit must not go looking for a counterpart.
func TestEditNonTransferIsUnaffected(t *testing.T) {
	a := newApp(t, false)
	acc := domain.Account{
		ID: "a1", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}
	if err := a.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	tx := domain.Transaction{
		ID: "t1", AccountID: acc.ID, Amount: money.New(-2500, "USD"),
		Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Desc: "Coffee",
	}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	edited := tx
	edited.Amount = money.New(-3000, "USD")
	if err := a.PutTransactionWithTransferPair(tx, edited); err != nil {
		t.Fatalf("PutTransactionWithTransferPair: %v", err)
	}
	got, _ := legByID(a, "t1")
	if got.Amount.Amount != -3000 {
		t.Errorf("amount = %d, want -3000", got.Amount.Amount)
	}
	if n := len(a.Transactions()); n != 1 {
		t.Errorf("transaction count = %d, want 1", n)
	}
}
