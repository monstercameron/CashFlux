// SPDX-License-Identifier: MIT

package transferpair

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// An orphan is the serious case: a row already flagged as a transfer whose far
// side is nowhere in the ledger, so one account is wrong by the moved amount.
func TestOrphanedTransferLegIsQueuedFirst(t *testing.T) {
	orphan := asTransfer(tx("o", "chk", "Transfer to Savings *6500", -50000, day(14)), "sav")
	// A healthy pair alongside it, which must NOT be queued.
	a := asTransfer(tx("a", "chk", "Transfer to account ending 1958", -75000, day(2)), "cns")
	b := asTransfer(tx("b", "cns", "Transfer from Checking *8945", 75000, day(2)), "chk")

	r := Build([]domain.Transaction{orphan, a, b}, accounts())
	if len(r.Items) != 1 {
		t.Fatalf("queued %d items, want 1: %+v", len(r.Items), r.Items)
	}
	if r.Items[0].Kind != KindOrphan || r.Items[0].Leg.ID != "o" {
		t.Errorf("item = %s/%s, want the orphan", r.Items[0].Kind, r.Items[0].Leg.ID)
	}
	if len(r.Orphans()) != 1 {
		t.Errorf("Orphans = %d, want 1", len(r.Orphans()))
	}
}

// Both legs imported as plain income and spending. Nothing says they are one
// movement, and left alone they inflate both totals — but the descriptors settle
// it, so this needs nobody.
func TestUnflaggedButVerifiedNeedsNoReview(t *testing.T) {
	out := tx("o", "chk", "Transfer to Savings *6500", -50000, day(14))
	in := tx("i", "sav", "Transfer from Checking *8945", 50000, day(14))

	r := Build([]domain.Transaction{out, in}, accounts())
	if len(r.Items) != 0 {
		t.Errorf("queued %d items, want none — the descriptors settle this: %+v", len(r.Items), r.Items)
	}
	if len(r.VerifiedUnflagged) != 1 {
		t.Fatalf("VerifiedUnflagged = %d, want 1", len(r.VerifiedUnflagged))
	}
	if r.VerifiedUnflagged[0].Leg.ID != "o" {
		t.Errorf("reported from %q, want the outgoing leg", r.VerifiedUnflagged[0].Leg.ID)
	}
}

// The same pair with no descriptor evidence is a judgement call, so it goes to a
// person rather than being acted on.
func TestUnflaggedWithoutEvidenceIsQueued(t *testing.T) {
	out := tx("o", "chk", "Online transfer", -50000, day(14))
	in := tx("i", "sav", "Online transfer", 50000, day(14))

	r := Build([]domain.Transaction{out, in}, accounts())
	if len(r.VerifiedUnflagged) != 0 {
		t.Errorf("acted without evidence: %+v", r.VerifiedUnflagged)
	}
	if len(r.Items) != 1 || r.Items[0].Kind != KindUnflagged {
		t.Fatalf("items = %+v, want one unflagged entry", r.Items)
	}
}

// One movement must produce one queue entry, not one from each end.
func TestAPairIsReportedOnceFromTheOutgoingLeg(t *testing.T) {
	out := tx("o", "chk", "Online transfer", -50000, day(14))
	in := tx("i", "sav", "Online transfer", 50000, day(14))

	r := Build([]domain.Transaction{out, in}, accounts())
	if len(r.Items) != 1 {
		t.Fatalf("items = %d, want 1 — the pair was reported from both ends", len(r.Items))
	}
	if !r.Items[0].Leg.Amount.IsNegative() {
		t.Errorf("reported from the incoming leg; the outgoing one is where the money left")
	}
}

// An ordinary purchase has no counterpart and belongs nowhere near this queue.
func TestOrdinarySpendingIsNotQueued(t *testing.T) {
	rows := []domain.Transaction{
		tx("a", "chk", "Publix", -8432, day(3)),
		tx("b", "chk", "Shell", -4210, day(4)),
		tx("c", "chk", "Payroll", 250000, day(1)),
	}
	if r := Build(rows, accounts()); len(r.Items) != 0 || len(r.VerifiedUnflagged) != 0 {
		t.Errorf("ordinary spending was queued: %+v %+v", r.Items, r.VerifiedUnflagged)
	}
}

// A refund that happens to equal a purchase on another account within the window
// is the false positive this queue would drown in. It has no descriptor evidence,
// so it must land in review rather than being auto-linked.
func TestALookAlikeIsNeverAutoLinked(t *testing.T) {
	purchase := tx("p", "chk", "Blue Bottle Coffee", -1850, day(5))
	refund := tx("r", "card", "Blue Bottle Coffee refund", 1850, day(6))

	r := Build([]domain.Transaction{purchase, refund}, accounts())
	if len(r.VerifiedUnflagged) != 0 {
		t.Errorf("a refund was auto-linked as a transfer: %+v", r.VerifiedUnflagged)
	}
}

// Ambiguity is reported as ambiguity, with the alternatives kept.
func TestAmbiguousFlaggedTransferIsQueuedWithItsAlternatives(t *testing.T) {
	out := asTransfer(tx("o", "chk", "Online transfer", -75000, day(14)), "sav")
	a := tx("a", "sav", "Online transfer", 75000, day(14))
	b := tx("b", "cns", "Online transfer", 75000, day(14))

	r := Build([]domain.Transaction{out, a, b}, accounts())
	if len(r.Items) == 0 {
		t.Fatal("nothing queued for an ambiguous transfer")
	}
	found := false
	for _, it := range r.Items {
		if it.Leg.ID == "o" && it.Kind == KindAmbiguous {
			found = true
			if len(it.Others) == 0 {
				t.Errorf("the alternative was hidden rather than offered")
			}
		}
	}
	if !found {
		t.Errorf("items = %+v, want an ambiguous entry for o", r.Items)
	}
}

// The queue is worked through top to bottom, so it must not reshuffle between
// renders when the store hands the ledger back in a different order.
func TestQueueOrderIsStable(t *testing.T) {
	rows := []domain.Transaction{
		asTransfer(tx("o1", "chk", "Transfer to Savings *6500", -50000, day(14)), "sav"),
		asTransfer(tx("o2", "chk", "Transfer to Savings *6500", -25000, day(3)), "sav"),
		tx("u1", "chk", "Online transfer", -11000, day(8)),
		tx("u2", "sav", "Online transfer", 11000, day(8)),
	}
	first := Build(rows, accounts())

	shuffled := []domain.Transaction{rows[3], rows[1], rows[2], rows[0]}
	second := Build(shuffled, accounts())

	if len(first.Items) != len(second.Items) {
		t.Fatalf("item counts differ: %d vs %d", len(first.Items), len(second.Items))
	}
	for i := range first.Items {
		if first.Items[i].Leg.ID != second.Items[i].Leg.ID {
			t.Errorf("position %d: %q vs %q — the queue reshuffled", i,
				first.Items[i].Leg.ID, second.Items[i].Leg.ID)
		}
	}
	// Orphans outrank everything else: they are the only kind that means an
	// account is currently wrong.
	if first.Items[0].Kind != KindOrphan {
		t.Errorf("first item is %s, want the orphan at the top", first.Items[0].Kind)
	}
}

// The shape of a real credit-union export: both legs of each sweep filed onto the
// SAME account, because the union exports its sub-accounts in one file. They can
// never pair — a pair needs two accounts — so they must surface as something a
// person can act on rather than silently balancing to nothing.
func TestBothLegsOnOneAccountDoNotPair(t *testing.T) {
	out := tx("o", "chk", "Transfer to Savings *6500", -50000, day(14))
	misfiled := tx("m", "chk", "Transfer from Checking *8945", 50000, day(14))

	m := For(out, []domain.Transaction{out, misfiled}, accounts())
	if m.Confidence != Unmatched {
		t.Errorf("confidence = %s, want unmatched — both rows are on one account", m.Confidence)
	}
	r := Build([]domain.Transaction{out, misfiled}, accounts())
	if len(r.VerifiedUnflagged) != 0 {
		t.Errorf("two rows on one account were treated as a movement: %+v", r.VerifiedUnflagged)
	}
}
