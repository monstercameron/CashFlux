// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

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

func TestTransferMissingFXRequiresExplicitReceivedAmount(t *testing.T) {
	a := newApp(t, false)
	from, _ := seedAssetPair(t, a, 100000, 0)
	yen := domain.Account{
		ID: "yen", Name: "Yen account", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "JPY",
		OpeningBalance: money.New(0, "JPY"),
	}
	if err := a.PutAccount(yen); err != nil {
		t.Fatalf("PutAccount yen: %v", err)
	}
	_, _, err := a.CreateTransferPair(TransferParams{
		OperationID: "missing-rate", FromAccountID: from.ID, ToAccountID: yen.ID, AmountMinor: 10000,
	})
	if !errors.Is(err, ErrTransferRateUnavailable) {
		t.Fatalf("CreateTransferPair error = %v, want ErrTransferRateUnavailable", err)
	}
	if got := balanceOf(t, a, from); got != 100000 {
		t.Errorf("source after rejected transfer = %d, want 100000", got)
	}
	if got := balanceOf(t, a, yen); got != 0 {
		t.Errorf("destination after rejected transfer = %d, want 0", got)
	}
	if len(a.Transactions()) != 0 {
		t.Errorf("transactions after rejected transfer = %d, want 0", len(a.Transactions()))
	}
	if _, _, err := a.CreateTransferPair(TransferParams{
		OperationID: "explicit-received", FromAccountID: from.ID, ToAccountID: yen.ID,
		AmountMinor: 10000, ReceivedMinor: 15000,
	}); err != nil {
		t.Fatalf("explicit received transfer: %v", err)
	}
	if got := balanceOf(t, a, yen); got != 15000 {
		t.Errorf("destination after explicit transfer = %d, want 15000", got)
	}
}

func TestTransferWithRateAndOperationRetry(t *testing.T) {
	a := newApp(t, false)
	from, to := seedFXPair(t, a)
	p := TransferParams{
		OperationID: "retryable-fx", FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000,
		Date: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), Desc: "Retry test",
	}
	outID, inID, err := a.CreateTransferPair(p)
	if err != nil {
		t.Fatalf("first CreateTransferPair: %v", err)
	}
	outRetry, inRetry, err := a.CreateTransferPair(p)
	if err != nil {
		t.Fatalf("retry CreateTransferPair: %v", err)
	}
	if outRetry != outID || inRetry != inID {
		t.Fatalf("retry IDs = %q/%q, want %q/%q", outRetry, inRetry, outID, inID)
	}
	p.FeeMinor = 100
	if _, _, err := a.CreateTransferPair(p); err == nil {
		t.Fatal("same operation ID with a different fee was accepted")
	}
	if len(a.Transactions()) != 2 {
		t.Fatalf("transactions after retry = %d, want 2", len(a.Transactions()))
	}
	if got := balanceOf(t, a, from); got != 95000 {
		t.Errorf("source after retry = %d, want 95000", got)
	}
	if got := balanceOf(t, a, to); got != 12500 {
		t.Errorf("destination after retry = %d, want 12500", got)
	}
}

func TestEditingCrossCurrencyTransferWithoutFXDoesNotRelabelAmount(t *testing.T) {
	a := newApp(t, false)
	from, to := seedFXPair(t, a)
	outID, _, err := a.CreateTransferPair(TransferParams{
		OperationID: "edit-missing-rate", FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 5000,
	})
	if err != nil {
		t.Fatalf("CreateTransferPair: %v", err)
	}
	s := a.Settings()
	s.FXRates = nil
	if err := a.PutSettings(s); err != nil {
		t.Fatalf("clear FX rates: %v", err)
	}
	var prior domain.Transaction
	for _, txn := range a.Transactions() {
		if txn.ID == outID {
			prior = txn
		}
	}
	if prior.ID == "" {
		t.Fatal("source transfer leg not found")
	}
	next := prior
	next.Amount.Amount = -6000
	if err := a.PutTransactionWithTransferPair(prior, next); !errors.Is(err, ErrTransferRateUnavailable) {
		t.Fatalf("PutTransactionWithTransferPair error = %v, want ErrTransferRateUnavailable", err)
	}
	for _, txn := range a.Transactions() {
		if txn.ID == prior.ID && txn.Amount.Amount != -5000 {
			t.Errorf("source leg after rejected edit = %d, want -5000", txn.Amount.Amount)
		}
		if txn.AccountID == to.ID && txn.TransferGroupID == prior.TransferGroupID && txn.Amount.Amount != 2500 {
			t.Errorf("destination leg after rejected edit = %d, want 2500", txn.Amount.Amount)
		}
	}
	if err := a.PutTransactionWithTransferPairReceived(prior, next, 2700); err != nil {
		t.Fatalf("explicit destination amount edit: %v", err)
	}
	for _, txn := range a.Transactions() {
		if txn.ID == prior.ID && txn.Amount.Amount != -6000 {
			t.Errorf("source leg after explicit edit = %d, want -6000", txn.Amount.Amount)
		}
		if txn.AccountID == to.ID && txn.TransferGroupID == prior.TransferGroupID && txn.Amount.Amount != 2700 {
			t.Errorf("destination leg after explicit edit = %d, want 2700", txn.Amount.Amount)
		}
	}
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
