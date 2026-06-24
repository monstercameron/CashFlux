// SPDX-License-Identifier: MIT

package subscriptions

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestChargedAfterCancel(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}

	cancels := []domain.SubscriptionCancellation{
		{ID: "c1", SubName: "Netflix", CancelledOn: date(2026, 3, 2)},
		{ID: "c2", SubName: "Spotify", CancelledOn: date(2026, 1, 15)},
	}

	txns := []domain.Transaction{
		// Netflix charged AFTER cancel — should appear.
		{ID: "t1", Desc: "Netflix", Date: date(2026, 4, 2), Amount: money.New(-1599, "USD")},
		// Netflix charged BEFORE cancel — should be ignored.
		{ID: "t2", Desc: "Netflix", Date: date(2026, 2, 2), Amount: money.New(-1599, "USD")},
		// Netflix charged ON cancel day — strictly-after rule excludes it.
		{ID: "t3", Desc: "Netflix", Date: date(2026, 3, 2), Amount: money.New(-1599, "USD")},
		// Spotify charged after cancel — should appear.
		{ID: "t4", Desc: "Spotify", Date: date(2026, 2, 1), Amount: money.New(-999, "USD")},
		// Non-matching description — should be ignored.
		{ID: "t5", Desc: "Amazon Prime", Date: date(2026, 4, 5), Amount: money.New(-1399, "USD")},
		// Income (positive) — should be excluded.
		{ID: "t6", Desc: "Netflix", Date: date(2026, 5, 1), Amount: money.New(5000, "USD")},
		// Transfer — should be excluded.
		{ID: "t7", Desc: "Netflix", Date: date(2026, 5, 2), Amount: money.New(-1599, "USD"), TransferAccountID: "acct-b"},
	}

	got, err := ChargedAfterCancel(txns, cancels, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d late charges, want 2: %+v", len(got), got)
	}
	// sorted by ChargeDate asc then SubName: Spotify (Feb 1) before Netflix (Apr 2)
	if got[0].SubName != "Spotify" {
		t.Errorf("got[0].SubName = %q, want Spotify", got[0].SubName)
	}
	if got[1].SubName != "Netflix" {
		t.Errorf("got[1].SubName = %q, want Netflix", got[1].SubName)
	}
	if got[1].Amount != 1599 {
		t.Errorf("got[1].Amount = %d, want 1599", got[1].Amount)
	}
}

func TestChargedAfterCancel_CaseInsensitive(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	cancels := []domain.SubscriptionCancellation{
		{ID: "c1", SubName: "NETFLIX", CancelledOn: date(2026, 1, 1)},
	}
	txns := []domain.Transaction{
		{ID: "t1", Desc: "netflix", Date: date(2026, 2, 1), Amount: money.New(-1599, "USD")},
	}
	got, err := ChargedAfterCancel(txns, cancels, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
}

func TestChargedAfterCancel_FXConversion(t *testing.T) {
	// 1 EUR = 1.1 USD; a €10 charge is $11.00
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}
	cancels := []domain.SubscriptionCancellation{
		{ID: "c1", SubName: "Deezer", CancelledOn: date(2026, 1, 1)},
	}
	txns := []domain.Transaction{
		{ID: "t1", Desc: "Deezer", Date: date(2026, 2, 1), Amount: money.New(-1000, "EUR")},
	}
	got, err := ChargedAfterCancel(txns, cancels, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if got[0].Amount != 1100 {
		t.Errorf("amount = %d, want 1100 (EUR→USD)", got[0].Amount)
	}
}

func TestChargedAfterCancel_MultipleCancels(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	cancels := []domain.SubscriptionCancellation{
		{ID: "c1", SubName: "ServiceA", CancelledOn: date(2026, 1, 10)},
		{ID: "c2", SubName: "ServiceB", CancelledOn: date(2026, 2, 5)},
	}
	txns := []domain.Transaction{
		{ID: "t1", Desc: "ServiceA", Date: date(2026, 1, 15), Amount: money.New(-500, "USD")},
		{ID: "t2", Desc: "ServiceB", Date: date(2026, 2, 10), Amount: money.New(-800, "USD")},
		{ID: "t3", Desc: "ServiceA", Date: date(2026, 1, 5), Amount: money.New(-500, "USD")}, // before cancel
	}
	got, err := ChargedAfterCancel(txns, cancels, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d late charges, want 2", len(got))
	}
}

func TestChargedAfterCancel_EmptyCancels(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	txns := []domain.Transaction{
		{ID: "t1", Desc: "Netflix", Date: date(2026, 4, 1), Amount: money.New(-1599, "USD")},
	}
	got, err := ChargedAfterCancel(txns, nil, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d, want 0", len(got))
	}
}
