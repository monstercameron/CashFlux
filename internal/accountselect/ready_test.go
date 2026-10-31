// SPDX-License-Identifier: MIT

package accountselect

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestCanPostTransfer(t *testing.T) {
	accounts := transferAccounts()
	tests := []struct {
		name string
		in   TransferInput
		want bool
	}{
		{"valid same-currency transfer", TransferInput{FromID: "chk", ToID: "sav", Amount: "25.00"}, true},
		{"fee and received left blank", TransferInput{FromID: "chk", ToID: "travel", Amount: "10", Received: "", Fee: ""}, true},
		{"zero fee is clean", TransferInput{FromID: "chk", ToID: "sav", Amount: "10", Fee: "0"}, true},
		{"fee given", TransferInput{FromID: "chk", ToID: "sav", Amount: "10", Fee: "1.50"}, true},
		{"no source", TransferInput{ToID: "sav", Amount: "10"}, false},
		{"no destination", TransferInput{FromID: "chk", Amount: "10"}, false},
		{"same account", TransferInput{FromID: "chk", ToID: "chk", Amount: "10"}, false},
		{"blank amount", TransferInput{FromID: "chk", ToID: "sav"}, false},
		{"zero amount", TransferInput{FromID: "chk", ToID: "sav", Amount: "0"}, false},
		{"negative amount", TransferInput{FromID: "chk", ToID: "sav", Amount: "-5"}, false},
		{"unparseable amount", TransferInput{FromID: "chk", ToID: "sav", Amount: "ten"}, false},
		{"negative fee", TransferInput{FromID: "chk", ToID: "sav", Amount: "10", Fee: "-1"}, false},
		{"unparseable received", TransferInput{FromID: "chk", ToID: "travel", Amount: "10", Received: "x"}, false},
		{"bad date does not block", TransferInput{FromID: "chk", ToID: "sav", Amount: "10", Date: "not a date"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanPostTransfer(accounts, tt.in); got != tt.want {
				t.Fatalf("CanPostTransfer(%+v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsCrossCurrency(t *testing.T) {
	accounts := transferAccounts()
	tests := []struct {
		name         string
		fromID, toID string
		want         bool
	}{
		{"same currency", "chk", "sav", false},
		{"different currencies", "chk", "travel", true},
		{"no destination yet", "chk", "", false},
		{"no source yet", "", "travel", false},
		{"unknown account", "chk", "missing", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCrossCurrency(accounts, tt.fromID, tt.toID); got != tt.want {
				t.Fatalf("IsCrossCurrency(%q, %q) = %v, want %v", tt.fromID, tt.toID, got, tt.want)
			}
		})
	}
}

func TestHasTransferPair(t *testing.T) {
	cash := func(id string) domain.Account {
		return domain.Account{ID: id, Name: id, Type: domain.TypeChecking, Class: domain.ClassAsset, Currency: "USD"}
	}
	valuation := func(id string, typ domain.AccountType) domain.Account {
		return domain.Account{ID: id, Name: id, Type: typ, Class: domain.ClassAsset, Currency: "USD"}
	}
	archived := cash("old")
	archived.Archived = true
	tests := []struct {
		name     string
		accounts []domain.Account
		want     bool
	}{
		{"no accounts", nil, false},
		{"one cash account", []domain.Account{cash("a")}, false},
		{"two cash accounts", []domain.Account{cash("a"), cash("b")}, true},
		{"two properties", []domain.Account{valuation("h1", domain.TypeProperty), valuation("h2", domain.TypeProperty)}, false},
		{"property and vehicle", []domain.Account{valuation("h", domain.TypeProperty), valuation("v", domain.TypeVehicle)}, false},
		{"cash and property", []domain.Account{cash("a"), valuation("h", domain.TypeProperty)}, false},
		{"cash and archived cash", []domain.Account{cash("a"), archived}, false},
		{"cash and a debt", []domain.Account{cash("a"), {ID: "cc", Name: "cc", Type: domain.TypeCreditCard, Class: domain.ClassLiability, Currency: "USD"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasTransferPair(tt.accounts); got != tt.want {
				t.Fatalf("HasTransferPair = %v, want %v", got, tt.want)
			}
		})
	}
}
