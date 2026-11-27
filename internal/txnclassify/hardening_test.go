// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestApplyClearsStaleTransferGroupID(t *testing.T) {
	accounts := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "sav", Name: "Savings", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "cc", Name: "Card", Class: domain.ClassLiability, Currency: "USD"},
	}
	orig := domain.Transaction{
		ID: "t1", AccountID: "chk", TransferAccountID: "sav", TransferGroupID: "grp-1",
		Amount: money.New(-5000, "USD"),
	}

	unclassified, err := Apply(orig, "", false, accounts)
	if err != nil {
		t.Fatalf("unclassify: %v", err)
	}
	if unclassified.TransferGroupID != "" || unclassified.TransferAccountID != "" {
		t.Fatalf("unclassify left stale transfer fields: %+v", unclassified)
	}

	repointed, err := Apply(orig, "cc", true, accounts)
	if err != nil {
		t.Fatalf("repoint: %v", err)
	}
	if repointed.TransferGroupID != "" || repointed.TransferAccountID != "cc" {
		t.Fatalf("repoint kept stale TransferGroupID: %+v", repointed)
	}

	// Toggling debt-payment flag without changing counterparty preserves TransferGroupID.
	toggled, err := Apply(orig, "sav", false, accounts)
	if err != nil {
		t.Fatalf("same counterparty: %v", err)
	}
	if toggled.TransferGroupID != "grp-1" {
		t.Fatalf("same counterparty cleared TransferGroupID: %+v", toggled)
	}
}

func TestSuggestByReciprocalHonorsPositiveOwedLiability(t *testing.T) {
	d := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Currency: "USD"},
		{
			ID: "cc", Name: "Card", Class: domain.ClassLiability,
			LiabilitySign: domain.LiabilityPositiveOwed, Currency: "USD",
			OpeningBalance: money.New(50000, "USD"),
		},
	}
	out := domain.Transaction{ID: "out", AccountID: "chk", Date: d, Desc: "Payment", Amount: money.New(-10000, "USD")}
	in := domain.Transaction{ID: "in", AccountID: "cc", Date: d, Desc: "Thank you", Amount: money.New(-10000, "USD")}

	sug, ok := suggestByReciprocal(out, accounts, []domain.Transaction{out, in})
	if !ok || sug.AccountID != "cc" || !sug.Exact {
		t.Fatalf("suggestByReciprocal = %+v,%v, want cc exact", sug, ok)
	}
}

func TestFindReciprocalRejectsFarApartYearsOverflow(t *testing.T) {
	d1 := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(1700, 1, 1, 0, 0, 0, 0, time.UTC)
	a := domain.Transaction{ID: "a", AccountID: "chk", Date: d1, Amount: money.New(-5000, "USD")}
	b := domain.Transaction{ID: "b", AccountID: "sav", Date: d2, Amount: money.New(5000, "USD")}
	if _, ok := FindReciprocal(a, "sav", []domain.Transaction{a, b}); ok {
		t.Fatal("FindReciprocal matched rows >292 years apart")
	}
	if AmbiguousLeg(a, "sav", []domain.Transaction{a, b, b}) {
		t.Fatal("AmbiguousLeg matched rows >292 years apart")
	}
}
