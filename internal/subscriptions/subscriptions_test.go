// SPDX-License-Identifier: MIT

package subscriptions

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// charge is a non-transfer expense (negative amount) in USD.
func charge(desc string, minor int64, on time.Time) domain.Transaction {
	return domain.Transaction{Desc: desc, Amount: money.New(-minor, "USD"), Date: on}
}

func usd() currency.Rates { return currency.Rates{Base: "USD"} }

func TestDetectMonthly(t *testing.T) {
	txns := []domain.Transaction{
		charge("Netflix", 1599, d(2026, time.April, 1)),
		charge("Netflix", 1599, d(2026, time.May, 1)),
		charge("Netflix", 1599, d(2026, time.June, 1)),
	}
	subs, err := Detect(txns, usd(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("got %d subs, want 1: %+v", len(subs), subs)
	}
	s := subs[0]
	if s.Name != "Netflix" || s.Cadence != CadenceMonthly || s.Amount != 1599 || s.Count != 3 {
		t.Errorf("sub = %+v, want Netflix monthly 1599 x3", s)
	}
	if s.MonthlyAmount() != 1599 || s.AnnualAmount() != 1599*12 {
		t.Errorf("monthly/annual = %d/%d, want 1599/%d", s.MonthlyAmount(), s.AnnualAmount(), 1599*12)
	}
	if !s.NextRenewal.Equal(d(2026, time.July, 1)) {
		t.Errorf("next renewal = %s, want 2026-07-01", s.NextRenewal.Format("2006-01-02"))
	}
}

func TestDetectYearlyAndWeekly(t *testing.T) {
	txns := []domain.Transaction{
		charge("Domain", 1200, d(2024, time.June, 10)),
		charge("Domain", 1200, d(2025, time.June, 11)),
		charge("Gym", 2500, d(2026, time.June, 1)),
		charge("Gym", 2500, d(2026, time.June, 8)),
		charge("Gym", 2500, d(2026, time.June, 15)),
	}
	subs, err := Detect(txns, usd(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	by := map[string]Subscription{}
	for _, s := range subs {
		by[s.Name] = s
	}
	if dn := by["Domain"]; dn.Cadence != CadenceYearly || dn.MonthlyAmount() != 1200/12 || dn.AnnualAmount() != 1200 {
		t.Errorf("Domain = %+v, want yearly", dn)
	}
	if g := by["Gym"]; g.Cadence != CadenceWeekly || g.AnnualAmount() != 2500*52 {
		t.Errorf("Gym = %+v, want weekly", g)
	}
}

func TestDetectIgnoresIrregularAndSparse(t *testing.T) {
	txns := []domain.Transaction{
		// Irregular spacing — not a subscription.
		charge("Random", 500, d(2026, time.January, 3)),
		charge("Random", 500, d(2026, time.February, 19)),
		charge("Random", 500, d(2026, time.June, 2)),
		// Only one occurrence — can't infer a cadence.
		charge("Once", 999, d(2026, time.May, 5)),
		// Income with a recurring-looking description — excluded.
		{Desc: "Salary", Amount: money.New(500000, "USD"), Date: d(2026, time.May, 1)},
		{Desc: "Salary", Amount: money.New(500000, "USD"), Date: d(2026, time.June, 1)},
	}
	subs, err := Detect(txns, usd(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("got %d subs, want 0: %+v", len(subs), subs)
	}
}

func TestMonthlyTotalAndOrder(t *testing.T) {
	txns := []domain.Transaction{
		charge("Cheap", 500, d(2026, time.April, 1)),
		charge("Cheap", 500, d(2026, time.May, 1)),
		charge("Pricey", 5000, d(2026, time.April, 2)),
		charge("Pricey", 5000, d(2026, time.May, 2)),
	}
	subs, err := Detect(txns, usd(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 || subs[0].Name != "Pricey" {
		t.Fatalf("expected Pricey first (biggest monthly): %+v", subs)
	}
	if MonthlyTotal(subs) != 5500 {
		t.Errorf("MonthlyTotal = %d, want 5500", MonthlyTotal(subs))
	}
}
