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

func TestP1DiscoverRecurring(t *testing.T) {
	in := baseInput()
	txns := monthlyCharges("Netflix", -1599, time.June, 4)
	txns = append(txns, monthlyCharges("Gym", -3000, time.June, 4)...)
	in.Transactions = txns
	got := p1DiscoverRecurring(in)
	if len(got) != 1 {
		t.Fatalf("want 1 discovery insight, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount <= 0 {
		t.Errorf("expected a monthly total, got %+v", got[0].Amount)
	}
}

func TestP1SkipsAlreadyTracked(t *testing.T) {
	in := baseInput()
	txns := monthlyCharges("Netflix", -1599, time.June, 4)
	txns = append(txns, monthlyCharges("Gym", -3000, time.June, 4)...)
	in.Transactions = txns
	in.Recurring = []domain.Recurring{
		{ID: "1", Label: "Netflix", Amount: usd(-1599), Cadence: domain.CadenceMonthly, NextDue: ref},
		{ID: "2", Label: "Gym", Amount: usd(-3000), Cadence: domain.CadenceMonthly, NextDue: ref},
	}
	if got := p1DiscoverRecurring(in); len(got) != 0 {
		t.Errorf("all tracked — want 0, got %d: %+v", len(got), got)
	}
}

func TestG15DebtStrategy(t *testing.T) {
	in := baseInput()
	hi := domain.Account{ID: "hi", Name: "Visa", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		Currency: "USD", InterestRateAPR: 24.0, OpeningBalance: usd(-500000), MinPayment: usd(10000)}
	lo := domain.Account{ID: "lo", Name: "Car Loan", Type: domain.TypeLoan, Class: domain.ClassLiability,
		Currency: "USD", InterestRateAPR: 5.0, OpeningBalance: usd(-300000), MinPayment: usd(10000)}
	in.Accounts = []domain.Account{hi, lo}
	got := g15DebtStrategy(in)
	if len(got) != 1 {
		t.Fatalf("want 1 strategy insight, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount <= 0 {
		t.Errorf("expected positive interest saved, got %+v", got[0].Amount)
	}
}

func TestG15SingleDebtNoComparison(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{liabilityCardAPR("c", 18, -500000, 24.0)}
	if got := g15DebtStrategy(in); len(got) != 0 {
		t.Errorf("one debt — want 0, got %d", len(got))
	}
}

func TestSU11Zombie(t *testing.T) {
	in := baseInput()
	in.Transactions = monthlyCharges("CloudBackup", -500, time.June, 7) // $5/mo, 7 periods
	got := su11Zombie(in)
	if len(got) != 1 {
		t.Fatalf("want 1 zombie insight, got %d: %+v", len(got), got)
	}
}

func TestSU11SkipsLargeOrShort(t *testing.T) {
	in := baseInput()
	in.Transactions = monthlyCharges("Netflix", -1599, time.June, 7) // $16/mo > $10 floor
	if got := su11Zombie(in); len(got) != 0 {
		t.Errorf("large charge — want 0, got %d", len(got))
	}
}

func TestG3AllocateSurplus(t *testing.T) {
	in := baseInput().withBaseline(500000, 300000) // $2000/mo surplus
	in.Goals = []domain.Goal{goal("g", "Vacation", 100000, 0, time.Time{})}
	got := g3AllocateSurplus(in)
	if len(got) != 1 {
		t.Fatalf("want 1 surplus nudge, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 200000 {
		t.Errorf("surplus = %d, want 200000", got[0].Amount.Amount)
	}
}

func TestG3NoSurplusNoNudge(t *testing.T) {
	in := baseInput().withBaseline(300000, 500000) // negative surplus
	in.Goals = []domain.Goal{goal("g", "Vacation", 100000, 0, time.Time{})}
	if got := g3AllocateSurplus(in); len(got) != 0 {
		t.Errorf("negative surplus — want 0, got %d", len(got))
	}
}

func TestP6ConfidenceBand(t *testing.T) {
	in := baseInput()
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	in.Transactions = []domain.Transaction{
		{ID: "m1", AccountID: "x", Date: monthStart.AddDate(0, -1, 9), Amount: usd(300000), Desc: "Pay"},
		{ID: "m2", AccountID: "x", Date: monthStart.AddDate(0, -2, 9), Amount: usd(100000), Desc: "Pay"},
		{ID: "m3", AccountID: "x", Date: monthStart.AddDate(0, -3, 9), Amount: usd(500000), Desc: "Pay"},
	}
	got := p6ConfidenceBand(in)
	if len(got) != 1 {
		t.Fatalf("want 1 confidence band, got %d: %+v", len(got), got)
	}
	// Range $1000–$5000 → swing $2000.
	if got[0].Amount.Amount != 200000 {
		t.Errorf("swing = %d, want 200000", got[0].Amount.Amount)
	}
}

func TestP9BreakEven(t *testing.T) {
	in := baseInput().withBaseline(500000, 300000)
	got := p9BreakEven(in)
	if len(got) != 1 {
		t.Fatalf("want 1 break-even insight, got %d", len(got))
	}
	if got[0].Amount.Amount != 500000 {
		t.Errorf("break-even = %d, want 500000", got[0].Amount.Amount)
	}
}

func TestSU6CostCreep(t *testing.T) {
	in := baseInput()
	var txns []domain.Transaction
	for i := range 6 {
		d := time.Date(2026, time.Month(1+i), 8, 0, 0, 0, 0, time.UTC)
		amt := int64(-5000)
		if i >= 3 {
			amt = -6000
		}
		txns = append(txns, domain.Transaction{ID: "x" + itoa64(int64(i)), AccountID: "a", Date: d, Amount: usd(amt), Desc: "Internet"})
	}
	in.Transactions = txns
	if got := su6CostCreep(in); len(got) != 1 {
		t.Fatalf("want 1 cost-creep insight, got %d: %+v", len(got), got)
	}
}

func TestSU8Forgotten(t *testing.T) {
	in := baseInput()                                                   // now June 15
	in.Transactions = monthlyCharges("OldGym", -3000, time.February, 4) // last charge Feb → stale
	if got := su8Forgotten(in); len(got) != 1 {
		t.Fatalf("want 1 forgotten insight, got %d: %+v", len(got), got)
	}
}

func TestBL4Autopay(t *testing.T) {
	in := baseInput() // now June 15, prev due June 5
	in.Accounts = []domain.Account{liabilityCard("c", "Visa", 5, 5000)}
	in.Transactions = []domain.Transaction{
		txn("p", "c", time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC), 5000), // payment near due date
	}
	got := bl4Autopay(in)
	if len(got) != 1 {
		t.Fatalf("want 1 autopay insight, got %d: %+v", len(got), got)
	}
	if got[0].Severity != smart.SeverityInfo {
		t.Errorf("autopay should be info, got %v", got[0].Severity)
	}
}

func TestD1AutoTodos(t *testing.T) {
	in := baseInput()
	var txns []domain.Transaction
	for i := range 5 {
		txns = append(txns, domain.Transaction{ID: "u" + itoa64(int64(i)), AccountID: "x",
			Date: ref.AddDate(0, 0, -i), Amount: usd(-1000), Desc: "Misc"}) // no CategoryID
	}
	in.Transactions = txns
	got := d1AutoTodos(in)
	if len(got) != 1 {
		t.Fatalf("want 1 todo nudge, got %d: %+v", len(got), got)
	}
	if got[0].Action == nil || got[0].Action.Kind != smart.ActionCreateTask {
		t.Errorf("expected a create-task action")
	}
}

func TestD1SkipsWhenFewUncategorized(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		{ID: "u", AccountID: "x", Date: ref, Amount: usd(-1000), Desc: "Misc"},
	}
	if got := d1AutoTodos(in); len(got) != 0 {
		t.Errorf("below threshold — want 0, got %d", len(got))
	}
}

func TestP5GoalOverlay(t *testing.T) {
	in := baseInput().withBaseline(500000, 300000)
	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	in.Goals = []domain.Goal{goal("g", "Car", 300000, 0, due)} // needs ~$1000/mo
	got := p5GoalOverlay(in)
	if len(got) != 1 {
		t.Fatalf("want 1 overlay insight, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 100000 {
		t.Errorf("monthly need = %d, want 100000", got[0].Amount.Amount)
	}
}

func TestBL1PredictVariable(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		{ID: "e1", AccountID: "x", Date: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-12000), Desc: "Electric"},
		{ID: "e2", AccountID: "x", Date: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-14000), Desc: "Electric"},
		{ID: "e3", AccountID: "x", Date: time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-16000), Desc: "Electric"},
	}
	got := bl1PredictVariable(in)
	if len(got) != 1 {
		t.Fatalf("want 1 prediction, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 14000 { // mean of 120/140/160
		t.Errorf("predicted = %d, want 14000", got[0].Amount.Amount)
	}
}

func TestBL1SkipsFixedBill(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{
		{ID: "r1", AccountID: "x", Date: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-15000), Desc: "Rent"},
		{ID: "r2", AccountID: "x", Date: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-15000), Desc: "Rent"},
		{ID: "r3", AccountID: "x", Date: time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC), Amount: usd(-15000), Desc: "Rent"},
	}
	if got := bl1PredictVariable(in); len(got) != 0 {
		t.Errorf("fixed bill — want 0, got %d", len(got))
	}
}

func TestBL8PaycheckGrouping(t *testing.T) {
	in := baseInput() // now June 15
	in.Transactions = []domain.Transaction{
		{ID: "pay", AccountID: "x", Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Amount: usd(300000), Desc: "Salary"}, // payday = 1st
	}
	in.Recurring = []domain.Recurring{
		{ID: "r", Label: "Phone", Amount: usd(-8000), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)},
	}
	got := bl8PaycheckGrouping(in)
	if len(got) != 1 {
		t.Fatalf("want 1 paycheck-grouping insight, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 8000 {
		t.Errorf("total = %d, want 8000", got[0].Amount.Amount)
	}
}

func TestBL8NoIncomeNoInsight(t *testing.T) {
	in := baseInput()
	in.Recurring = []domain.Recurring{
		{ID: "r", Label: "Phone", Amount: usd(-8000), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)},
	}
	if got := bl8PaycheckGrouping(in); len(got) != 0 {
		t.Errorf("no income — want 0, got %d", len(got))
	}
}

func TestP4Affordability(t *testing.T) {
	in := baseInput().withBaseline(0, 250000) // $2500/mo essentials
	got := p4Affordability(in)
	if len(got) != 1 {
		t.Fatalf("want 1 buffer suggestion, got %d: %+v", len(got), got)
	}
	if got[0].Amount.Amount != 250000 {
		t.Errorf("buffer = %d, want 250000", got[0].Amount.Amount)
	}
}

func TestP4NoSpendNoSuggestion(t *testing.T) {
	if got := p4Affordability(baseInput()); len(got) != 0 {
		t.Errorf("no spend history — want 0, got %d", len(got))
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
