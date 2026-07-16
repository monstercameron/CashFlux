// SPDX-License-Identifier: MIT

package reviewqueue

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func dtu(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func txn(id, cat string, tags []string, on time.Time) domain.Transaction {
	return domain.Transaction{ID: id, CategoryID: cat, Tags: tags, Amount: money.New(-1000, "USD"), Date: on}
}

func TestNeedsAndReason(t *testing.T) {
	cases := []struct {
		name string
		t    domain.Transaction
		want bool
		why  Reason
	}{
		{"uncategorized", txn("a", "", nil, dtu(2026, 6, 1)), true, ReasonUncategorized},
		{"flagged but categorized", txn("b", "food", []string{ReviewTag}, dtu(2026, 6, 1)), true, ReasonFlagged},
		{"clean categorized", txn("c", "food", nil, dtu(2026, 6, 1)), false, ReasonFlagged},
		{"uncategorized outranks flag", txn("d", "", []string{ReviewTag}, dtu(2026, 6, 1)), true, ReasonUncategorized},
	}
	for _, tc := range cases {
		if got := Needs(tc.t); got != tc.want {
			t.Errorf("%s: Needs = %v, want %v", tc.name, got, tc.want)
		}
		if tc.want {
			if got := ReasonFor(tc.t); got != tc.why {
				t.Errorf("%s: ReasonFor = %v, want %v", tc.name, got, tc.why)
			}
		}
	}
}

func TestTransfersNeverQueued(t *testing.T) {
	// An uncategorized transfer, even one tagged for review, must not appear.
	tr := domain.Transaction{ID: "t1", TransferAccountID: "acct-2", Tags: []string{ReviewTag},
		Amount: money.New(-5000, "USD"), Date: dtu(2026, 6, 2)}
	if Needs(tr) {
		t.Error("Needs(transfer) = true, want false (transfers are never queued)")
	}
	if Count([]domain.Transaction{tr}) != 0 {
		t.Error("Count included a transfer")
	}
}

func TestQueueOrderAndCount(t *testing.T) {
	txns := []domain.Transaction{
		txn("z", "", nil, dtu(2026, 6, 5)),          // newest
		txn("m", "food", nil, dtu(2026, 6, 9)),      // clean — excluded
		txn("a", "", []string{ReviewTag}, dtu(2026, 6, 1)), // oldest
		txn("b", "", nil, dtu(2026, 6, 1)),          // same date as a → id tie-break
		{ID: "x", TransferAccountID: "y", Amount: money.New(-100, "USD"), Date: dtu(2026, 6, 8)}, // transfer — excluded
	}
	if got := Count(txns); got != 3 {
		t.Fatalf("Count = %d, want 3", got)
	}
	q := Queue(txns)
	gotIDs := []string{}
	for _, t := range q {
		gotIDs = append(gotIDs, t.ID)
	}
	want := []string{"z", "a", "b"} // newest first; a<b on the shared Jun-1 date
	if len(gotIDs) != len(want) {
		t.Fatalf("Queue ids = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("Queue order = %v, want %v", gotIDs, want)
		}
	}
}
