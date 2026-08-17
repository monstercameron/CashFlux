// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var (
	cStart = time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	cEnd   = time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
)

// cTxn builds an in-window expense in a category.
func cTxn(id, cat string, day int, minor int64) domain.Transaction {
	return domain.Transaction{
		ID:         id,
		CategoryID: cat,
		Date:       time.Date(2026, time.August, day, 0, 0, 0, 0, time.UTC),
		Amount:     money.New(-minor, "USD"),
	}
}

func cBudget() domain.Budget {
	return domain.Budget{
		ID: "b1", CategoryID: "gro", Period: domain.PeriodMonthly,
		Limit: money.New(50000, "USD"), Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
	}
}

func TestContributingCount(t *testing.T) {
	b := cBudget()
	tests := []struct {
		name string
		txns []domain.Transaction
		want int
	}{
		{
			name: "counts in-window charges in the tracked category",
			txns: []domain.Transaction{cTxn("1", "gro", 3, 4000), cTxn("2", "gro", 9, 2500)},
			want: 2,
		},
		{
			name: "ignores other categories",
			txns: []domain.Transaction{cTxn("1", "gro", 3, 4000), cTxn("2", "fuel", 9, 2500)},
			want: 1,
		},
		{
			name: "ignores charges outside the window",
			txns: []domain.Transaction{
				cTxn("1", "gro", 3, 4000),
				{ID: "2", CategoryID: "gro", Date: time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC), Amount: money.New(-2500, "USD")},
			},
			want: 1,
		},
		{
			name: "ignores income",
			txns: []domain.Transaction{
				cTxn("1", "gro", 3, 4000),
				{ID: "2", CategoryID: "gro", Date: time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC), Amount: money.New(4000, "USD")},
			},
			want: 1,
		},
		{
			name: "a charge excluded from reports does not count (TXC-1)",
			txns: []domain.Transaction{
				cTxn("1", "gro", 3, 4000),
				func() domain.Transaction { x := cTxn("2", "gro", 5, 2500); x.ExcludeFromReports = true; return x }(),
			},
			want: 1,
		},
		{
			name: "a split receipt is ONE trip however many covered lines it has",
			txns: []domain.Transaction{{
				ID: "1", Date: time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC),
				Amount: money.New(-10000, "USD"),
				Splits: []domain.CategorySplit{
					{CategoryID: "gro", Amount: money.New(-4000, "USD")},
					{CategoryID: "gro", Amount: money.New(-3000, "USD")},
					{CategoryID: "hh", Amount: money.New(-3000, "USD")},
				},
			}},
			want: 1,
		},
		{
			name: "a split with no covered line does not count",
			txns: []domain.Transaction{{
				ID: "1", Date: time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC),
				Amount: money.New(-10000, "USD"),
				Splits: []domain.CategorySplit{
					{CategoryID: "hh", Amount: money.New(-5000, "USD")},
					{CategoryID: "fuel", Amount: money.New(-5000, "USD")},
				},
			}},
			want: 0,
		},
		{name: "no transactions", txns: nil, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ContributingCount(b, tc.txns, cStart, cEnd, nil); got != tc.want {
				t.Errorf("ContributingCount = %d, want %d", got, tc.want)
			}
		})
	}
}

// An individual budget counts only what its owner is on the hook for — the same
// per-line owner rule spentCovered applies (XC10).
func TestContributingCount_IndividualScope(t *testing.T) {
	b := cBudget()
	b.Scope, b.OwnerID = domain.ScopeIndividual, "m1"

	mine := cTxn("1", "gro", 3, 4000)
	mine.MemberID = "m1"
	theirs := cTxn("2", "gro", 4, 4000)
	theirs.MemberID = "m2"
	// A shared receipt paid by m2 with a line owned by m1 still counts for m1.
	shared := domain.Transaction{
		ID: "3", Date: time.Date(2026, time.August, 6, 0, 0, 0, 0, time.UTC),
		Amount: money.New(-9000, "USD"), MemberID: "m2",
		Splits: []domain.CategorySplit{
			{CategoryID: "gro", Amount: money.New(-4000, "USD"), MemberID: "m1"},
			{CategoryID: "gro", Amount: money.New(-5000, "USD")},
		},
	}
	if got := ContributingCount(b, []domain.Transaction{mine, theirs, shared}, cStart, cEnd, nil); got != 2 {
		t.Errorf("ContributingCount = %d, want 2 (own charge + own line of the shared receipt)", got)
	}
}

// A tracked tag counts the charge once, and takes priority over category
// matching, so a charge matching both is never counted twice.
func TestContributingCount_TrackedTag(t *testing.T) {
	b := cBudget()
	b.TrackedTags = []string{"vacation"}

	tagged := cTxn("1", "fuel", 3, 4000)
	tagged.Tags = []string{"vacation"}
	both := cTxn("2", "gro", 4, 2000)
	both.Tags = []string{"vacation"}

	if got := ContributingCount(b, []domain.Transaction{tagged, both}, cStart, cEnd, nil); got != 2 {
		t.Errorf("ContributingCount = %d, want 2", got)
	}
}

// A rollup budget's count must include the descendant categories, matching the
// predicate EvaluateRollup uses for the figure beside it.
func TestContributingCount_RollupPredicate(t *testing.T) {
	b := cBudget()
	covers := func(id string) bool { return b.TracksCategory(id) || id == "produce" }
	txns := []domain.Transaction{cTxn("1", "gro", 3, 4000), cTxn("2", "produce", 5, 2000), cTxn("3", "fuel", 6, 1000)}

	if got := ContributingCount(b, txns, cStart, cEnd, nil); got != 1 {
		t.Errorf("own-category count = %d, want 1", got)
	}
	if got := ContributingCount(b, txns, cStart, cEnd, covers); got != 2 {
		t.Errorf("rollup count = %d, want 2", got)
	}
}

// The count must agree with the figure it is displayed beside: whenever Spent is
// non-zero the count is non-zero, and vice versa.
func TestContributingCount_AgreesWithSpent(t *testing.T) {
	b := cBudget()
	rates := currency.Rates{Base: "USD"}
	cases := [][]domain.Transaction{
		nil,
		{cTxn("1", "gro", 3, 4000)},
		{cTxn("1", "fuel", 3, 4000)},
		{func() domain.Transaction { x := cTxn("1", "gro", 3, 4000); x.ExcludeFromReports = true; return x }()},
	}
	for i, txns := range cases {
		spent, err := Spent(b, txns, cStart, cEnd, rates)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		n := ContributingCount(b, txns, cStart, cEnd, nil)
		if (spent.Amount > 0) != (n > 0) {
			t.Errorf("case %d: spent %s but count %d — the two disagree", i, spent, n)
		}
	}
}
