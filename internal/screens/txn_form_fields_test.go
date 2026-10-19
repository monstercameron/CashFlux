// SPDX-License-Identifier: MIT

package screens

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestTxnFormAccountOptions(t *testing.T) {
	tests := []struct {
		name     string
		accounts []domain.Account
		selected string
		want     []TxnFormOption
	}{
		{
			name: "sorts by visible name and keeps names unqualified",
			accounts: []domain.Account{
				{ID: "z", Name: "Savings", Type: domain.TypeSavings},
				{ID: "b", Name: "Checking", Type: domain.TypeChecking},
				{ID: "a", Name: "Checking", Type: domain.TypeChecking},
			},
			want: []TxnFormOption{{Value: "a", Label: "Checking"}, {Value: "b", Label: "Checking"}, {Value: "z", Label: "Savings"}},
		},
		{
			name: "keeps selected archived account only",
			accounts: []domain.Account{
				{ID: "old", Name: "Old account", Archived: true},
				{ID: "active", Name: "Active"},
				{ID: "other", Name: "Other archived", Archived: true},
			},
			selected: "old",
			want:     []TxnFormOption{{Value: "active", Label: "Active"}, {Value: "old", Label: "Old account"}},
		},
		{
			name: "empty accounts",
			want: []TxnFormOption{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TxnFormAccountOptions(tt.accounts, tt.selected); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("TxnFormAccountOptions() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTxnEditHeading(t *testing.T) {
	tests := []struct {
		name     string
		txn      domain.Transaction
		wantHead string
		wantSub  string
	}{
		{name: "description with distinct payee", txn: domain.Transaction{Desc: "Groceries", Payee: "Greenfield Market"}, wantHead: "Groceries", wantSub: "Greenfield Market"},
		{name: "same description and payee", txn: domain.Transaction{Desc: "Coffee", Payee: "Coffee"}, wantHead: "Coffee"},
		{name: "description only", txn: domain.Transaction{Desc: "Rent"}, wantHead: "Rent"},
		{name: "payee fallback", txn: domain.Transaction{Payee: "Market"}, wantHead: "Market"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHead, gotSub := TxnEditHeading(tt.txn)
			if gotHead != tt.wantHead || gotSub != tt.wantSub {
				t.Fatalf("TxnEditHeading() = (%q, %q), want (%q, %q)", gotHead, gotSub, tt.wantHead, tt.wantSub)
			}
		})
	}
}
