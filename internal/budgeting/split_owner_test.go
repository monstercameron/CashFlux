// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestSpentSplitLineOwner verifies XC10: a split line's own owner (MemberID)
// decides which individual budget it counts against, overriding the
// transaction's payer for that line only. A line with no owner falls back to the
// payer, as before.
func TestSpentSplitLineOwner(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}

	// A shared Costco run paid by member A: one household-groceries line owned by
	// A (no explicit owner → falls back to payer A), and one hobby line owned by B.
	shared := domain.Transaction{
		CategoryID: "food",
		MemberID:   "A",
		Amount:     usd(-10000),
		Date:       mustDate("2026-06-10"),
		Splits: []domain.CategorySplit{
			{CategoryID: "food", Amount: usd(-6000)},                // no owner → payer A
			{CategoryID: "food", Amount: usd(-4000), MemberID: "B"}, // owned by B
		},
	}
	all := []domain.Transaction{shared}

	budgetA := domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "A", Limit: usd(50000)}
	budgetB := domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "B", Limit: usd(50000)}

	gotA, err := Spent(budgetA, all, start, end, rates)
	if err != nil {
		t.Fatalf("Spent(A): %v", err)
	}
	if gotA.Amount != 6000 {
		t.Errorf("A's individual budget: got %d, want 6000 (only the payer-owned line)", gotA.Amount)
	}

	gotB, err := Spent(budgetB, all, start, end, rates)
	if err != nil {
		t.Fatalf("Spent(B): %v", err)
	}
	if gotB.Amount != 4000 {
		t.Errorf("B's individual budget: got %d, want 4000 (the line B owns, on A's card)", gotB.Amount)
	}
}

// TestSpentSplitOwnerFallbackAndShared checks the fallback and shared-scope cases
// in a table: an empty line owner accrues to the payer, and a shared/household
// budget counts every line regardless of per-line owner.
func TestSpentSplitOwnerFallbackAndShared(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}

	txn := domain.Transaction{
		CategoryID: "food",
		MemberID:   "A",
		Amount:     usd(-9000),
		Date:       mustDate("2026-06-12"),
		Splits: []domain.CategorySplit{
			{CategoryID: "food", Amount: usd(-5000)},                // no owner → A
			{CategoryID: "food", Amount: usd(-4000), MemberID: "B"}, // B
		},
	}
	all := []domain.Transaction{txn}

	cases := []struct {
		name   string
		budget domain.Budget
		want   int64
	}{
		{
			name:   "individual A gets the fallback line",
			budget: domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "A", Limit: usd(50000)},
			want:   5000,
		},
		{
			name:   "individual B gets its owned line",
			budget: domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "B", Limit: usd(50000)},
			want:   4000,
		},
		{
			name:   "shared budget counts every line",
			budget: domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)},
			want:   9000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Spent(tc.budget, all, start, end, rates)
			if err != nil {
				t.Fatalf("Spent: %v", err)
			}
			if got.Amount != tc.want {
				t.Errorf("got %d, want %d", got.Amount, tc.want)
			}
		})
	}
}
