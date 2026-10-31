// SPDX-License-Identifier: MIT

package appstate

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTCS2AtomicEditMoveAndAmount(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "invalid-move"}[invalid], func(t *testing.T) {
			a, out, in := seedTransferPair(t)
			acc := a.Accounts()[0]
			acc.ID = "new-account"
			if err := a.PutAccount(acc); err != nil {
				t.Fatal(err)
			}
			draft := out
			draft.Amount = money.New(-15000, "USD")
			draft.AccountID = acc.ID
			if invalid {
				draft.AccountID = "missing"
			}
			notices := 0
			a.OnTxnMutated(func() { notices++ })
			err := a.EditTransaction(out, draft)
			current, _ := a.TransactionByID(out.ID)
			partner, _ := a.TransactionByID(in.ID)
			if invalid {
				if err == nil || !reflect.DeepEqual(current, out) || !reflect.DeepEqual(partner, in) || notices != 0 {
					t.Fatalf("partial rejected edit: err=%v row=%+v partner=%+v notifications=%d", err, current, partner, notices)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if current.AccountID != acc.ID || current.Amount.Amount != -15000 || partner.TransferAccountID != acc.ID || partner.Amount.Amount != 15000 || notices != 1 {
					t.Fatalf("inconsistent edit: row=%+v partner=%+v notifications=%d", current, partner, notices)
				}
				if err := a.EditTransaction(out, draft); err == nil {
					t.Fatal("stale prior overwrote an edit")
				}
			}
		})
	}
}

func TestTCS2BatchValidationIsAtomic(t *testing.T) {
	a, out, in := seedTransferPair(t)
	out.Desc = "changed"
	in.AccountID = "missing"
	if err := a.PutTransactionsAtomic([]domain.Transaction{out, in}); err == nil {
		t.Fatal("accepted invalid second row")
	}
	got, _ := a.TransactionByID(out.ID)
	if got.Desc == "changed" {
		t.Fatal("first row survived rejected batch")
	}
}

func TestTCS2DeleteRestoreCarriesRelationships(t *testing.T) {
	a, out, in := seedTransferPair(t)
	extra := out
	extra.ID = "extra"
	extra.TransferAccountID = ""
	extra.TransferGroupID = ""
	if err := a.Store().PutTransaction(extra); err != nil {
		t.Fatal(err)
	}
	group := domain.TxnLink{ID: "group", Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{out.ID, in.ID, extra.ID}}
	refund := domain.TxnLink{ID: "refund", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{out.ID, extra.ID}, Amount: money.New(100, "USD")}
	for _, link := range []domain.TxnLink{group, refund} {
		if err := a.Store().PutTxnLink(link); err != nil {
			t.Fatal(err)
		}
	}
	debt := domain.DebtPaymentLink{ID: "debt", TransactionIDs: []string{out.ID, in.ID}, DebtAccountID: "to1", Currency: "USD", PrincipalMinor: 7700}
	if err := a.Store().PutDebtPaymentLink(debt); err != nil {
		t.Fatal(err)
	}
	snapshots := []domain.Transaction{out, in}
	n := 0
	a.OnTxnMutated(func() { n++ })
	deleted, err := a.DeleteTransactionsBulk([]string{out.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 || len(a.TxnLinks()) != 0 || len(a.DebtPaymentLinks()) != 0 || n != 1 {
		t.Fatalf("incomplete delete: ids=%v links=%v debt=%v n=%d", deleted, a.TxnLinks(), a.DebtPaymentLinks(), n)
	}
	if err := a.RestoreTransactions(snapshots); err != nil {
		t.Fatal(err)
	}
	if len(a.TxnLinks()) != 2 || !reflect.DeepEqual(a.DebtPaymentLinks(), []domain.DebtPaymentLink{debt}) || n != 2 {
		t.Fatalf("incomplete restore: links=%v debt=%v n=%d", a.TxnLinks(), a.DebtPaymentLinks(), n)
	}
	// Membership indexes must be restored too, not just the allocation document.
	if err := a.DeleteTransaction(in.ID); err != nil {
		t.Fatal(err)
	}
	if len(a.DebtPaymentLinks()) != 0 {
		t.Fatal("debt membership index was lost on restore")
	}
}

func TestTCS2UnclassifyRequiresNamedPartnerRemoval(t *testing.T) {
	a, out, in := seedTransferPair(t)
	draft := out
	draft.TransferAccountID = ""
	draft.TransferGroupID = ""
	if err := a.EditTransaction(out, draft); err == nil {
		t.Fatal("silently converted the other leg into income")
	}
	if err := a.EditTransactionRemovingPartner(out, draft); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.TransactionByID(in.ID); ok {
		t.Fatal("confirmed removal left other leg as income")
	}
	got, _ := a.TransactionByID(out.ID)
	if got.IsTransfer() {
		t.Fatal("opened row was not unclassified")
	}
}

func TestTCS2LegacyIdenticalPairsConsumeDistinctLegs(t *testing.T) {
	a, out, in := seedTransferPair(t)
	out.TransferGroupID = ""
	in.TransferGroupID = ""
	out.ID = "o1"
	in.ID = "i1"
	out.Date = time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	in.Date = out.Date
	secondOut, secondIn := out, in
	secondOut.ID = "o2"
	secondIn.ID = "i2"
	all := []domain.Transaction{out, in, secondOut, secondIn}
	for _, ids := range [][]string{{"o1", "o2"}, {"i1", "i2"}, {"o1", "i1", "o2"}} {
		expanded := expandTransferPairs(all, ids, a.transactionPairRates())
		found := map[string]bool{}
		for _, id := range expanded {
			found[id] = true
		}
		if len(found) != 4 || len(expanded) != 4 {
			t.Fatalf("%v expanded to %v", ids, expanded)
		}
	}
}

func TestTCS2ExpandPositiveOwedLegacyPayments(t *testing.T) {
	a, out, in := seedTransferPair(t)
	account, _ := domain.AccountByID(a.Accounts(), in.AccountID)
	account.Class = domain.ClassLiability
	account.Type = domain.TypeCreditCard
	account.LiabilitySign = domain.LiabilityPositiveOwed
	if err := a.Store().PutAccount(account); err != nil {
		t.Fatal(err)
	}
	out.TransferGroupID = ""
	in.TransferGroupID = ""
	in.Amount.Amount = -in.Amount.Amount
	if err := a.Store().PutTransactionsAtomic([]domain.Transaction{out, in}); err != nil {
		t.Fatal(err)
	}
	ids := a.ExpandTransferPairs([]string{out.ID})
	if len(ids) != 2 {
		t.Fatalf("both-negative payment expansion=%v", ids)
	}
}

func TestTCS2RestoreRefusesInterveningRelationshipEdit(t *testing.T) {
	a, out, in := seedTransferPair(t)
	extra := out
	extra.ID = "third"
	extra.TransferAccountID = ""
	extra.TransferGroupID = ""
	if err := a.Store().PutTransaction(extra); err != nil {
		t.Fatal(err)
	}
	group := domain.TxnLink{ID: "order", Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{out.ID, in.ID, extra.ID}}
	if err := a.Store().PutTxnLink(group); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteTransaction(out.ID); err != nil {
		t.Fatal(err)
	}
	live := a.TxnLinks()[0]
	live.Note = "later decision"
	if err := a.Store().PutTxnLink(live); err != nil {
		t.Fatal(err)
	}
	if err := a.RestoreTransactions([]domain.Transaction{out}); err == nil {
		t.Fatal("silently overwrote intervening group edit")
	}
	if _, ok := a.TransactionByID(out.ID); ok {
		t.Fatal("row restored despite failed relationship restore")
	}
}

func TestTCS2ReceiptAndReferenceAreAtomic(t *testing.T) {
	a, out, _ := seedTransferPair(t)
	art := domain.Artifact{ID: "receipt", Name: "Receipt", Kind: "image", MIME: "image/png", Bytes: []byte{1, 2, 3}, Size: 3}
	if err := a.AttachTransactionArtifact("missing", art); err == nil {
		t.Fatal("attached to a missing transaction")
	}
	if _, ok, _ := a.Store().GetArtifact(art.ID); ok {
		t.Fatal("orphan receipt survived rejected attachment")
	}
	if err := a.AttachTransactionArtifact(out.ID, art); err != nil {
		t.Fatal(err)
	}
	row, _ := a.TransactionByID(out.ID)
	if len(row.Attachments) != 1 || row.Attachments[0].ArtifactID != art.ID {
		t.Fatal("receipt reference missing")
	}
	if err := a.AttachTransactionArtifact(out.ID, art); err == nil {
		t.Fatal("overwrote an existing receipt id")
	}
	row, _ = a.TransactionByID(out.ID)
	if len(row.Attachments) != 1 {
		t.Fatal("duplicate reference survived rejected receipt")
	}
}

func TestTCS2EditMoveUsesAccountBookingConvention(t *testing.T) {
	a, out, in := seedTransferPair(t)
	debt := domain.Account{ID: "move-debt", Name: "Card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", LiabilitySign: domain.LiabilityPositiveOwed}
	if err := a.PutAccount(debt); err != nil {
		t.Fatal(err)
	}
	draft := out
	draft.AccountID = debt.ID
	draft.Amount = money.New(-15000, "USD")
	if err := a.EditTransaction(out, draft); err != nil {
		t.Fatal(err)
	}
	moved, _ := a.TransactionByID(out.ID)
	partner, _ := a.TransactionByID(in.ID)
	if moved.Amount.Amount != 15000 || partner.Amount.Amount != 15000 || partner.TransferAccountID != debt.ID {
		t.Fatalf("booking conventions lost: moved=%+v partner=%+v", moved, partner)
	}
}
