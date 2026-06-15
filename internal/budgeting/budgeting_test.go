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
		expense(10000, "USD", "food", "m1", "2026-06-03"),       // counts
		expense(5000, "USD", "food", "m2", "2026-06-04"),        // other member, excluded
		expense(3000, "USD", "rent", "m1", "2026-06-05"),        // other category, excluded
		expense(2000, "USD", "food", "m1", "2026-07-02"),        // out of period, excluded
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
