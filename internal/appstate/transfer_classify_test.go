// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

// These tests cover what happens when an ALREADY-STORED row is reclassified as a
// transfer — the path the edit modal takes. The pure rules live in
// internal/txnclassify; what is proved here is that the write path honours them:
// nothing is invented, nothing is mirrored onto an unrelated row, and no balance
// moves.

const classifyDay = 14

func classifyDate() time.Time {
	return time.Date(2026, 7, classifyDay, 0, 0, 0, 0, time.UTC)
}

// seedClassify creates checking, savings and a positive-owed credit card, plus
// one imported expense on checking that is really a transfer.
func seedClassify(t *testing.T, a *App) (chk, sav, card domain.Account, imported domain.Transaction) {
	t.Helper()
	chk = domain.Account{
		ID: "chk", Name: "SCCU Checkings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(500000, "USD"),
	}
	sav = domain.Account{
		ID: "sav", Name: "SCCU Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
		OpeningBalance: money.New(0, "USD"),
	}
	card = domain.Account{
		ID: "card", Name: "Apple Credit Card", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
		OpeningBalance: money.New(100000, "USD"), // $1,000 owed, stored POSITIVE
	}
	for _, acc := range []domain.Account{chk, sav, card} {
		if err := a.PutAccount(acc); err != nil {
			t.Fatalf("PutAccount(%s): %v", acc.Name, err)
		}
	}
	imported = domain.Transaction{
		ID: "imp1", AccountID: chk.ID, Desc: "Transfer to Savings *6500",
		Amount: money.New(-50000, "USD"), Date: classifyDate(),
		CategoryID: "cat-transfers", Source: domain.TxnSourceImported,
	}
	if err := a.PutTransaction(imported); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	return chk, sav, card, imported
}

func txnByID(t *testing.T, a *App, id string) domain.Transaction {
	t.Helper()
	for _, x := range a.Transactions() {
		if x.ID == id {
			return x
		}
	}
	t.Fatalf("transaction %q not found", id)
	return domain.Transaction{}
}

func TestClassifyImportedRowAsTransferSaves(t *testing.T) {
	a := newApp(t, false)
	_, sav, _, imported := seedClassify(t, a)

	next, err := txnclassify.Apply(imported, sav.ID, false, a.Accounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := a.PutTransactionWithTransferPair(imported, next); err != nil {
		t.Fatalf("PutTransactionWithTransferPair: %v", err)
	}

	stored := txnByID(t, a, imported.ID)
	if !stored.IsTransfer() {
		t.Errorf("stored row is not a transfer")
	}
	if stored.TransferAccountID != sav.ID {
		t.Errorf("TransferAccountID = %q, want %q", stored.TransferAccountID, sav.ID)
	}
	if stored.IsExpense() || stored.IsIncome() {
		t.Errorf("stored row still reads as income/expense")
	}
	if !stored.Reviewed {
		t.Errorf("classifying should mark the row reviewed")
	}
}

// Classifying one row must not conjure its counterpart. The far leg of an
// imported transfer is usually another imported row waiting its turn; writing a
// third transaction would invent money.
func TestClassifyDoesNotCreateACounterpartLeg(t *testing.T) {
	a := newApp(t, false)
	_, sav, _, imported := seedClassify(t, a)
	before := len(a.Transactions())

	next, err := txnclassify.Apply(imported, sav.ID, false, a.Accounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := a.PutTransactionWithTransferPair(imported, next); err != nil {
		t.Fatalf("save: %v", err)
	}

	if got := len(a.Transactions()); got != before {
		t.Errorf("transaction count %d -> %d; classifying must not write a second row", before, got)
	}
	if bal, err := ledger.Balance(sav, a.Transactions()); err != nil {
		t.Fatalf("Balance: %v", err)
	} else if bal.Amount != 0 {
		t.Errorf("savings balance = %d, want 0 — no money arrived, only a label changed", bal.Amount)
	}
}

func TestClassifyLeavesEveryBalanceAlone(t *testing.T) {
	a := newApp(t, false)
	chk, sav, card, imported := seedClassify(t, a)
	accs := []domain.Account{chk, sav, card}

	before := map[string]int64{}
	for _, acc := range accs {
		b, err := ledger.Balance(acc, a.Transactions())
		if err != nil {
			t.Fatalf("Balance(%s): %v", acc.Name, err)
		}
		before[acc.ID] = b.Amount
	}

	next, err := txnclassify.Apply(imported, sav.ID, false, a.Accounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := a.PutTransactionWithTransferPair(imported, next); err != nil {
		t.Fatalf("save: %v", err)
	}

	for _, acc := range accs {
		b, err := ledger.Balance(acc, a.Transactions())
		if err != nil {
			t.Fatalf("Balance(%s): %v", acc.Name, err)
		}
		if b.Amount != before[acc.ID] {
			t.Errorf("%s balance moved: %d -> %d", acc.Name, before[acc.ID], b.Amount)
		}
	}
}

// A card payment imported on the CARD side already carries the sign that reduces
// a positive-owed debt. Classifying it must not re-sign it — the liability
// signing in CreateTransferPair is for legs the app writes, not ones it inherits.
func TestClassifyCardSideRowKeepsItsSignAndDebt(t *testing.T) {
	a := newApp(t, false)
	chk, _, card, _ := seedClassify(t, a)
	cardRow := domain.Transaction{
		ID: "cardpay", AccountID: card.ID,
		Desc:   "ACH deposit — Internet transfer from account ending in 8945",
		Amount: money.New(-51848, "USD"), Date: classifyDate(), Source: domain.TxnSourceImported,
	}
	if err := a.PutTransaction(cardRow); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	debtBefore, err := ledger.Balance(card, a.Transactions())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}

	next, aerr := txnclassify.Apply(cardRow, chk.ID, false, a.Accounts())
	if aerr != nil {
		t.Fatalf("Apply: %v", aerr)
	}
	if err := a.PutTransactionWithTransferPair(cardRow, next); err != nil {
		t.Fatalf("save: %v", err)
	}

	stored := txnByID(t, a, cardRow.ID)
	if stored.Amount.Amount != -51848 {
		t.Errorf("amount = %d, want -51848 unchanged", stored.Amount.Amount)
	}
	debtAfter, err := ledger.Balance(card, a.Transactions())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if debtAfter.Amount != debtBefore.Amount {
		t.Errorf("debt moved on a pure reclassification: %d -> %d", debtBefore.Amount, debtAfter.Amount)
	}
}

// Ticking "count this as a payment toward the debt" is what the debt surfaces
// read; ledger.BillPaymentForAccount is the reader.
func TestClassifyAsDebtPaymentIsVisibleToTheDebtSurface(t *testing.T) {
	a := newApp(t, false)
	_, _, card, imported := seedClassify(t, a)
	imported.Desc = "Transfer to Account Ending 1677"

	if info := ledger.BillPaymentForAccount(card.ID, a.Transactions()); info.HasAny {
		t.Fatalf("no payment should be linked before classifying")
	}

	next, err := txnclassify.Apply(imported, card.ID, true, a.Accounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := a.PutTransactionWithTransferPair(imported, next); err != nil {
		t.Fatalf("save: %v", err)
	}

	info := ledger.BillPaymentForAccount(card.ID, a.Transactions())
	if !info.HasAny {
		t.Fatalf("the debt surface cannot see the payment")
	}
	if info.LatestTxn != imported.ID {
		t.Errorf("linked payment = %q, want %q", info.LatestTxn, imported.ID)
	}
	if info.Latest.Amount != 50000 {
		t.Errorf("payment amount = %d, want 50000 as a magnitude", info.Latest.Amount)
	}
}

// Un-classifying returns the row to plain spending and drops the payment link
// with it, so the debt surface stops claiming a payment that is no longer one.
func TestUnclassifyingRestoresPlainSpending(t *testing.T) {
	a := newApp(t, false)
	_, _, card, imported := seedClassify(t, a)

	next, err := txnclassify.Apply(imported, card.ID, true, a.Accounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := a.PutTransactionWithTransferPair(imported, next); err != nil {
		t.Fatalf("save: %v", err)
	}

	stored := txnByID(t, a, imported.ID)
	back, err := txnclassify.Apply(stored, "", false, a.Accounts())
	if err != nil {
		t.Fatalf("Apply clear: %v", err)
	}
	if err := a.PutTransactionWithTransferPair(stored, back); err != nil {
		t.Fatalf("save: %v", err)
	}

	final := txnByID(t, a, imported.ID)
	if final.IsTransfer() {
		t.Errorf("row is still a transfer")
	}
	if !final.IsExpense() {
		t.Errorf("row should read as spending again")
	}
	if info := ledger.BillPaymentForAccount(card.ID, a.Transactions()); info.HasAny {
		t.Errorf("the debt surface still claims a payment that was un-classified")
	}
}

// Classifying and correcting the amount in the SAME save must not mirror the new
// amount onto an unrelated row. The pre-edit copy is a plain expense, so nothing
// can look like its counterpart — but the guard is worth pinning, because the
// mirroring path keys on the pre-edit values.
func TestClassifyAndEditAmountTogetherTouchesOnlyThisRow(t *testing.T) {
	a := newApp(t, false)
	chk, sav, _, imported := seedClassify(t, a)

	// A decoy that WOULD look reciprocal if the mirroring path misfired: same
	// date, exactly opposite amount, already pointing back at checking.
	decoy := domain.Transaction{
		ID: "decoy", AccountID: sav.ID, TransferAccountID: chk.ID,
		Desc: "Unrelated savings row", Amount: money.New(50000, "USD"), Date: classifyDate(),
	}
	if err := a.PutTransaction(decoy); err != nil {
		t.Fatalf("PutTransaction decoy: %v", err)
	}

	next, err := txnclassify.Apply(imported, sav.ID, false, a.Accounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	next.Amount = money.New(-60000, "USD") // the user also corrects the amount
	if err := a.PutTransactionWithTransferPair(imported, next); err != nil {
		t.Fatalf("save: %v", err)
	}

	if got := txnByID(t, a, imported.ID).Amount.Amount; got != -60000 {
		t.Errorf("edited row amount = %d, want -60000", got)
	}
	if got := txnByID(t, a, decoy.ID).Amount.Amount; got != 50000 {
		t.Errorf("decoy amount = %d, want 50000 untouched", got)
	}
}

// Once BOTH legs of a real pair are classified they become a genuine transfer
// pair, so deleting one takes the other with it. That is the existing pair
// behaviour, reached from the classify path.
func TestClassifyingBothLegsMakesDeleteTakeThePair(t *testing.T) {
	a := newApp(t, false)
	chk, sav, _, outLeg := seedClassify(t, a)
	inLeg := domain.Transaction{
		ID: "imp2", AccountID: sav.ID, Desc: "Transfer from Checking *8945",
		Amount: money.New(50000, "USD"), Date: classifyDate(), Source: domain.TxnSourceImported,
	}
	if err := a.PutTransaction(inLeg); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	for _, pair := range []struct {
		txn domain.Transaction
		to  string
	}{{outLeg, sav.ID}, {inLeg, chk.ID}} {
		next, err := txnclassify.Apply(pair.txn, pair.to, false, a.Accounts())
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if err := a.PutTransactionWithTransferPair(pair.txn, next); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	if err := a.DeleteTransactionWithTransferPair(outLeg.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := len(a.Transactions()); n != 0 {
		t.Errorf("%d rows survived; classifying both legs should make them one pair", n)
	}
}

// Both legs of a same-account misfiling do NOT become a pair — they sit on one
// account and both point away from it — so classifying them silences the phantom
// income without letting a delete take an unrelated row with it.
func TestMisfiledSameAccountLegsDoNotPairOnDelete(t *testing.T) {
	a := newApp(t, false)
	_, sav, _, outLeg := seedClassify(t, a)
	misfiled := domain.Transaction{
		ID: "imp3", AccountID: outLeg.AccountID, // misfiled: really the savings side
		Desc: "Transfer from Checking *8945", Amount: money.New(50000, "USD"),
		Date: classifyDate(), Source: domain.TxnSourceImported,
	}
	if err := a.PutTransaction(misfiled); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	for _, x := range []domain.Transaction{outLeg, misfiled} {
		next, err := txnclassify.Apply(x, sav.ID, false, a.Accounts())
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if err := a.PutTransactionWithTransferPair(x, next); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	if err := a.DeleteTransactionWithTransferPair(outLeg.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := len(a.Transactions()); n != 1 {
		t.Errorf("%d rows left, want 1 — two legs on ONE account are not a pair", n)
	}
}

func TestClassifyRejectsItsOwnAccountThroughTheStore(t *testing.T) {
	a := newApp(t, false)
	_, _, _, imported := seedClassify(t, a)
	if _, err := txnclassify.Apply(imported, imported.AccountID, false, a.Accounts()); err == nil {
		t.Fatalf("want an error classifying a row against its own account")
	}
	// And the store agrees, so neither layer can be bypassed by the other.
	bad := imported
	bad.TransferAccountID = imported.AccountID
	if err := a.PutTransaction(bad); err == nil {
		t.Errorf("the store accepted a transfer to its own account")
	}
}
