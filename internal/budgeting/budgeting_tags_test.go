// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// taggedExpense is an expense with tags, for the cross-category tag-tracking tests.
func taggedExpense(amount int64, cat, member, day string, tags ...string) domain.Transaction {
	t := expense(amount, "USD", cat, member, day)
	t.Tags = tags
	return t
}

// TestSpentTracksTags covers cross-category tag tracking: a budget that lists TrackedTags
// counts any charge carrying one of them, in full, across categories — and never more than
// once even when several of the budget's tags overlap on the same charge.
func TestSpentTracksTags(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}

	cases := []struct {
		name   string
		budget domain.Budget
		txns   []domain.Transaction
		want   money.Money
	}{
		{
			name:   "tag matches across categories",
			budget: domain.Budget{Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TrackedTags: []string{"vacation"}, Limit: usd(100000)},
			txns: []domain.Transaction{
				taggedExpense(30000, "travel", "m1", "2026-06-03", "vacation"),   // counts (travel)
				taggedExpense(12000, "dining", "m1", "2026-06-05", "vacation"),   // counts (dining)
				taggedExpense(8000, "shopping", "m1", "2026-06-07", "household"), // no tracked tag → excluded
			},
			want: usd(42000),
		},
		{
			name:   "overlapping tags count the charge once",
			budget: domain.Budget{Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TrackedTags: []string{"vacation", "trip", "honeymoon"}, Limit: usd(100000)},
			txns: []domain.Transaction{
				// carries THREE of the budget's tracked tags — must count once, not 3×.
				taggedExpense(50000, "travel", "m1", "2026-06-03", "vacation", "trip", "honeymoon"),
			},
			want: usd(50000),
		},
		{
			name:   "duplicate tags on the budget don't double count",
			budget: domain.Budget{Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TrackedTags: []string{"Vacation", "vacation", " VACATION "}, Limit: usd(100000)},
			txns: []domain.Transaction{
				taggedExpense(25000, "travel", "m1", "2026-06-03", "vacation"),
			},
			want: usd(25000),
		},
		{
			name:   "category+tag on the same charge counts once",
			budget: domain.Budget{Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: "travel", TrackedTags: []string{"vacation"}, Limit: usd(100000)},
			txns: []domain.Transaction{
				taggedExpense(40000, "travel", "m1", "2026-06-03", "vacation"), // matches both — once
			},
			want: usd(40000),
		},
		{
			name:   "excluded-from-reports charge is ignored even with the tag",
			budget: domain.Budget{Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TrackedTags: []string{"vacation"}, Limit: usd(100000)},
			txns: []domain.Transaction{
				func() domain.Transaction {
					x := taggedExpense(15000, "travel", "m1", "2026-06-03", "vacation")
					x.ExcludeFromReports = true
					return x
				}(),
			},
			want: usd(0),
		},
		{
			name:   "individual budget only counts the owner's tagged charges",
			budget: domain.Budget{Scope: domain.ScopeIndividual, OwnerID: "m1", TrackedTags: []string{"vacation"}, Limit: usd(100000)},
			txns: []domain.Transaction{
				taggedExpense(20000, "travel", "m1", "2026-06-03", "vacation"), // counts
				taggedExpense(9000, "travel", "m2", "2026-06-04", "vacation"),  // other member → excluded
			},
			want: usd(20000),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Spent(tc.budget, tc.txns, start, end, rates)
			if err != nil {
				t.Fatalf("Spent error: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("Spent = %v, want %v", got, tc.want)
			}
		})
	}
}
