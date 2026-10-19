// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

func TestTxnLinkSubscriptionPickerUsesDetectedSubscriptions(t *testing.T) {
	detected := []subscriptions.Subscription{
		{Name: "HOA dues", Amount: 4000},
		{Name: "Dinner out"},
		{Name: "HOA dues"}, // one merchant may be detected in multiple currencies
		{Name: "  "},
	}
	got := txnLinkSubscriptionNames(detected)
	want := []string{"HOA dues", "Dinner out"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subscription picker names = %#v, want %#v", got, want)
	}
}

func TestTxnLinkSubscriptionPickerExcludesNonSubscriptionSeries(t *testing.T) {
	detected := []subscriptions.Subscription{
		{Name: "Dinner out"},
		{Name: "Coffee & snacks"},
		{Name: "Gas"},
		{Name: "Takeout"},
		{Name: "Holiday gifts"},
		{Name: "HOA dues", Amount: 4000},
	}
	txns := []domain.Transaction{
		{Desc: "Dinner out", CategoryID: "dining"},
		{Desc: "Coffee & snacks", CategoryID: "dining"},
		{Desc: "Gas", CategoryID: "utilities"},
		{Desc: "Takeout", CategoryID: "dining"},
		{Desc: "Holiday gifts", CategoryID: "gifts"},
		{Desc: "HOA dues", CategoryID: "membership", Amount: money.New(-4000, "USD")},
	}
	filtered := txnLinkFilterSubscriptionKinds(detected, txns, txns, nil, map[string]string{
		"dining":     "Dining",
		"utilities":  "Utilities",
		"gifts":      "Gifts",
		"membership": "Membership",
	}, nil, nil)
	if got, want := txnLinkSubscriptionNames(filtered), []string{"HOA dues"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("subscription picker names = %#v, want %#v", got, want)
	}
	if got := bestTxnSubscription(txns[len(txns)-1], filtered, currency.Rates{Base: "USD"}); got != "HOA dues" {
		t.Fatalf("same-amount HOA dues payment preselection = %q, want HOA dues", got)
	}
}

func TestBestTxnSubscriptionRequiresMerchantAndAmountWithinTenPercent(t *testing.T) {
	detected := []subscriptions.Subscription{
		{Name: "Dinner out", Amount: 4000},
		{Name: "HOA dues", Amount: 4399},
		{Name: "HOA dues", Amount: 4000},
		{Name: "HOA dues", Amount: 4500}, // outside tolerance
	}
	rates := currency.Rates{Base: "USD"}
	txn := domain.Transaction{Payee: "hoa dues", Amount: money.New(-4000, "USD")}
	if got := bestTxnSubscription(txn, detected, rates); got != "HOA dues" {
		t.Fatalf("bestTxnSubscription() = %q, want HOA dues", got)
	}

	txn.Payee = "Property manager"
	txn.Desc = "HOA dues"
	if got := bestTxnSubscription(txn, detected, rates); got != "HOA dues" {
		t.Fatalf("description match = %q, want HOA dues", got)
	}

	txn.Payee, txn.Desc = "HOA dues", ""
	if got := bestTxnSubscription(txn, detected[:1], rates); got != "" {
		t.Fatalf("unrelated merchant match = %q, want empty", got)
	}
}
