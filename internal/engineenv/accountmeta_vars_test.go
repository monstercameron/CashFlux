// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestCarryingCostVars checks the AC4 carrying-cost atoms: per-debt debt_*_carry and
// the household interest_drag_monthly total.
func TestCarryingCostVars(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d := Data{
		Accounts: []domain.Account{
			{ID: "visa", Name: "Visa", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
				Currency: "USD", OpeningBalance: usd(-6000), InterestRateAPR: 24},
		},
		Rates: currency.Rates{Base: "USD"},
		Now:   now,
	}
	v := Vars(d)
	// 6000 owed at 24% APR → 6000 * 0.24 / 12 = 120/month.
	if got := v["debt_visa_carry"]; got != 120 {
		t.Errorf("debt_visa_carry = %v, want 120", got)
	}
	if got := v["interest_drag_monthly"]; got != 120 {
		t.Errorf("interest_drag_monthly = %v, want 120", got)
	}
}

// TestExcludeFromNetWorthVar checks the AC11 flag drops an account from the
// net-worth atoms without affecting other accounts.
func TestExcludeFromNetWorthVar(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	base := Data{
		Accounts: []domain.Account{
			{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(1000)},
			{ID: "cust", Name: "Custodial", Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: usd(500)},
		},
		Rates: currency.Rates{Base: "USD"},
		Now:   now,
	}
	before := Vars(base)["assets"]
	base.Accounts[1].ExcludeFromNetWorth = true
	after := Vars(base)["assets"]
	if before != 1500 {
		t.Fatalf("assets before = %v, want 1500", before)
	}
	if after != 1000 {
		t.Errorf("assets after exclude = %v, want 1000 (custodial dropped)", after)
	}
}

// TestIdleCashVars checks the AC15 idle-cash atoms.
func TestIdleCashVars(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	d := Data{
		Accounts: []domain.Account{
			{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(12000)},
		},
		Rates:                   currency.Rates{Base: "USD"},
		Now:                     now,
		IdleBenchmarkAPRPercent: 4,
	}
	v := Vars(d)
	// No bills or goals → committed 0 → idle = 12000; forgone = 12000 * 4% = 480/yr.
	if v["idle_cash"] != 12000 {
		t.Errorf("idle_cash = %v, want 12000", v["idle_cash"])
	}
	if v["idle_cash_forgone_annual"] != 480 {
		t.Errorf("idle_cash_forgone_annual = %v, want 480", v["idle_cash_forgone_annual"])
	}
	if v["idle_cash_benchmark"] != 4 {
		t.Errorf("idle_cash_benchmark = %v, want 4", v["idle_cash_benchmark"])
	}
}
