// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// C680: the pair has to be findable by IDENTITY, not by resemblance. Every test
// here is a case where resemblance fails and identity does not.

func TestCreateTransferPairGivesBothLegsOneGroupID(t *testing.T) {
	a, outLeg, inLeg := seedTransferPair(t)
	_ = a
	if outLeg.TransferGroupID == "" {
		t.Fatal("out leg carries no group id")
	}
	if outLeg.TransferGroupID != inLeg.TransferGroupID {
		t.Errorf("group ids differ: %q vs %q", outLeg.TransferGroupID, inLeg.TransferGroupID)
	}
}

// The C629 case, now solved by identity rather than by passing the pre-edit copy:
// an edited amount stops the legs looking reciprocal, and the group id does not care.
func TestGroupIDSurvivesAnAmountEdit(t *testing.T) {
	a, outLeg, inLeg := seedTransferPair(t)

	edited := outLeg
	edited.Amount = money.New(-12000, "USD")
	if err := a.PutTransactionWithTransferPair(outLeg, edited); err != nil {
		t.Fatalf("edit: %v", err)
	}
	gotOut, _ := legByID(a, outLeg.ID)
	gotIn, _ := legByID(a, inLeg.ID)
	if gotOut.TransferGroupID == "" || gotOut.TransferGroupID != gotIn.TransferGroupID {
		t.Errorf("group ids after edit: out=%q in=%q — the relationship was lost",
			gotOut.TransferGroupID, gotIn.TransferGroupID)
	}
	// And the amounts still mirror, which is what the id exists to keep possible.
	if gotIn.Amount.Amount != 12000 {
		t.Errorf("in leg = %d, want 12000", gotIn.Amount.Amount)
	}
}

// Two legs of one move, matched purely on the id, with amounts that share no
// resemblance at all — the cross-currency case the old matcher could never see.
func TestReciprocalMatchesOnGroupIDDespiteDifferentAmounts(t *testing.T) {
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	out := domain.Transaction{
		ID: "o", AccountID: "chk", TransferAccountID: "eur", TransferGroupID: "g1",
		Amount: money.New(-10000, "USD"), Date: day,
	}
	in := domain.Transaction{
		ID: "i", AccountID: "eur", TransferAccountID: "chk", TransferGroupID: "g1",
		Amount: money.New(9150, "EUR"), Date: day.AddDate(0, 0, 2),
	}
	if !isReciprocalTransferLeg(out, in) {
		t.Error("an FX pair sharing a group id was not recognised as reciprocal")
	}
}

// The ambiguity C680 names: two same-amount, same-day moves between the same two
// accounts. With ids, each finds its own partner and neither adopts the other's.
func TestGroupIDDisambiguatesTwoIdenticalMoves(t *testing.T) {
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id, acct, other, group string, minor int64) domain.Transaction {
		return domain.Transaction{
			ID: id, AccountID: acct, TransferAccountID: other, TransferGroupID: group,
			Amount: money.New(minor, "USD"), Date: day,
		}
	}
	outA, inA := mk("oA", "chk", "sav", "gA", -5000), mk("iA", "sav", "chk", "gA", 5000)
	outB, inB := mk("oB", "chk", "sav", "gB", -5000), mk("iB", "sav", "chk", "gB", 5000)

	if !isReciprocalTransferLeg(outA, inA) || !isReciprocalTransferLeg(outB, inB) {
		t.Fatal("a leg did not match its own partner")
	}
	if isReciprocalTransferLeg(outA, inB) || isReciprocalTransferLeg(outB, inA) {
		t.Error("a leg matched the OTHER move's partner — the ambiguity survived")
	}
}

// A grouped leg whose partner is absent must NOT fall back to resemblance and
// adopt a look-alike: "no unrelated row is auto-mutated" is the acceptance.
func TestGroupedLegDoesNotAdoptALookAlike(t *testing.T) {
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	orphan := domain.Transaction{
		ID: "orphan", AccountID: "chk", TransferAccountID: "sav", TransferGroupID: "gone",
		Amount: money.New(-5000, "USD"), Date: day,
	}
	// Reciprocal in every way the old heuristic checked, but a different move.
	lookAlike := domain.Transaction{
		ID: "other", AccountID: "sav", TransferAccountID: "chk", TransferGroupID: "different",
		Amount: money.New(5000, "USD"), Date: day,
	}
	if isReciprocalTransferLeg(orphan, lookAlike) {
		t.Error("a grouped orphan adopted an unrelated look-alike leg")
	}
}

// Rows written before the id existed still have to pair, or upgrading the app
// would orphan every transfer already in the ledger.
func TestLegacyRowsWithNoGroupIDStillPairByResemblance(t *testing.T) {
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	out := domain.Transaction{
		ID: "o", AccountID: "chk", TransferAccountID: "sav",
		Amount: money.New(-5000, "USD"), Date: day,
	}
	in := domain.Transaction{
		ID: "i", AccountID: "sav", TransferAccountID: "chk",
		Amount: money.New(5000, "USD"), Date: day,
	}
	if !isReciprocalTransferLeg(out, in) {
		t.Error("legacy ungrouped legs stopped pairing")
	}
}
