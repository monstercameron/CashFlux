// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// seedFXPair creates a USD source, a EUR destination, and a 1 EUR = 2 USD
// saved rate (base USD), returning both accounts.
func seedFXPair(t *testing.T, a *App) (from, eur domain.Account) {
	t.Helper()
	from, _ = seedAssetPair(t, a, 100000, 0)
	eur = domain.Account{
		ID: "oeur", Name: "Override Euro Card", OwnerID: "m1", Scope: domain.ScopeIndividual,
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
	return from, eur
}

// TestTransferReceivedOverride: the user's actual landed amount beats the saved
// rate — $50.00 with ReceivedMinor €46.10 lands exactly €46.10, not €25.00.
func TestTransferReceivedOverride(t *testing.T) {
	a := newApp(t, false)
	from, eur := seedFXPair(t, a)

	pv, err := a.PreviewTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: eur.ID, AmountMinor: 5000, ReceivedMinor: 4610,
	})
	if err != nil {
		t.Fatalf("PreviewTransferPair: %v", err)
	}
	if pv.ToAfter.Amount != 10000+4610 {
		t.Errorf("preview to after = %d, want %d", pv.ToAfter.Amount, 10000+4610)
	}
	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: eur.ID, AmountMinor: 5000, ReceivedMinor: 4610,
	}); err != nil {
		t.Fatalf("CreateTransferPair: %v", err)
	}
	if got := balanceOf(t, a, eur); got != 10000+4610 {
		t.Errorf("posted eur balance = %d, want %d", got, 10000+4610)
	}
	if got := balanceOf(t, a, from); got != 95000 {
		t.Errorf("posted from balance = %d, want 95000", got)
	}
}

// TestTransferReceivedOverrideIgnoredSameCurrency: for a same-currency pair the
// moved amount IS the amount — a stray override must not desync the legs.
func TestTransferReceivedOverrideIgnoredSameCurrency(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 25000)

	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000, ReceivedMinor: 9999,
	}); err != nil {
		t.Fatalf("CreateTransferPair: %v", err)
	}
	if got := balanceOf(t, a, to); got != 30000 {
		t.Errorf("posted to balance = %d, want 30000 (override must be ignored)", got)
	}
}

// TestTransferFee posts a third real expense on the source account, outside the
// transfer pair (it must count as spending), and the preview includes it.
func TestTransferFee(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 25000)

	pv, err := a.PreviewTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000, FeeMinor: 250,
	})
	if err != nil {
		t.Fatalf("PreviewTransferPair: %v", err)
	}
	if pv.FromAfter.Amount != 100000-5000-250 {
		t.Errorf("preview from after = %d, want %d", pv.FromAfter.Amount, 100000-5000-250)
	}
	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000, FeeMinor: 250,
	}); err != nil {
		t.Fatalf("CreateTransferPair: %v", err)
	}
	if got := balanceOf(t, a, from); got != 100000-5000-250 {
		t.Errorf("posted from balance = %d, want %d", got, 100000-5000-250)
	}
	if got := balanceOf(t, a, to); got != 30000 {
		t.Errorf("posted to balance = %d, want 30000 (fee stays on the source)", got)
	}
	// The fee must be a plain expense — not a transfer leg — so it shows up in
	// spending totals and doesn't orphan-pair with anything.
	var fees int
	for _, tx := range a.Transactions() {
		if tx.AccountID == from.ID && tx.Amount.Amount == -250 {
			if tx.TransferAccountID != "" {
				t.Errorf("fee transaction must not carry TransferAccountID, got %q", tx.TransferAccountID)
			}
			fees++
		}
	}
	if fees != 1 {
		t.Errorf("fee transactions = %d, want 1", fees)
	}
}

// TestTransferNegativeOverrideAndFeeRejected: bad inputs are refused up front.
func TestTransferNegativeOverrideAndFeeRejected(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 25000)

	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000, ReceivedMinor: -1,
	}); err == nil {
		t.Error("negative ReceivedMinor: expected an error, got nil")
	}
	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000, FeeMinor: -1,
	}); err == nil {
		t.Error("negative FeeMinor: expected an error, got nil")
	}
	if _, err := a.PreviewTransferPair(TransferParams{
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000, FeeMinor: -1,
	}); err == nil {
		t.Error("preview negative FeeMinor: expected an error, got nil")
	}
}
