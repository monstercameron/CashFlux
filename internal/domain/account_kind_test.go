// SPDX-License-Identifier: MIT

package domain_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestAccountTypeIsValuation(t *testing.T) {
	want := map[domain.AccountType]bool{
		domain.TypeInvestment: true,
		domain.TypeRetirement: true,
		domain.TypeCrypto:     true,
		domain.TypeProperty:   true,
		domain.TypeVehicle:    true,
		domain.TypeOther:      true,
	}
	for _, typ := range domain.AllAccountTypes {
		if got := typ.IsValuation(); got != want[typ] {
			t.Errorf("%s.IsValuation() = %v, want %v", typ, got, want[typ])
		}
	}
}

func TestAccountShowsCashCurve(t *testing.T) {
	tests := []struct {
		name string
		acct domain.Account
		want bool
	}{
		{"checking asset", domain.Account{Class: domain.ClassAsset, Type: domain.TypeChecking}, true},
		{"savings asset", domain.Account{Class: domain.ClassAsset, Type: domain.TypeSavings}, true},
		{"credit card liability", domain.Account{Class: domain.ClassLiability, Type: domain.TypeCreditCard}, false},
		{"investment valuation", domain.Account{Class: domain.ClassAsset, Type: domain.TypeInvestment}, false},
		{"property valuation", domain.Account{Class: domain.ClassAsset, Type: domain.TypeProperty}, false},
		{"other flagged as liability", domain.Account{Class: domain.ClassLiability, Type: domain.TypeOther}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.acct.ShowsCashCurve(); got != tc.want {
				t.Fatalf("ShowsCashCurve() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAccountIsShared(t *testing.T) {
	tests := []struct {
		name string
		acct domain.Account
		want bool
	}{
		{"individual", domain.Account{OwnerID: "m1", Scope: domain.ScopeIndividual}, false},
		{"shared scope", domain.Account{OwnerID: "m1", Scope: domain.ScopeShared}, true},
		{"group owned", domain.Account{OwnerID: domain.GroupOwnerID}, true},
		{"ownership shares", domain.Account{OwnerID: "m1", OwnershipShares: map[string]int{"m1": 50, "m2": 50}}, true},
		{"empty shares map", domain.Account{OwnerID: "m1", OwnershipShares: map[string]int{}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.acct.IsShared(); got != tc.want {
				t.Fatalf("IsShared() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOwnerName(t *testing.T) {
	members := []domain.Member{{ID: "m1", Name: "Ana"}, {ID: "m2", Name: "Ben"}}
	tests := []struct {
		name    string
		ownerID string
		want    string
	}{
		{"known member", "m2", "Ben"},
		{"empty id", "", ""},
		{"group owner", domain.GroupOwnerID, ""},
		{"unknown id", "m9", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.OwnerName(members, tc.ownerID); got != tc.want {
				t.Fatalf("OwnerName(%q) = %q, want %q", tc.ownerID, got, tc.want)
			}
		})
	}
}

func TestNormalizeInstitution(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"blank", "   ", ""},
		{"lower case words", "bmo harris", "Bmo Harris"},
		{"keeps typed capitals", "BMO", "BMO"},
		{"trims and collapses space", "  first   national  ", "First National"},
		{"non ascii first letter", "élan financial", "Élan Financial"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.NormalizeInstitution(tc.in); got != tc.want {
				t.Fatalf("NormalizeInstitution(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNameLess(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"lower before capital later letter", "alpha", "Beta", true},
		{"capital before lower later letter", "Alpha", "beta", true},
		{"same name differing case", "alpha", "Alpha", false},
		{"later letter", "Beta", "alpha", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.NameLess(tc.a, tc.b); got != tc.want {
				t.Fatalf("NameLess(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
