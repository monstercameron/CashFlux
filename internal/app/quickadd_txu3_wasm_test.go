// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/screens"
)

func TestQuickAddRequiredFieldErrors(t *testing.T) {
	accounts := []domain.Account{{ID: "checking", Currency: "USD"}}
	tests := []struct {
		name, accountID, amount, kind, payee, description string
		wantField, wantKey                                string
	}{
		{name: "missing account", amount: "12.50", kind: "Expense", description: "Lunch", wantField: "txn-add-account", wantKey: "txu3.needAccount"},
		{name: "empty amount", accountID: "checking", kind: "Expense", description: "Lunch", wantField: "txn-add-amount", wantKey: "txu3.needAmount"},
		{name: "invalid amount", accountID: "checking", amount: "coffee", kind: "Expense", description: "Lunch", wantField: "txn-add-amount", wantKey: "txu3.needAmount"},
		{name: "both payee and description missing", accountID: "checking", amount: "12.50", kind: "Expense", wantField: "txn-add-payee", wantKey: "r4f.addPayeeOrDescription"},
		{name: "payee only is valid", accountID: "checking", amount: "12.50", kind: "Expense", payee: "Corner Market"},
		{name: "description only is valid", accountID: "checking", amount: "12.50", kind: "Expense", description: "Lunch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			field, key := quickAddInvalidFieldR4F(accounts, tc.accountID, tc.amount, tc.kind, tc.payee, tc.description)
			if field != tc.wantField || key != tc.wantKey {
				t.Fatalf("quickAddInvalidFieldR4F() = (%q, %q), want (%q, %q)", field, key, tc.wantField, tc.wantKey)
			}
		})
	}
}

func TestQuickAddDefaultsToReviewedAndCurrentMember(t *testing.T) {
	if !quickAddReviewedValue(false) {
		t.Fatal("an unchecked Needs review later box should save as reviewed")
	}
	if quickAddReviewedValue(true) {
		t.Fatal("a checked Needs review later box should save as not reviewed")
	}
	if got := quickAddMemberValue("member-current", "", false); got != "member-current" {
		t.Fatalf("default member = %q, want current member", got)
	}
	if got := quickAddMemberValue("member-current", "member-other", true); got != "member-other" {
		t.Fatalf("explicit member = %q, want selected member", got)
	}
	if got := quickAddMemberValue("member-current", "", true); got != "" {
		t.Fatalf("explicit automatic assignment = %q, want empty", got)
	}
}

func TestQuickAddNextFocusAfterSentenceFill(t *testing.T) {
	accounts := []domain.Account{{ID: "checking", Currency: "USD"}}
	tests := []struct {
		name, accountID, amount, payee, description string
		fill                                        screens.TxnDraftFill
		want                                        string
	}{
		{name: "missing account", amount: "40", payee: "Whole Foods", fill: screens.TxnDraftFill{AmountMajor: "40", Payee: "Whole Foods"}, want: "txn-add-account"},
		{name: "missing amount", accountID: "checking", fill: screens.TxnDraftFill{Payee: "Whole Foods"}, want: "txn-add-amount"},
		{name: "missing payee and description", accountID: "checking", fill: screens.TxnDraftFill{AmountMajor: "40"}, want: "txn-add-payee"},
		{name: "all required values filled", accountID: "checking", fill: screens.TxnDraftFill{AmountMajor: "40", Payee: "Whole Foods"}, want: "txn-add-save"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := quickAddNextFocusR4F(accounts, tc.accountID, tc.amount, tc.payee, tc.description, tc.fill); got != tc.want {
				t.Fatalf("quickAddNextFocusR4F() = %q, want %q", got, tc.want)
			}
		})
	}
}
