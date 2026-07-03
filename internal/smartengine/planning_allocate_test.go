// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestP10BillShock(t *testing.T) {
	in := baseInput()
	in.Recurring = []domain.Recurring{{
		ID: "ins", Label: "Car Insurance", Amount: usd(-120000), Cadence: domain.CadenceYearly,
		NextDue: ref.AddDate(0, 0, 50),
	}}
	got := p10BillShock(in)
	if len(got) != 1 {
		t.Fatalf("want 1 bill-shock warning, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 120000 {
		t.Errorf("charge amount = %d, want 120000", got[0].Amount.Amount)
	}
}

func TestP10SkipsDistantAndMonthly(t *testing.T) {
	in := baseInput()
	in.Recurring = []domain.Recurring{
		{ID: "far", Label: "Taxes", Amount: usd(-200000), Cadence: domain.CadenceYearly, NextDue: ref.AddDate(0, 0, 200)},
		{ID: "rent", Label: "Rent", Amount: usd(-150000), Cadence: domain.CadenceMonthly, NextDue: ref.AddDate(0, 0, 5)},
	}
	if got := p10BillShock(in); len(got) != 0 {
		t.Errorf("distant/monthly — want 0, got %d: %+v", len(got), got)
	}
}

func TestAL1SuggestsDebtForHighAPR(t *testing.T) {
	in := baseInput()
	card := domain.Account{
		ID: "c", Name: "Visa", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		Currency: "USD", InterestRateAPR: 22.0, OpeningBalance: usd(-50000),
	}
	in.Accounts = []domain.Account{card}
	got := al1SuggestedProfile(in)
	if len(got) != 1 || got[0].Key != "SMART-AL1:debt" {
		t.Fatalf("want debt profile, got %+v", got)
	}
}

func TestAL1BalancedWhenSteady(t *testing.T) {
	in := baseInput()
	// A steady situation still requires SOME data — an account to read.
	in.Accounts = []domain.Account{acct("chk", "Checking", domain.TypeChecking, 300000, ref)}
	got := al1SuggestedProfile(in)
	if len(got) != 1 || got[0].Key != "SMART-AL1:balanced" {
		t.Fatalf("want balanced profile, got %+v", got)
	}
}

func TestAL1EmptyDatasetSaysNothing(t *testing.T) {
	// A brand-new dataset (no accounts) has no situation to read — "your
	// finances look steady" would be a non-fact (C356).
	in := baseInput()
	if got := al1SuggestedProfile(in); len(got) != 0 {
		t.Fatalf("empty dataset should produce no profile suggestion, got %+v", got)
	}
}

func TestAL1SafetyForThinEmergency(t *testing.T) {
	in := baseInput().withBaseline(0, 200000) // $2000/mo essentials
	in.Accounts = []domain.Account{acct("chk", "Checking", domain.TypeChecking, 300000, ref)}
	in.Goals = []domain.Goal{goal("ef", "Emergency Fund", 1200000, 100000, time.Time{})} // 0.5 months
	got := al1SuggestedProfile(in)
	if len(got) != 1 || got[0].Key != "SMART-AL1:safety" {
		t.Fatalf("want safety profile, got %+v", got)
	}
}

func TestAL3SmartReserve(t *testing.T) {
	in := baseInput().withBaseline(0, 200000) // $2000/mo essentials
	got := al3SmartReserve(in)
	if len(got) != 1 {
		t.Fatalf("want 1 reserve suggestion, got %d", len(got))
	}
	if got[0].Amount.Amount != 1200000 { // 6 × $2000
		t.Errorf("reserve = %d, want 1200000", got[0].Amount.Amount)
	}
}

func TestAL3NoSpendNoSuggestion(t *testing.T) {
	if got := al3SmartReserve(baseInput()); len(got) != 0 {
		t.Errorf("no spend history — want 0, got %d", len(got))
	}
}
