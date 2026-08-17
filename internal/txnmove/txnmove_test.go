// SPDX-License-Identifier: MIT

package txnmove

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var day = time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

func accts() []domain.Account {
	return []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "biz", Name: "Business Checking", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "sav", Name: "Savings", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "eur", Name: "Travel Card", Class: domain.ClassLiability, Currency: "EUR"},
	}
}

func tx(id, acct string, minor int64) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, Amount: money.New(minor, "USD"), Date: day, Desc: "Groceries",
	}
}

// The whole point: a row that landed on the wrong account can be reposted, and
// nothing about the money or the filing changes with it.
func TestMoveChangesOnlyTheAccount(t *testing.T) {
	in := tx("a", "chk", -4500)
	in.CategoryID = "groceries"
	in.Payee = "Whole Foods"

	got, _, hasPartner, err := Apply(in, "biz", accts(), nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if hasPartner {
		t.Error("a plain expense reported a transfer partner")
	}
	if got.AccountID != "biz" {
		t.Errorf("account = %q, want biz", got.AccountID)
	}
	if got.Amount != in.Amount || !got.Date.Equal(in.Date) ||
		got.CategoryID != in.CategoryID || got.Payee != in.Payee || got.ID != in.ID {
		t.Errorf("the move changed something other than the account:\n got %+v\nwant %+v", got, in)
	}
}

// Refused, not converted. A mismatch does not merely skip the row —
// ledger.bulkBalances drops the whole ACCOUNT from the balance map — so a move
// that mixed currencies would silently blank a balance.
func TestMoveToAnAccountInAnotherCurrencyIsRefused(t *testing.T) {
	p := PlanMove(tx("a", "chk", -4500), "eur", accts(), nil)
	if p.Err == nil {
		t.Fatal("a USD row was allowed onto a EUR account")
	}
	if _, _, _, err := Apply(tx("a", "chk", -4500), "eur", accts(), nil); err == nil {
		t.Fatal("Apply allowed it too — plan and apply disagree")
	}
}

// A transfer's partner names this row's account as its counterparty. Move one leg
// and the other must follow, or the pair describes a movement between accounts
// that no longer has a row on one end.
func TestMovingATransferLegCarriesItsPartnersCounterparty(t *testing.T) {
	out := tx("out", "chk", -30000)
	out.TransferAccountID = "sav"
	out.TransferGroupID = "g1"
	in := tx("in", "sav", 30000)
	in.TransferAccountID = "chk"
	in.TransferGroupID = "g1"

	moved, partner, hasPartner, err := Apply(out, "biz", accts(), []domain.Transaction{out, in})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !hasPartner {
		t.Fatal("the partner was not found, so it would have been left pointing at the old account")
	}
	if moved.AccountID != "biz" {
		t.Errorf("moved account = %q, want biz", moved.AccountID)
	}
	if partner.TransferAccountID != "biz" {
		t.Errorf("partner counterparty = %q, want biz — the pair no longer describes one movement",
			partner.TransferAccountID)
	}
	if partner.ID != "in" || partner.AccountID != "sav" {
		t.Errorf("the wrong row was treated as the partner: %+v", partner)
	}
}

// A transfer cannot have the same account on both sides.
func TestMovingALegOntoItsOwnCounterpartyIsRefused(t *testing.T) {
	out := tx("out", "chk", -30000)
	out.TransferAccountID = "sav"
	if _, _, _, err := Apply(out, "sav", accts(), nil); err == nil {
		t.Fatal("a transfer leg was allowed to move onto its own counterparty")
	}
}

// Grouped but alone: the partner is missing, and the matcher must NOT drop to
// resemblance and adopt an unrelated look-alike (the C680 rule).
func TestAGroupedOrphanDoesNotAdoptALookAlikePartner(t *testing.T) {
	out := tx("out", "chk", -30000)
	out.TransferAccountID = "sav"
	out.TransferGroupID = "gone"
	lookAlike := tx("other", "sav", 30000)
	lookAlike.TransferAccountID = "chk"
	lookAlike.TransferGroupID = "different"

	_, _, hasPartner, err := Apply(out, "biz", accts(), []domain.Transaction{out, lookAlike})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if hasPartner {
		t.Error("an unrelated row was adopted as the partner and would have been mutated")
	}
}

// Legs written before the group id existed still have to pair, or moving one
// would orphan every transfer already in the ledger.
func TestLegacyLegsStillPairByResemblance(t *testing.T) {
	out := tx("out", "chk", -30000)
	out.TransferAccountID = "sav"
	in := tx("in", "sav", 30000)
	in.TransferAccountID = "chk"

	_, partner, hasPartner, err := Apply(out, "biz", accts(), []domain.Transaction{out, in})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !hasPartner || partner.ID != "in" {
		t.Fatalf("legacy partner not found: hasPartner=%v partner=%+v", hasPartner, partner)
	}
	if partner.TransferAccountID != "biz" {
		t.Errorf("partner counterparty = %q, want biz", partner.TransferAccountID)
	}
}

// Moving a row off an account someone has reconciled changes a balance they
// signed off against a statement. Not refused — that is exactly when a
// mis-import needs correcting — but it has to be said.
func TestMovingAReconciledRowIsFlaggedButAllowed(t *testing.T) {
	list := accts()
	list[0].Reconciliations = []domain.Reconciliation{
		{StatementDate: day.AddDate(0, 0, 5)},
	}
	settled := tx("a", "chk", -4500)
	settled.Cleared = true

	p := PlanMove(settled, "biz", list, nil)
	if p.Err != nil {
		t.Fatalf("the move was refused: %v", p.Err)
	}
	if !p.BreaksReconciliation {
		t.Error("moving a cleared, reconciled row was not flagged")
	}
	// An uncleared row over the same window is not a reconciliation concern.
	if PlanMove(tx("b", "chk", -4500), "biz", list, nil).BreaksReconciliation {
		t.Error("an uncleared row was flagged as breaking reconciliation")
	}
}

// Re-choosing the account a row is already on is a person confirming, not an
// error, and it must not rewrite anything.
func TestMovingToTheSameAccountIsANoOp(t *testing.T) {
	in := tx("a", "chk", -4500)
	p := PlanMove(in, "chk", accts(), nil)
	if p.Err != nil || !p.NoOp {
		t.Fatalf("plan = %+v, want a clean no-op", p)
	}
	got, _, hasPartner, err := Apply(in, "chk", accts(), nil)
	if err != nil || hasPartner || got.AccountID != "chk" {
		t.Errorf("apply = %+v hasPartner=%v err=%v", got, hasPartner, err)
	}
}

func TestMoveToAnUnknownAccountIsRefused(t *testing.T) {
	if _, _, _, err := Apply(tx("a", "chk", -4500), "nope", accts(), nil); err == nil {
		t.Fatal("a move to a non-existent account was allowed")
	}
}

// The plan names both ends so the UI can say "Checking → Business Checking"
// rather than showing two opaque ids.
func TestPlanNamesBothAccounts(t *testing.T) {
	p := PlanMove(tx("a", "chk", -4500), "biz", accts(), nil)
	if p.FromName != "Checking" || p.ToName != "Business Checking" {
		t.Errorf("names = %q -> %q", p.FromName, p.ToName)
	}
}
