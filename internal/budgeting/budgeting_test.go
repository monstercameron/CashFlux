package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func mustDate(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

var june = func() (start, end time.Time) { return dateutil.MonthRange(mustDate("2026-06-15")) }

func TestPeriodRange(t *testing.T) {
	ref := mustDate("2026-06-15") // a Monday

	// Monthly: the whole of June.
	s, e := PeriodRange(domain.PeriodMonthly, ref, time.Sunday)
	if s != mustDate("2026-06-01") || e != mustDate("2026-07-01") {
		t.Errorf("monthly = %v..%v", s.Format("2006-01-02"), e.Format("2006-01-02"))
	}

	// Weekly (Sunday start): 2026-06-14 .. 2026-06-21.
	s, e = PeriodRange(domain.PeriodWeekly, ref, time.Sunday)
	if s != mustDate("2026-06-14") || e != mustDate("2026-06-21") {
		t.Errorf("weekly(Sun) = %v..%v", s.Format("2006-01-02"), e.Format("2006-01-02"))
	}
	// Weekly (Monday start): 2026-06-15 .. 2026-06-22.
	s, e = PeriodRange(domain.PeriodWeekly, ref, time.Monday)
	if s != mustDate("2026-06-15") || e != mustDate("2026-06-22") {
		t.Errorf("weekly(Mon) = %v..%v", s.Format("2006-01-02"), e.Format("2006-01-02"))
	}

	// Quarterly: Q2 is Apr 1 .. Jul 1.
	s, e = PeriodRange(domain.PeriodQuarterly, ref, time.Sunday)
	if s != mustDate("2026-04-01") || e != mustDate("2026-07-01") {
		t.Errorf("quarterly = %v..%v", s.Format("2006-01-02"), e.Format("2006-01-02"))
	}
}

func expense(amount int64, cur, cat, member, day string) domain.Transaction {
	return domain.Transaction{
		Amount:     money.New(-amount, cur),
		CategoryID: cat,
		MemberID:   member,
		Date:       mustDate(day),
	}
}

func TestSpentIndividualScope(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "m1", Limit: usd(50000)}
	all := []domain.Transaction{
		expense(10000, "USD", "food", "m1", "2026-06-03"),                                     // counts
		expense(5000, "USD", "food", "m2", "2026-06-04"),                                      // other member, excluded
		expense(3000, "USD", "rent", "m1", "2026-06-05"),                                      // other category, excluded
		expense(2000, "USD", "food", "m1", "2026-07-02"),                                      // out of period, excluded
		{Amount: usd(9999), CategoryID: "food", MemberID: "m1", Date: mustDate("2026-06-06")}, // income, excluded
	}
	spent, err := Spent(budget, all, start, end, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !spent.Equal(usd(10000)) {
		t.Errorf("spent = %v, want 10000 USD", spent)
	}
}

func TestSpentSharedScope(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}
	all := []domain.Transaction{
		expense(10000, "USD", "food", "m1", "2026-06-03"),
		expense(5000, "USD", "food", "m2", "2026-06-04"),
	}
	spent, err := Spent(budget, all, start, end, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !spent.Equal(usd(15000)) {
		t.Errorf("spent = %v, want 15000 USD (all members)", spent)
	}
}

func TestSpentIgnoresTransfers(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}
	all := []domain.Transaction{
		expense(2000, "USD", "food", "", "2026-06-03"),
		{
			AccountID: "checking", TransferAccountID: "savings", CategoryID: "food",
			Amount: usd(-9000), Date: mustDate("2026-06-04"),
		},
	}

	spent, err := Spent(budget, all, start, end, rates)
	if err != nil {
		t.Fatalf("Spent: %v", err)
	}
	if !spent.Equal(usd(2000)) {
		t.Errorf("spent with transfer = %v, want 2000 USD", spent)
	}
}

func TestSpentScopeAggregationMixedMembers(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		expense(10000, "USD", "food", "m1", "2026-06-03"),
		expense(5000, "USD", "food", "m2", "2026-06-04"),
		expense(3000, "USD", "rent", "m1", "2026-06-05"),
	}
	individual := domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "m1", Limit: usd(50000)}
	group := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}

	indivSpent, err := Spent(individual, txns, start, end, rates)
	if err != nil {
		t.Fatalf("individual Spent error: %v", err)
	}
	if !indivSpent.Equal(usd(10000)) {
		t.Errorf("individual spent = %v, want 10000 USD", indivSpent)
	}

	groupSpent, err := Spent(group, txns, start, end, rates)
	if err != nil {
		t.Fatalf("group Spent error: %v", err)
	}
	if !groupSpent.Equal(usd(15000)) {
		t.Errorf("group spent = %v, want 15000 USD", groupSpent)
	}
}

func TestSpentMultiCurrency(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(50000)}
	all := []domain.Transaction{
		expense(10000, "USD", "food", "", "2026-06-03"), // 100 USD
		expense(10000, "EUR", "food", "", "2026-06-04"), // 100 EUR -> 110 USD
	}
	spent, err := Spent(budget, all, start, end, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !spent.Equal(usd(21000)) { // 210.00
		t.Errorf("spent = %v, want 21000 USD", spent)
	}
}

func TestEvaluateStates(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	mk := func(spentMinor int64) Status {
		budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(10000)}
		all := []domain.Transaction{expense(spentMinor, "USD", "food", "", "2026-06-03")}
		s, err := Evaluate(budget, all, start, end, rates, DefaultNearThreshold)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		return s
	}

	ok := mk(5000) // 50% of 100.00
	if ok.State != StateOK || ok.Percent != 50 || !ok.Remaining.Equal(usd(5000)) {
		t.Errorf("ok: state=%s pct=%d rem=%v", ok.State, ok.Percent, ok.Remaining)
	}
	near := mk(9000) // 90%
	if near.State != StateNear || near.Percent != 90 {
		t.Errorf("near: state=%s pct=%d", near.State, near.Percent)
	}
	over := mk(12000) // 120%
	if over.State != StateOver || over.Percent != 120 || !over.Remaining.Equal(usd(-2000)) {
		t.Errorf("over: state=%s pct=%d rem=%v", over.State, over.Percent, over.Remaining)
	}
}

func TestEvaluateZeroLimit(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(0)}
	all := []domain.Transaction{expense(1000, "USD", "food", "", "2026-06-03")}
	s, err := Evaluate(budget, all, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if s.State != StateOver || s.Percent != 100 {
		t.Errorf("zero limit with spend: state=%s pct=%d, want over/100", s.State, s.Percent)
	}
}
