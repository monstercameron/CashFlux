// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestG13Windfall(t *testing.T) {
	in := baseInput().withBaseline(500000, 300000) // $5000/mo avg income
	// A recent $8,000 deposit — 1.6× the monthly average → a windfall.
	in.Transactions = append(in.Transactions,
		domain.Transaction{ID: "bonus", AccountID: "x", Date: ref.AddDate(0, 0, -10), Amount: usd(800000), Desc: "Bonus"})
	got := g13Windfall(in)
	if len(got) != 1 {
		t.Fatalf("want 1 windfall, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 800000 {
		t.Errorf("windfall amount = %d, want 800000", got[0].Amount.Amount)
	}
}

func TestG13NoWindfallForRegularIncome(t *testing.T) {
	in := baseInput().withBaseline(500000, 300000)
	// A normal $5,000 paycheck is not a windfall.
	in.Transactions = append(in.Transactions,
		domain.Transaction{ID: "pay", AccountID: "x", Date: ref.AddDate(0, 0, -5), Amount: usd(500000), Desc: "Pay"})
	if got := g13Windfall(in); len(got) != 0 {
		t.Errorf("regular income — want 0, got %d: %+v", len(got), got)
	}
}

func liabilityCardAPR(id string, dueDay int, openingOwed int64, apr float64) domain.Account {
	return domain.Account{
		ID: id, Name: "Visa", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		Currency: "USD", DueDayOfMonth: dueDay, MinPayment: usd(2500),
		OpeningBalance: usd(openingOwed), InterestRateAPR: apr,
	}
}

func TestBL6LateFeeRisk(t *testing.T) {
	in := baseInput() // now June 15
	in.Accounts = []domain.Account{liabilityCardAPR("c", 18, -200000, 22.0)} // due the 18th (3 days), owes $2000
	got := bl6LateFeeRisk(in)
	if len(got) != 1 {
		t.Fatalf("want 1 late-fee warning, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount <= 0 {
		t.Errorf("expected a positive interest estimate, got %+v", got[0].Amount)
	}
}

func TestBL6SkipsDistantDue(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{liabilityCardAPR("c", 28, -200000, 22.0)} // due the 28th → 13 days out
	if got := bl6LateFeeRisk(in); len(got) != 0 {
		t.Errorf("distant due date — want 0, got %d", len(got))
	}
}

func TestSU3TrialConversion(t *testing.T) {
	in := baseInput() // now June 15
	in.Transactions = []domain.Transaction{
		{ID: "trial", AccountID: "x", Date: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), Amount: usd(-99), Desc: "Hulu"},  // $0.99 intro
		{ID: "real", AccountID: "x", Date: time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-1799), Desc: "Hulu"}, // first real charge
	}
	got := su3TrialConversion(in)
	if len(got) != 1 {
		t.Fatalf("want 1 conversion warning, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 1799 {
		t.Errorf("charge amount = %d, want 1799", got[0].Amount.Amount)
	}
}

func TestSU3NoIntroNoWarning(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		{ID: "r1", AccountID: "x", Date: time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-1799), Desc: "Hulu"},
	}
	if got := su3TrialConversion(in); len(got) != 0 {
		t.Errorf("no intro charge — want 0, got %d", len(got))
	}
}
