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
		{ID: "6", Date: mid, CategoryID: "dining", Amount: money.New(-7000, "USD"), MemberID: "you", ExcludeFromReports: true},
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

func TestAttributeByMemberExclusionsReconcileWithSpent(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start, end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "b", Scope: domain.ScopeShared, CategoryID: "dining", Limit: money.New(50000, "USD")}
	txns := []domain.Transaction{
		{ID: "included", Date: start.AddDate(0, 0, 1), CategoryID: "dining", Amount: money.New(-2500, "USD"), MemberID: "m1"},
		{ID: "excluded", Date: start.AddDate(0, 0, 2), CategoryID: "dining", Amount: money.New(-9000, "USD"), MemberID: "m2", ExcludeFromReports: true},
	}
	shares, err := AttributeByMember(b, txns, start, end, rates, nil)
	if err != nil {
		t.Fatal(err)
	}
	status, err := Evaluate(b, txns, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, share := range shares {
		sum += share.Spent.Amount
	}
	if sum != status.Spent.Amount || sum != 2500 {
		t.Fatalf("shares %d != Spent %d; want 2500", sum, status.Spent.Amount)
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

func TestAttributeByMemberNetsPositiveDiscountLine(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start, end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: money.New(20000, "USD")}
	txn := domain.Transaction{ID: "receipt", Date: start.AddDate(0, 0, 2), CategoryID: "food", MemberID: "m1", Amount: money.New(-10000, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-12000, "USD")},
		{CategoryID: "food", Amount: money.New(2000, "USD")},
	}}
	shares, err := AttributeByMember(b, []domain.Transaction{txn}, start, end, rates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 1 || shares[0].Spent.Amount != 10000 {
		t.Fatalf("discount line should net to $100.00, got %+v", shares)
	}
}

func TestAttributeByMemberDiscountAcrossOwnersReconciles(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start, end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: money.New(20000, "USD")}
	txn := domain.Transaction{ID: "receipt", Date: start.AddDate(0, 0, 2), CategoryID: "food", Amount: money.New(-10000, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-12000, "USD"), MemberID: "payer"},
		{CategoryID: "food", Amount: money.New(2000, "USD"), MemberID: "discount"},
	}}
	shares, err := AttributeByMember(b, []domain.Transaction{txn}, start, end, rates, nil)
	if err != nil {
		t.Fatal(err)
	}
	byMember := map[string]int64{}
	for _, share := range shares {
		byMember[share.MemberID] = share.Spent.Amount
	}
	if byMember["payer"] != 12000 || byMember["discount"] != -2000 || byMember["payer"]+byMember["discount"] != 10000 {
		t.Fatalf("signed owner shares should net to $100 spend: shares=%v", byMember)
	}
}

func TestAttributeByMemberSplitsSharedTransactionMinorUnits(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "b1", Scope: domain.ScopeShared, CategoryID: "dining", Limit: money.New(50000, "USD")}
	txns := []domain.Transaction{{
		ID: "shared", Date: mid, CategoryID: "dining", Amount: money.New(-1001, "USD"), MemberIDs: []string{"you", "priya"},
	}}
	shares, err := AttributeByMember(budget, txns, start, end, rates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 2 || shares[0].MemberID != "you" || shares[0].Spent.Amount != 501 || shares[1].MemberID != "priya" || shares[1].Spent.Amount != 500 {
		t.Fatalf("$10.01 shared spend attributed as %#v", shares)
	}
}
