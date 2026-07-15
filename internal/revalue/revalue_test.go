// SPDX-License-Identifier: MIT

package revalue

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
)

func windows() freshness.Windows {
	return freshness.DefaultWindows().Merge(DefaultCadences())
}

func TestIsRevaluable(t *testing.T) {
	for _, ty := range []domain.AccountType{domain.TypeProperty, domain.TypeVehicle, domain.TypeCrypto} {
		if !IsRevaluable(ty) {
			t.Errorf("%s should be revaluable", ty)
		}
	}
	for _, ty := range []domain.AccountType{domain.TypeChecking, domain.TypeSavings, domain.TypeInvestment} {
		if IsRevaluable(ty) {
			t.Errorf("%s should not be revaluable", ty)
		}
	}
}

func TestDefaultCadences(t *testing.T) {
	w := windows()
	cases := map[domain.AccountType]int{
		domain.TypeProperty: PropertyDays,
		domain.TypeVehicle:  VehicleDays,
		domain.TypeCrypto:   CryptoDays,
	}
	for ty, want := range cases {
		if got, _ := w.WindowDays(ty); got != want {
			t.Errorf("%s window = %d, want %d", ty, got, want)
		}
	}
}

func TestCadenceDaysOverrideWins(t *testing.T) {
	w := windows()
	a := domain.Account{Type: domain.TypeProperty, RevalueDays: 30}
	if got, ok := CadenceDays(a, w); !ok || got != 30 {
		t.Errorf("override CadenceDays = %d,%v want 30,true", got, ok)
	}
	a.RevalueDays = 0
	if got, ok := CadenceDays(a, w); !ok || got != PropertyDays {
		t.Errorf("default CadenceDays = %d,%v want %d,true", got, ok, PropertyDays)
	}
}

func TestIsDue(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	w := windows()
	tests := []struct {
		name string
		acc  domain.Account
		want bool
	}{
		{"property fresh", domain.Account{Type: domain.TypeProperty, BalanceAsOf: now.AddDate(0, 0, -30)}, false},
		{"property stale past quarter", domain.Account{Type: domain.TypeProperty, BalanceAsOf: now.AddDate(0, 0, -100)}, true},
		{"crypto stale past week", domain.Account{Type: domain.TypeCrypto, BalanceAsOf: now.AddDate(0, 0, -10)}, true},
		{"crypto fresh within week", domain.Account{Type: domain.TypeCrypto, BalanceAsOf: now.AddDate(0, 0, -3)}, false},
		{"override tightens property", domain.Account{Type: domain.TypeProperty, RevalueDays: 7, BalanceAsOf: now.AddDate(0, 0, -30)}, true},
		{"never confirmed is due", domain.Account{Type: domain.TypeVehicle}, true},
		{"archived never due", domain.Account{Type: domain.TypeProperty, Archived: true, BalanceAsOf: now.AddDate(0, 0, -400)}, false},
	}
	for _, tc := range tests {
		if got := IsDue(tc.acc, w, now); got != tc.want {
			t.Errorf("%s: IsDue = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNextDue(t *testing.T) {
	w := windows()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := domain.Account{Type: domain.TypeProperty, BalanceAsOf: base}
	got, ok := NextDue(a, w)
	if !ok {
		t.Fatal("expected a finite cadence")
	}
	if want := base.AddDate(0, 0, PropertyDays); !got.Equal(want) {
		t.Errorf("NextDue = %v, want %v", got, want)
	}
	if _, ok := NextDue(domain.Account{Type: domain.TypeProperty}, w); !ok {
		t.Error("never-confirmed revaluable account should still report a finite cadence")
	}
}
