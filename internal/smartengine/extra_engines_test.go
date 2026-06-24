// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
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
	in := baseInput()                                                        // now June 15
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

func TestG12SuggestEmergencyFund(t *testing.T) {
	in := baseInput().withBaseline(0, 200000) // $2000/mo essentials
	got := g12SuggestGoals(in)
	if len(got) != 1 {
		t.Fatalf("want 1 suggestion, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 600000 { // 3 × $2000
		t.Errorf("target = %d, want 600000", got[0].Amount.Amount)
	}
}

func TestG12SkipsWhenFundExists(t *testing.T) {
	in := baseInput().withBaseline(0, 200000)
	in.Goals = []domain.Goal{goal("ef", "Emergency Fund", 600000, 100000, time.Time{})}
	if got := g12SuggestGoals(in); len(got) != 0 {
		t.Errorf("already has a fund — want 0, got %d", len(got))
	}
}

func TestG18FeasibilityRed(t *testing.T) {
	in := baseInput().withBaseline(400000, 380000) // $200/mo surplus
	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{goal("g", "Car", 300000, 0, due)} // needs ~$1000/mo
	got := g18Feasibility(in)
	if len(got) != 1 {
		t.Fatalf("want 1 at-risk goal, got %d: %+v", len(got), got)
	}
	if got[0].Severity != smart.SeverityWarn {
		t.Errorf("at-risk goal should warn, got %v", got[0].Severity)
	}
}

func TestG18FeasibilityGreen(t *testing.T) {
	in := baseInput().withBaseline(800000, 100000) // $7000/mo surplus
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{goal("g", "Trip", 60000, 0, due)} // tiny need
	if got := g18Feasibility(in); len(got) != 0 {
		t.Errorf("comfortably affordable — want 0, got %d: %+v", len(got), got)
	}
}

func TestT11Timeline(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		{ID: "small", AccountID: "x", Date: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), Amount: usd(-5000), Desc: "Lunch"},
		{ID: "big", AccountID: "x", Date: time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), Amount: usd(-30000), Desc: "Flight"},
	}
	got := t11Timeline(in)
	if len(got) != 1 {
		t.Fatalf("want 1 annotation, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 30000 {
		t.Errorf("biggest = %d, want 30000", got[0].Amount.Amount)
	}
}

func TestBL13StatementClarity(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{liabilityCardAPR("c", 18, -200000, 22.0)} // owes $2000, min $25, 22% APR
	got := bl13StatementClarity(in)
	if len(got) != 1 {
		t.Fatalf("want 1 statement-clarity insight, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount <= 0 {
		t.Errorf("expected a positive monthly-interest figure, got %+v", got[0].Amount)
	}
}

func TestBL13SkipsClearedCard(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{liabilityCardAPR("c", 18, -1000, 22.0)} // owes $10, min $25 → minimum clears it
	if got := bl13StatementClarity(in); len(got) != 0 {
		t.Errorf("minimum clears the balance — want 0, got %d", len(got))
	}
}

func TestG8GoalImpact(t *testing.T) {
	in := baseInput()
	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{goal("g", "Vacation", 60000, 0, due)} // needs ~$200/mo
	in.Transactions = []domain.Transaction{
		{ID: "buy", AccountID: "x", Date: time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), Amount: usd(-30000), Desc: "TV"},
	}
	got := g8GoalImpact(in)
	if len(got) != 1 {
		t.Fatalf("want 1 impact insight, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 30000 {
		t.Errorf("expense amount = %d, want 30000", got[0].Amount.Amount)
	}
}

func TestP8ExtraDebt(t *testing.T) {
	in := baseInput().withBaseline(500000, 300000) // $2000/mo surplus
	card := domain.Account{
		ID: "c", Name: "Visa", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		Currency: "USD", InterestRateAPR: 20.0, OpeningBalance: usd(-500000), MinPayment: usd(20000), DueDayOfMonth: 18,
	}
	in.Accounts = []domain.Account{card}
	got := p8ExtraDebt(in)
	if len(got) != 1 {
		t.Fatalf("want 1 extra-payment suggestion, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount <= 0 {
		t.Errorf("expected a positive extra amount, got %+v", got[0].Amount)
	}
}

func TestP8NoSurplusNoSuggestion(t *testing.T) {
	in := baseInput().withBaseline(300000, 500000) // negative surplus
	card := domain.Account{
		ID: "c", Name: "Visa", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		Currency: "USD", InterestRateAPR: 20.0, OpeningBalance: usd(-500000), MinPayment: usd(20000),
	}
	in.Accounts = []domain.Account{card}
	if got := p8ExtraDebt(in); len(got) != 0 {
		t.Errorf("no surplus — want 0, got %d", len(got))
	}
}
