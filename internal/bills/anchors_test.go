// SPDX-License-Identifier: MIT

package bills

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestLiabilityAnchors guards the meaning of "account-tied": a flow is anchored
// when it settles a liability, NOT when it merely posts from a funding account.
// Every flow in this table carries the same funding account, so a reading of
// domain.Recurring.AccountID would call all of them anchored.
func TestLiabilityAnchors(t *testing.T) {
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	now := onDay(2026, 7, 1)
	until := onDay(2026, 8, 15)
	const funding = "acct-checking"

	accounts := []domain.Account{
		{ID: funding, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "acct-carloan", Name: "Car Loan", Class: domain.ClassLiability, Type: domain.TypeLoan,
			Currency: "USD", DueDayOfMonth: 15, MinPayment: usd(62000)},
		{ID: "acct-mortgage", Name: "Mortgage", Class: domain.ClassLiability, Type: domain.TypeMortgage,
			Currency: "USD", DueDayOfMonth: 1, MinPayment: usd(148000)},
	}
	recurring := []domain.Recurring{
		{ID: "rec-carpay", Label: "Car payment", Amount: usd(-62000), Cadence: domain.CadenceMonthly,
			NextDue: onDay(2026, 7, 15), AccountID: funding},
		{ID: "rec-mortgage", Label: "Mortgage payment", Amount: usd(-148000), Cadence: domain.CadenceMonthly,
			NextDue: onDay(2026, 8, 1), AccountID: funding},
		{ID: "rec-gym", Label: "Gym membership", Amount: usd(-5000), Cadence: domain.CadenceMonthly,
			NextDue: onDay(2026, 7, 3), AccountID: funding},
		{ID: "rec-hoa", Label: "HOA dues", Amount: usd(-38000), Cadence: domain.CadenceMonthly,
			NextDue: onDay(2026, 8, 1), AccountID: funding},
	}

	got := LiabilityAnchors(accounts, recurring, now, until)
	want := map[string]string{
		"rec-carpay":   "acct-carloan",
		"rec-mortgage": "acct-mortgage",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for id, anchor := range want {
		if got[id] != anchor {
			t.Errorf("flow %q: got anchor %q, want %q", id, got[id], anchor)
		}
	}
	for _, free := range []string{"rec-gym", "rec-hoa"} {
		if a, ok := got[free]; ok {
			t.Errorf("free-floating flow %q reported anchor %q", free, a)
		}
	}
}

// TestLiabilityAnchorsEmpty keeps the helper honest on an empty household — no
// accounts, no flows, no panic, no phantom anchors.
func TestLiabilityAnchorsEmpty(t *testing.T) {
	got := LiabilityAnchors(nil, nil, time.Now(), time.Now().AddDate(0, 0, 45))
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}
