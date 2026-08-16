// SPDX-License-Identifier: MIT

package budgeting

import "testing"

// The reported defect: a source offering its whole LIMIT under "available" while
// its own card said far less was left.
func TestSourceFundsOfMovableIsWhatIsLeftNotTheLimit(t *testing.T) {
	// Groceries: $354.85 limit, $76.56 left after spending.
	got := SourceFundsOf(usd(35485), usd(7656), usd(0))

	if got.Movable.Amount != 7656 {
		t.Errorf("Movable = %d, want 7656 — what is left, not the limit", got.Movable.Amount)
	}
	if got.Free.Amount != 7656 {
		t.Errorf("Free = %d, want 7656 with nothing committed", got.Free.Amount)
	}
	if got.HasCommitment() {
		t.Error("HasCommitment = true with no commitments")
	}
}

// The second reported case: $59.16 offered while only $20.16 was genuinely free,
// the difference being recurring charges this period has already claimed.
func TestSourceFundsOfSeparatesCommittedFromFree(t *testing.T) {
	got := SourceFundsOf(usd(20000), usd(5916), usd(3900))

	if got.Movable.Amount != 5916 {
		t.Errorf("Movable = %d, want 5916 — a commitment does not stop a deliberate move", got.Movable.Amount)
	}
	if got.Free.Amount != 2016 {
		t.Errorf("Free = %d, want 2016", got.Free.Amount)
	}
	if !got.HasCommitment() {
		t.Error("HasCommitment = false with $39.00 committed")
	}
}

// A budget with a non-positive limit fails validation, so a move must always
// leave at least one minor unit behind.
func TestSourceFundsOfKeepsTheSourceValid(t *testing.T) {
	got := SourceFundsOf(usd(10000), usd(10000), usd(0))
	if got.Movable.Amount != 9999 {
		t.Errorf("Movable = %d, want 9999 — a cent must stay so the limit remains positive", got.Movable.Amount)
	}
}

// An already-overspent source has nothing to give, and must not report a
// negative as if it were an amount.
func TestSourceFundsOfOverspentSourceGivesNothing(t *testing.T) {
	got := SourceFundsOf(usd(10000), usd(-2500), usd(0))
	if got.Movable.Amount != 0 || got.Free.Amount != 0 {
		t.Errorf("movable/free = %d/%d, want 0/0 for an overspent source", got.Movable.Amount, got.Free.Amount)
	}
	if got.CanGive() {
		t.Error("CanGive = true for an overspent source")
	}
}

// Committed can never exceed what is movable — the split has to add up, or the
// caption would claim more is spoken for than exists.
func TestSourceFundsOfClampsCommitted(t *testing.T) {
	got := SourceFundsOf(usd(10000), usd(3000), usd(9999))
	if got.Committed.Amount != got.Movable.Amount {
		t.Errorf("Committed = %d, want it clamped to Movable %d", got.Committed.Amount, got.Movable.Amount)
	}
	if got.Free.Amount != 0 {
		t.Errorf("Free = %d, want 0", got.Free.Amount)
	}
	if got.Committed.Amount+got.Free.Amount != got.Movable.Amount {
		t.Error("committed + free must equal movable")
	}
}

func TestSourceFundsAfterGiving(t *testing.T) {
	f := SourceFundsOf(usd(20000), usd(5916), usd(3900))
	if got := f.AfterGiving(2000); got.Amount != 3916 {
		t.Errorf("AfterGiving(2000) = %d, want 3916", got.Amount)
	}
	// The caller caps at Movable; a preview never shows a negative balance.
	if got := f.AfterGiving(999999); got.Amount != 0 {
		t.Errorf("AfterGiving(huge) = %d, want 0", got.Amount)
	}
}
