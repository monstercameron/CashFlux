// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// seedAssetPair creates two same-owner USD asset accounts with the given
// opening balances (minor units).
func seedAssetPair(t *testing.T, a *App, fromMinor, toMinor int64) (from, to domain.Account) {
	t.Helper()
	from = domain.Account{
		ID: "pchk", Name: "Preview Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(fromMinor, "USD"),
	}
	to = domain.Account{
		ID: "psav", Name: "Preview Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
		OpeningBalance: money.New(toMinor, "USD"),
	}
	if err := a.PutAccount(from); err != nil {
		t.Fatalf("seedAssetPair PutAccount from: %v", err)
	}
	if err := a.PutAccount(to); err != nil {
		t.Fatalf("seedAssetPair PutAccount to: %v", err)
	}
	return from, to
}

func TestPreviewTransferPairSameCurrency(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 25000) // $1,000.00 / $250.00

	pv, err := a.PreviewTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000,
	})
	if err != nil {
		t.Fatalf("PreviewTransferPair: %v", err)
	}
	if pv.FromBefore.Amount != 100000 || pv.FromAfter.Amount != 95000 {
		t.Errorf("from before/after = %d/%d, want 100000/95000", pv.FromBefore.Amount, pv.FromAfter.Amount)
	}
	if pv.ToBefore.Amount != 25000 || pv.ToAfter.Amount != 30000 {
		t.Errorf("to before/after = %d/%d, want 25000/30000", pv.ToBefore.Amount, pv.ToAfter.Amount)
	}
}

// TestPreviewTransferPairMatchesPost is the parity guarantee: posting the same
// params must land both accounts exactly on the previewed after-balances.
func TestPreviewTransferPairMatchesPost(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 25000)

	pv, err := a.PreviewTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 12345,
	})
	if err != nil {
		t.Fatalf("PreviewTransferPair: %v", err)
	}
	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 12345,
	}); err != nil {
		t.Fatalf("CreateTransferPair: %v", err)
	}
	if got := balanceOf(t, a, from); got != pv.FromAfter.Amount {
		t.Errorf("posted from balance = %d, preview said %d", got, pv.FromAfter.Amount)
	}
	if got := balanceOf(t, a, to); got != pv.ToAfter.Amount {
		t.Errorf("posted to balance = %d, preview said %d", got, pv.ToAfter.Amount)
	}
}

// TestPreviewTransferPairLiabilityPayment covers both at-rest debt sign
// conventions: the previewed after-balance must move the debt toward zero.
func TestPreviewTransferPairLiabilityPayment(t *testing.T) {
	cases := []struct {
		name         string
		openingMinor int64
		wantToAfter  int64
	}{
		{"positive-stored debt shrinks", 50000, 45000},
		{"negative-stored debt rises toward zero", -50000, -45000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			checking, loan := seedLiability(t, a, tc.openingMinor)

			pv, err := a.PreviewTransferPair(TransferParams{
				FromAccountID: checking.ID, ToAccountID: loan.ID, AmountMinor: 5000,
			})
			if err != nil {
				t.Fatalf("PreviewTransferPair: %v", err)
			}
			if pv.ToAfter.Amount != tc.wantToAfter {
				t.Errorf("loan after = %d, want %d", pv.ToAfter.Amount, tc.wantToAfter)
			}
			if pv.FromAfter.Amount != 95000 {
				t.Errorf("checking after = %d, want 95000", pv.FromAfter.Amount)
			}
		})
	}
}

// TestPreviewTransferPairFXConversion previews across currencies at a saved
// rate: base USD, 1 EUR = 2 USD, so $50.00 lands as €25.00.
func TestPreviewTransferPairFXConversion(t *testing.T) {
	a := newApp(t, false)
	from, _ := seedAssetPair(t, a, 100000, 0)
	eur := domain.Account{
		ID: "peur", Name: "Euro Card", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR",
		OpeningBalance: money.New(10000, "EUR"), // €100.00
	}
	if err := a.PutAccount(eur); err != nil {
		t.Fatalf("PutAccount eur: %v", err)
	}
	s := a.Settings()
	s.BaseCurrency = "USD"
	s.FXRates = map[string]float64{"EUR": 2.0}
	if err := a.PutSettings(s); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}

	pv, err := a.PreviewTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: eur.ID, AmountMinor: 5000, // $50.00
	})
	if err != nil {
		t.Fatalf("PreviewTransferPair: %v", err)
	}
	if pv.FromAfter.Amount != 95000 || pv.FromAfter.Currency != "USD" {
		t.Errorf("from after = %d %s, want 95000 USD", pv.FromAfter.Amount, pv.FromAfter.Currency)
	}
	if pv.ToAfter.Amount != 12500 || pv.ToAfter.Currency != "EUR" {
		t.Errorf("to after = %d %s, want 12500 EUR", pv.ToAfter.Amount, pv.ToAfter.Currency)
	}
}

func TestPreviewTransferPairValidation(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 25000)

	cases := []struct {
		name string
		p    TransferParams
	}{
		{"missing accounts", TransferParams{AmountMinor: 100}},
		{"same account", TransferParams{FromAccountID: from.ID, ToAccountID: from.ID, AmountMinor: 100}},
		{"zero amount", TransferParams{FromAccountID: from.ID, ToAccountID: to.ID}},
		{"negative amount", TransferParams{FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: -5}},
		{"unknown destination", TransferParams{FromAccountID: from.ID, ToAccountID: "nope", AmountMinor: 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := a.PreviewTransferPair(tc.p); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}
