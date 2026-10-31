// SPDX-License-Identifier: MIT

package reconcile

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestDisplayBalance(t *testing.T) {
	usd := func(n int64) money.Money { return money.New(n, "USD") }
	card := func(opening int64) domain.Account {
		return domain.Account{ID: "c", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: usd(opening)}
	}
	tests := []struct {
		name            string
		acct            domain.Account
		balance, clear  money.Money
		wantBal, wantCl money.Money
		wantShow        bool
	}{
		{
			name:    "asset keeps stored sign and shows differing cleared",
			acct:    domain.Account{Class: domain.ClassAsset, Type: domain.TypeChecking},
			balance: usd(10000), clear: usd(9000),
			wantBal: usd(10000), wantCl: usd(9000), wantShow: true,
		},
		{
			name:    "asset hides cleared when equal",
			acct:    domain.Account{Class: domain.ClassAsset, Type: domain.TypeChecking},
			balance: usd(10000), clear: usd(10000),
			wantBal: usd(10000), wantCl: usd(10000), wantShow: false,
		},
		{
			name:    "valuation type that is not reconcilable hides cleared",
			acct:    domain.Account{Class: domain.ClassAsset, Type: domain.TypeProperty},
			balance: usd(30000000), clear: usd(0),
			wantBal: usd(30000000), wantCl: usd(0), wantShow: false,
		},
		{
			name:    "positive-owed liability is shown as a negative figure",
			acct:    card(50000),
			balance: usd(60000), clear: usd(55000),
			wantBal: usd(-60000), wantCl: usd(-55000), wantShow: true,
		},
		{
			name:    "negative-owed liability keeps its stored sign",
			acct:    card(-50000),
			balance: usd(-60000), clear: usd(-60000),
			wantBal: usd(-60000), wantCl: usd(-60000), wantShow: false,
		},
		{
			name:    "overpaid positive-owed card states as a positive figure",
			acct:    card(50000),
			balance: usd(-2000), clear: usd(-2000),
			wantBal: usd(2000), wantCl: usd(2000), wantShow: false,
		},
		{
			name:    "utility shell hides cleared even when it differs",
			acct:    domain.Account{Class: domain.ClassLiability, Type: domain.TypeUtilities, Currency: "USD"},
			balance: usd(0), clear: usd(30000),
			wantBal: usd(0), wantCl: usd(-30000), wantShow: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DisplayBalance(tc.acct, tc.balance, tc.clear)
			if got.Balance != tc.wantBal || got.Cleared != tc.wantCl || got.ShowCleared != tc.wantShow {
				t.Fatalf("DisplayBalance = %+v, want balance %v cleared %v show %v", got, tc.wantBal, tc.wantCl, tc.wantShow)
			}
		})
	}
}

func TestOwedMinorAndStatedOwed(t *testing.T) {
	cases := []struct {
		name       string
		stated     int64
		owed       int64
		statedBack int64
	}{
		{"debt owes its magnitude", -4200, 4200, -4200},
		{"zero owes nothing", 0, 0, 0},
		{"held money owes nothing", 900, 0, 0},
		{"min int64 saturates", math.MinInt64, math.MaxInt64, -math.MaxInt64},
	}
	for _, c := range cases {
		if got := OwedMinor(c.stated); got != c.owed {
			t.Errorf("%s: OwedMinor(%d) = %d, want %d", c.name, c.stated, got, c.owed)
		}
		if got := StatedOwed(c.owed); got != c.statedBack {
			t.Errorf("%s: StatedOwed(%d) = %d, want %d", c.name, c.owed, got, c.statedBack)
		}
	}
	if got := StatedOwed(5000); got != -5000 {
		t.Errorf("StatedOwed(5000) = %d, want -5000", got)
	}
	if got := StatedOwed(math.MinInt64); got != math.MinInt64 {
		t.Errorf("StatedOwed(MinInt64) = %d, want MinInt64", got)
	}
}
