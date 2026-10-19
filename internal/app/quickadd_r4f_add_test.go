// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/screens"
)

func TestQuickAddPayeeOrDescriptionR4F(t *testing.T) {
	accounts := []domain.Account{{ID: "checking", Currency: "USD"}}
	tests := []struct {
		name, payee, description, wantField, wantKey string
	}{
		{name: "payee only succeeds", payee: "Corner Market"},
		{name: "description only succeeds", description: "Lunch with Sam"},
		{name: "both empty reports on payee", wantField: "txn-add-payee", wantKey: "r4f.addPayeeOrDescription"},
		{name: "whitespace only reports on payee", payee: "  ", description: "\t", wantField: "txn-add-payee", wantKey: "r4f.addPayeeOrDescription"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			field, key := quickAddInvalidFieldR4F(accounts, "checking", "12.50", "Expense", tc.payee, tc.description)
			if field != tc.wantField || key != tc.wantKey {
				t.Fatalf("quickAddInvalidFieldR4F() = (%q, %q), want (%q, %q)", field, key, tc.wantField, tc.wantKey)
			}
		})
	}
}

func TestQuickAddDescriptionDefaultsToPayeeR4F(t *testing.T) {
	tests := []struct {
		name, payee, description, want string
	}{
		{name: "description left empty", payee: " Corner Market ", want: "Corner Market"},
		{name: "explicit description retained", payee: "Corner Market", description: " Groceries ", want: "Groceries"},
		{name: "both empty", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := quickAddDescriptionR4F(tc.payee, tc.description); got != tc.want {
				t.Fatalf("quickAddDescriptionR4F() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestQuickAddSentenceFillFocusesPayeeWhenBothAreEmptyR4F(t *testing.T) {
	accounts := []domain.Account{{ID: "checking", Currency: "USD"}}
	got := quickAddNextFocusR4F(accounts, "checking", "12.50", "", "", screens.TxnDraftFill{})
	if got != "txn-add-payee" {
		t.Fatalf("focus after incomplete sentence fill = %q, want txn-add-payee", got)
	}
}
