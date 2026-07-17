// SPDX-License-Identifier: MIT

package liquidity

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

var now = time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)

func acct(t domain.AccountType) domain.Account {
	return domain.Account{ID: string(t), Type: t, Class: domain.ClassAsset}
}

func TestOf(t *testing.T) {
	cases := []struct {
		typ  domain.AccountType
		want Class
	}{
		{domain.TypeChecking, Available},
		{domain.TypeDebit, Available},
		{domain.TypeCash, Available},
		{domain.TypeSavings, Available},
		{domain.TypeRetirement, Restricted},
		{domain.TypeInvestment, Investments},
		{domain.TypeCrypto, Investments},
		{domain.TypeProperty, Held},
		{domain.TypeVehicle, Held},
		{domain.TypeOther, Held},
	}
	for _, tc := range cases {
		if got := Of(acct(tc.typ), now); got != tc.want {
			t.Errorf("Of(%s) = %s, want %s", tc.typ, got, tc.want)
		}
	}
}

func TestOfLockedIsRestricted(t *testing.T) {
	locked := acct(domain.TypeSavings)
	locked.LockUntil = now.AddDate(0, 1, 0)
	if got := Of(locked, now); got != Restricted {
		t.Errorf("future-locked savings = %s, want restricted", got)
	}
	// An expired lock is back to its type's class.
	locked.LockUntil = now.AddDate(0, -1, 0)
	if got := Of(locked, now); got != Available {
		t.Errorf("expired-lock savings = %s, want available", got)
	}
}

func TestTotals(t *testing.T) {
	chk := acct(domain.TypeChecking)
	sav := acct(domain.TypeSavings)
	inv := acct(domain.TypeInvestment)
	prop := acct(domain.TypeProperty)
	arch := acct(domain.TypeCash)
	arch.Archived = true
	liab := domain.Account{ID: "loan", Type: domain.TypeLoan, Class: domain.ClassLiability}
	norate := acct(domain.TypeDebit)
	norate.ID = "norate"

	vals := map[string]int64{"checking": 100, "savings": 50, "investment": 200, "property": 900, "cash": 999, "loan": -500}
	conv := func(a domain.Account) (int64, bool) {
		if a.ID == "norate" {
			return 0, false
		}
		return vals[a.ID], true
	}
	got := Totals([]domain.Account{chk, sav, inv, prop, arch, liab, norate}, now, conv)
	if got[Available] != 150 {
		t.Errorf("available = %d, want 150", got[Available])
	}
	if got[Investments] != 200 {
		t.Errorf("investments = %d, want 200", got[Investments])
	}
	if got[Held] != 900 {
		t.Errorf("held = %d, want 900", got[Held])
	}
	if got[Restricted] != 0 {
		t.Errorf("restricted = %d, want 0", got[Restricted])
	}
}
