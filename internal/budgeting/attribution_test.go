// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAttributeByMember(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)

	budget := domain.Budget{ID: "b1", Scope: domain.ScopeShared, CategoryID: "dining", Limit: money.New(50000, "USD")}

	txns := []domain.Transaction{
		{ID: "1", Date: mid, CategoryID: "dining", Amount: money.New(-18000, "USD"), MemberID: "you"},
		{ID: "2", Date: mid, CategoryID: "dining", Amount: money.New(-14000, "USD"), MemberID: "priya"},
		{ID: "3", Date: mid, CategoryID: "dining", Amount: money.New(-2000, "USD")},                     // unassigned
		{ID: "4", Date: before, CategoryID: "dining", Amount: money.New(-9000, "USD"), MemberID: "you"}, // out of period
		{ID: "5", Date: mid, CategoryID: "groceries", Amount: money.New(-5000, "USD"), MemberID: "you"}, // wrong category
	}

	shares, err := AttributeByMember(budget, txns, start, end, rates, nil)
	if err != nil {
		t.Fatalf("attribute: %v", err)
	}
	if len(shares) != 3 {
		t.Fatalf("want 3 shares, got %d: %+v", len(shares), shares)
	}
	// Sorted largest first: you 180, priya 140, unassigned 20.
	if shares[0].MemberID != "you" || shares[0].Spent.Amount != 18000 {
		t.Errorf("share[0] = %+v", shares[0])
	}
	if shares[1].MemberID != "priya" || shares[1].Spent.Amount != 14000 {
		t.Errorf("share[1] = %+v", shares[1])
	}
	if shares[2].MemberID != UnassignedMemberID || shares[2].Spent.Amount != 2000 {
		t.Errorf("share[2] = %+v", shares[2])
	}

	// Shares sum to the evaluated budget spend.
	st, err := Evaluate(budget, txns, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	var sum int64
	for _, s := range shares {
		sum += s.Spent.Amount
	}
	if sum != st.Spent.Amount {
		t.Errorf("shares sum %d != budget spent %d", sum, st.Spent.Amount)
	}
}

func TestAttributeByMemberSplitLineOwner(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	budget := domain.Budget{ID: "b1", Scope: domain.ScopeShared, CategoryID: "shopping", Limit: money.New(50000, "USD")}

	// One charge paid by "you", split: household line (falls back to payer) + a line
	// explicitly owned by "priya".
	txns := []domain.Transaction{
		{
			ID: "1", Date: mid, Amount: money.New(-10000, "USD"), MemberID: "you", CategoryID: "shopping",
			Splits: []domain.CategorySplit{
				{CategoryID: "shopping", Amount: money.New(-6000, "USD")},                    // -> you (payer)
				{CategoryID: "shopping", Amount: money.New(-4000, "USD"), MemberID: "priya"}, // -> priya
			},
		},
	}

	shares, err := AttributeByMember(budget, txns, start, end, rates, nil)
	if err != nil {
		t.Fatalf("attribute: %v", err)
	}
	got := map[string]int64{}
	for _, s := range shares {
		got[s.MemberID] = s.Spent.Amount
	}
	if got["you"] != 6000 || got["priya"] != 4000 {
		t.Errorf("split owner attribution wrong: %+v", got)
	}
}
