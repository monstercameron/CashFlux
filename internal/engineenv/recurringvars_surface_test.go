// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddRecurringVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	recs := []domain.Recurring{
		{ID: "r1", Label: "Paycheck", Amount: money.New(200000, "USD"), Cadence: domain.CadenceBiweekly, NextDue: now},
		{ID: "r2", Label: "Gym membership", Amount: money.New(-5000, "USD"), Cadence: domain.CadenceMonthly, NextDue: now},
	}
	vars := Vars(Data{Recurring: recs, Rates: currency.Rates{Base: "USD"}, Now: now})

	// Biweekly $2,000 → monthly equivalent per domain.MonthlyEquivalent (26/12 pay periods).
	wantIn := float64(recs[0].MonthlyEquivalent()) / 100
	if got := vars["recurring_monthly_in"]; got != wantIn {
		t.Errorf("recurring_monthly_in = %v, want %v", got, wantIn)
	}
	if got := vars["recurring_monthly_out"]; got != 50 {
		t.Errorf("recurring_monthly_out = %v, want 50", got)
	}
	if got := vars["recurring_monthly_net"]; got != wantIn-50 {
		t.Errorf("recurring_monthly_net = %v, want %v", got, wantIn-50)
	}
	if got := vars["recurring_count"]; got != 2 {
		t.Errorf("recurring_count = %v, want 2", got)
	}
	// Per-flow identity: the gym flow is addressable by its slug.
	if got := vars["recurring_gym_membership_monthly"]; got != -50 {
		t.Errorf("recurring_gym_membership_monthly = %v, want -50", got)
	}
	if got := vars["recurring_gym_membership_amount"]; got != -50 {
		t.Errorf("recurring_gym_membership_amount = %v, want -50", got)
	}
}

func TestRecurringVarBasesCollision(t *testing.T) {
	bases := RecurringVarBases([]domain.Recurring{{Label: "Rent"}, {Label: "Rent"}})
	if len(bases) != 2 || bases[0].Prefix != "recurring_rent_" || bases[1].Prefix != "recurring_rent_2_" {
		t.Errorf("collision handling wrong: %+v", bases)
	}
}
