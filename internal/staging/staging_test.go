// SPDX-License-Identifier: MIT

package staging

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func on(day int) time.Time { return time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC) }

func in(day int, desc string, minor int64) Input {
	return Input{Date: on(day), Description: desc, AmountMinor: minor, HasAmount: true}
}

func existing(id, acct, desc string, minor int64, day int) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: acct, Desc: desc,
		Amount: money.New(minor, "USD"), Date: on(day)}
}

func TestNewRowsAreApproved(t *testing.T) {
	b := Stage([]Input{
		in(3, "Publix", -8432),
		in(4, "Shell Oil", -4210),
	}, nil, "chk", "USD")

	if b.Counts.New != 2 || b.Counts.Total != 2 {
		t.Fatalf("counts = %+v, want two new", b.Counts)
	}
	if len(b.Approved()) != 2 {
		t.Errorf("approved = %d, want 2", len(b.Approved()))
	}
	if !b.Clean() {
		t.Errorf("Clean = false on a readable file")
	}
	for _, r := range b.Rows {
		if r.Hash == "" {
			t.Errorf("row %d has no hash", r.Index)
		}
		if r.AccountID != "chk" {
			t.Errorf("row %d landed on %q, want the chosen account", r.Index, r.AccountID)
		}
	}
}

// A row the ledger already holds is skipped, and says which row it matched — a
// count alone leaves a person unable to check the claim.
func TestARowAlreadyInTheLedgerIsADuplicate(t *testing.T) {
	have := []domain.Transaction{existing("t1", "chk", "Publix", -8432, 3)}
	b := Stage([]Input{in(3, "Publix", -8432), in(4, "Shell Oil", -4210)}, have, "chk", "USD")

	if b.Counts.Duplicate != 1 || b.Counts.New != 1 {
		t.Fatalf("counts = %+v, want one of each", b.Counts)
	}
	if b.Rows[0].Verdict != Duplicate {
		t.Errorf("row 0 = %s, want duplicate", b.Rows[0].Verdict)
	}
	if b.Rows[0].MatchID != "t1" {
		t.Errorf("MatchID = %q, want the row it matched", b.Rows[0].MatchID)
	}
	if !b.Clean() {
		t.Errorf("Clean = false — a duplicate is the safeguard working, not a fault")
	}
}

// The same charge on a DIFFERENT account is a different payment. Staging must use
// the app's one duplicate rule, not a looser one of its own (C688).
func TestTheSameChargeOnAnotherAccountIsNotADuplicate(t *testing.T) {
	have := []domain.Transaction{existing("t1", "business-card", "OpenRouter", -2000, 3)}
	b := Stage([]Input{in(3, "OpenRouter", -2000)}, have, "personal-card", "USD")

	if b.Counts.Duplicate != 0 || b.Counts.New != 1 {
		t.Errorf("counts = %+v, want the other account's charge treated as new", b.Counts)
	}
}

// A statement listing the same charge twice is far more often a parsing artefact
// than two identical charges, so the first imports and the rest are held.
func TestARepeatedRowInOneFileIsHeld(t *testing.T) {
	b := Stage([]Input{
		in(3, "Publix", -8432),
		in(3, "Publix", -8432),
		in(3, "Publix", -8432),
	}, nil, "chk", "USD")

	if b.Counts.New != 1 {
		t.Errorf("New = %d, want the first copy only", b.Counts.New)
	}
	if b.Counts.RepeatedInFile != 2 {
		t.Errorf("RepeatedInFile = %d, want 2", b.Counts.RepeatedInFile)
	}
	if b.Rows[0].Verdict != New || b.Rows[1].Verdict != RepeatedInFile {
		t.Errorf("verdicts = %s/%s, want the first kept", b.Rows[0].Verdict, b.Rows[1].Verdict)
	}
}

// Writing a transaction to the wrong account is not something a person can fix by
// looking at it later, so a row with no account is refused rather than guessed at.
func TestARowWithNoAccountIsUnusable(t *testing.T) {
	b := Stage([]Input{in(3, "Publix", -8432)}, nil, "", "USD")
	if b.Counts.Unusable != 1 {
		t.Fatalf("counts = %+v, want the row refused", b.Counts)
	}
	if b.Rows[0].Reason != "no account" {
		t.Errorf("Reason = %q, want it named", b.Rows[0].Reason)
	}
	if b.Clean() {
		t.Errorf("Clean = true with an unreadable row")
	}
	if b.Rows[0].Hash != "" {
		t.Errorf("an unusable row was hashed; there is nothing to hash it as")
	}
}

func TestMissingDateOrAmountIsUnusable(t *testing.T) {
	b := Stage([]Input{
		{Description: "no date", AmountMinor: -100, HasAmount: true},
		{Date: on(3), Description: "no amount"},
	}, nil, "chk", "USD")

	if b.Counts.Unusable != 2 {
		t.Fatalf("counts = %+v, want both refused", b.Counts)
	}
	if b.Rows[0].Reason != "no date" || b.Rows[1].Reason != "no amount" {
		t.Errorf("reasons = %q/%q", b.Rows[0].Reason, b.Rows[1].Reason)
	}
}

// A zero amount is a real amount and must not read as a missing one.
func TestAZeroAmountIsNotAMissingAmount(t *testing.T) {
	b := Stage([]Input{{Date: on(3), Description: "Fee waived", AmountMinor: 0, HasAmount: true}},
		nil, "chk", "USD")
	if b.Counts.Unusable != 0 {
		t.Errorf("a zero amount was refused: %+v", b.Rows[0])
	}
}

// Every row is kept in the batch, including the skipped ones — a review table
// that hides them cannot be checked against the file it came from.
func TestSkippedRowsStayInTheBatch(t *testing.T) {
	have := []domain.Transaction{existing("t1", "chk", "Publix", -8432, 3)}
	b := Stage([]Input{
		in(3, "Publix", -8432),
		{Description: "broken"},
		in(4, "Shell Oil", -4210),
	}, have, "chk", "USD")

	if len(b.Rows) != 3 {
		t.Fatalf("rows = %d, want every line kept", len(b.Rows))
	}
	if b.Rows[0].Index != 0 || b.Rows[2].Index != 2 {
		t.Errorf("indexes do not point back at the file lines")
	}
}

// The same file staged twice must produce the same hashes, or nothing built on
// them can be compared across runs.
func TestHashingIsStableAndDistinguishing(t *testing.T) {
	a := Hash("chk", on(3), -8432, "publix")
	if a != Hash("chk", on(3), -8432, "publix") {
		t.Errorf("the same row hashed differently twice")
	}
	for _, other := range []string{
		Hash("sav", on(3), -8432, "publix"),
		Hash("chk", on(4), -8432, "publix"),
		Hash("chk", on(3), -8433, "publix"),
		Hash("chk", on(3), -8432, "shell"),
	} {
		if other == a {
			t.Errorf("two different rows share a hash")
		}
	}
}

// The classic concatenated-key collision: "AB"+"C" must not equal "A"+"BC".
func TestHashPartsCannotRunTogether(t *testing.T) {
	if Hash("ab", on(3), -1, "c") == Hash("a", on(3), -1, "bc") {
		t.Errorf("hash parts ran together, so two different rows collide")
	}
}

// The merchant is normalized so trivial spacing and case differences do not make
// one charge look like two.
func TestNormalizeCollapsesTrivialDifferences(t *testing.T) {
	if Normalize("AMZN  Mktp  US") != Normalize("amzn mktp us") {
		t.Errorf("%q vs %q", Normalize("AMZN  Mktp  US"), Normalize("amzn mktp us"))
	}
	if Normalize("  ") != "" {
		t.Errorf("blank descriptor = %q, want empty", Normalize("  "))
	}
}

// Collisions surface the file disagreeing with itself, listed rather than
// counted, so a person can look at the rows.
func TestCollisionsListTheRowsRatherThanCountingThem(t *testing.T) {
	b := Stage([]Input{
		in(3, "Publix", -8432),
		in(3, "Publix", -8432),
		in(4, "Shell Oil", -4210),
	}, nil, "chk", "USD")

	got := Collisions(b)
	if len(got) != 1 {
		t.Fatalf("collisions = %d groups, want 1", len(got))
	}
	for _, rows := range got {
		if len(rows) != 2 {
			t.Errorf("group has %d rows, want 2", len(rows))
		}
		if rows[0].Index != 0 || rows[1].Index != 1 {
			t.Errorf("rows are not in file order: %d,%d", rows[0].Index, rows[1].Index)
		}
	}
}

func TestAnEmptyFileStagesCleanly(t *testing.T) {
	b := Stage(nil, nil, "chk", "USD")
	if b.Counts.Total != 0 || len(b.Approved()) != 0 || !b.Clean() {
		t.Errorf("empty batch = %+v", b.Counts)
	}
	if got := Collisions(b); len(got) != 0 {
		t.Errorf("collisions on an empty batch: %+v", got)
	}
}
