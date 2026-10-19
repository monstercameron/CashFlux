// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestQuickAddOwnerSelectionDefaultsAndExplicitUnassigned(t *testing.T) {
	individual := domain.Account{Scope: domain.ScopeIndividual, OwnerID: "m2"}
	cases := []struct {
		name       string
		owners     []string
		active     string
		account    domain.Account
		touched    bool
		wantOwners []string
	}{
		{name: "active member", active: "m1", account: individual, wantOwners: []string{"m1"}},
		{name: "individual account owner", account: individual, wantOwners: []string{"m2"}},
		{name: "shared account stays unassigned", wantOwners: nil},
		{name: "explicit clear means unassigned", owners: []string{}, account: individual, touched: true, wantOwners: nil},
		{name: "explicit shared owners", owners: []string{"m1", "m2"}, account: individual, touched: true, wantOwners: []string{"m1", "m2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := quickAddOwnerSelection(tc.owners, tc.active, tc.account, tc.touched); !reflect.DeepEqual(got, tc.wantOwners) {
				t.Fatalf("owner selection = %#v, want %#v", got, tc.wantOwners)
			}
		})
	}
}
