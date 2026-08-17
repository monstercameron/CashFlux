// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var pairDay = time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

func pairTxn(id, acct string, minor int64, dayOffset int) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct,
		Amount: money.New(minor, "USD"),
		Date:   pairDay.AddDate(0, 0, dayOffset),
	}
}

func pairAccounts() []domain.Account {
	return []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "sav", Name: "Savings", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "card", Name: "Rewards Card", Class: domain.ClassLiability, Currency: "USD"},
	}
}

func TestFindReciprocalExactOppositeLeg(t *testing.T) {
	out := pairTxn("out", "chk", -30000, 0)
	in := pairTxn("in", "sav", 30000, 1)
	m, ok := FindReciprocal(out, "sav", []domain.Transaction{out, in, pairTxn("noise", "sav", -500, 0)})
	if !ok || m.Txn.ID != "in" || !m.Exact {
		t.Fatalf("match = %+v ok=%v, want exact match on 'in'", m, ok)
	}
}

// A wire fee makes the two sides differ. That is still the pair, and the caller
// needs to know it is not exact rather than be told nothing was found.
func TestFindReciprocalReportsANearMatch(t *testing.T) {
	out := pairTxn("out", "chk", -30000, 0)
	in := pairTxn("in", "sav", 29850, 0)
	m, ok := FindReciprocal(out, "sav", []domain.Transaction{out, in})
	if !ok || m.Txn.ID != "in" || m.Exact {
		t.Fatalf("match = %+v ok=%v, want a NON-exact match on 'in'", m, ok)
	}
}

func TestFindReciprocalRejectsSameDirectionAndFarDates(t *testing.T) {
	out := pairTxn("out", "chk", -30000, 0)
	sameWay := pairTxn("same", "sav", -30000, 0) // both leaving: not two sides of one move
	tooFar := pairTxn("far", "sav", 30000, 30)   // a month later
	if _, ok := FindReciprocal(out, "sav", []domain.Transaction{out, sameWay, tooFar}); ok {
		t.Error("found a partner among same-direction and far-dated rows")
	}
}

// A leg already paired with a different account is spoken for.
func TestFindReciprocalSkipsAlreadyPairedLegs(t *testing.T) {
	out := pairTxn("out", "chk", -30000, 0)
	taken := pairTxn("taken", "sav", 30000, 0)
	taken.TransferAccountID = "other"
	if _, ok := FindReciprocal(out, "sav", []domain.Transaction{out, taken}); ok {
		t.Error("matched a leg already linked to another account")
	}
}

func TestFindReciprocalPrefersExactOverNearer(t *testing.T) {
	out := pairTxn("out", "chk", -30000, 0)
	near := pairTxn("near", "sav", 29000, 0)   // same day, wrong amount
	exact := pairTxn("exact", "sav", 30000, 2) // two days out, right amount
	m, ok := FindReciprocal(out, "sav", []domain.Transaction{out, near, exact})
	if !ok || m.Txn.ID != "exact" || !m.Exact {
		t.Fatalf("match = %+v, want the exact-amount row even though it is dated further away", m)
	}
}

// The preview has to state the size of the correction, because "this row will
// stop counting" is not the same information as "$300 leaves your spending".
func TestPreviewStatesWhatLeavesTheTotals(t *testing.T) {
	out := pairTxn("out", "chk", -30000, 0)
	in := pairTxn("in", "sav", 30000, 0)
	p := PreviewApply(out, "sav", false, pairAccounts(), []domain.Transaction{out, in})
	if p.Err != nil {
		t.Fatalf("preview error: %v", p.Err)
	}
	if p.Kind != KindNeutral || p.CounterpartyName != "Savings" {
		t.Errorf("kind=%q name=%q, want neutral/Savings", p.Kind, p.CounterpartyName)
	}
	if p.LeavesTotalsMinor != 30000 {
		t.Errorf("leaves = %d, want 30000", p.LeavesTotalsMinor)
	}
	if !p.HasPartner || p.Partner.Txn.ID != "in" {
		t.Errorf("partner = %+v, want 'in'", p.Partner)
	}
}

func TestPreviewOfADebtPaymentReadsAsDebt(t *testing.T) {
	pay := pairTxn("pay", "chk", -20000, 0)
	p := PreviewApply(pay, "card", true, pairAccounts(), []domain.Transaction{pay})
	if p.Err != nil {
		t.Fatalf("preview error: %v", p.Err)
	}
	if p.Kind != KindDebt {
		t.Errorf("kind = %q, want debt", p.Kind)
	}
	if p.HasPartner {
		t.Error("reported a partner for a one-sided card payment")
	}
}

// A preview must never promise a save that Apply would refuse.
func TestPreviewSurfacesTheSameRefusalApplyWould(t *testing.T) {
	pay := pairTxn("pay", "chk", -20000, 0)
	p := PreviewApply(pay, "sav", true, pairAccounts(), []domain.Transaction{pay})
	if p.Err == nil {
		t.Fatal("preview allowed a debt payment toward an asset account")
	}
	if _, err := Apply(pay, "sav", true, pairAccounts()); err == nil {
		t.Fatal("Apply allowed it too — the two disagree")
	}
}

// Re-pointing an existing transfer changes which account it names, not the
// totals: it was already out of them.
func TestPreviewOfAnAlreadyLinkedRowLeavesNothingFurther(t *testing.T) {
	linked := pairTxn("linked", "chk", -30000, 0)
	linked.TransferAccountID = "sav"
	p := PreviewApply(linked, "card", false, pairAccounts(), []domain.Transaction{linked})
	if p.LeavesTotalsMinor != 0 {
		t.Errorf("leaves = %d, want 0 — it was already a transfer", p.LeavesTotalsMinor)
	}
}
