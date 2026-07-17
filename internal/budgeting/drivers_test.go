// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTopDrivers(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "b1", CategoryID: "groceries", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD"), Scope: domain.ScopeShared}
	covers := b.TracksCategory
	day := func(d int) time.Time { return time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC) }
	txns := []domain.Transaction{
		{ID: "big", Payee: "Costco", CategoryID: "groceries", Amount: money.New(-24000, "USD"), Date: day(3)},
		{ID: "mid", Payee: "Whole Foods", CategoryID: "groceries", Amount: money.New(-9000, "USD"), Date: day(10)},
		{ID: "small", Payee: "Corner Store", CategoryID: "groceries", Amount: money.New(-1500, "USD"), Date: day(12)},
		{ID: "other", Payee: "Shell", CategoryID: "gas", Amount: money.New(-6000, "USD"), Date: day(5)},        // wrong category
		{ID: "income", Payee: "Job", CategoryID: "groceries", Amount: money.New(30000, "USD"), Date: day(1)},   // income, excluded
		{ID: "old", Payee: "Costco", CategoryID: "groceries", Amount: money.New(-40000, "USD"), Date: time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)}, // before period
	}
	drivers, err := TopDrivers(b, txns, start, end, rates, covers, 2, nil)
	if err != nil {
		t.Fatalf("TopDrivers: %v", err)
	}
	if len(drivers) != 2 {
		t.Fatalf("got %d drivers, want top 2: %+v", len(drivers), drivers)
	}
	if drivers[0].TxnID != "big" || drivers[0].Amount != 24000 {
		t.Errorf("driver[0] = %+v, want Costco $240", drivers[0])
	}
	if drivers[1].TxnID != "mid" {
		t.Errorf("driver[1] = %+v, want Whole Foods", drivers[1])
	}
	if drivers[0].Label != "Costco" {
		t.Errorf("label = %q, want Costco", drivers[0].Label)
	}
}

func TestTopDriversTagAndSplit(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	// A tag budget: a charge carrying the tag counts once and whole.
	b := domain.Budget{ID: "b2", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD"), Scope: domain.ScopeShared, TrackedTags: []string{"vacation"}}
	txns := []domain.Transaction{
		{ID: "trip", Payee: "Airbnb", CategoryID: "travel", Amount: money.New(-28000, "USD"), Date: time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC), Tags: []string{"vacation"}},
		{ID: "notag", Payee: "Rent", CategoryID: "housing", Amount: money.New(-90000, "USD"), Date: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
	}
	drivers, err := TopDrivers(b, txns, start, end, rates, b.TracksCategory, 5, nil)
	if err != nil {
		t.Fatalf("TopDrivers: %v", err)
	}
	if len(drivers) != 1 || drivers[0].TxnID != "trip" {
		t.Fatalf("tag budget should surface only the tagged charge, got %+v", drivers)
	}
	if len(drivers[0].Tags) == 0 || drivers[0].Tags[0] != "vacation" {
		t.Errorf("driver should carry its tags for subscription/recurring detection: %+v", drivers[0])
	}
}

// TestTopDriversAggregatesByMerchant proves the panel groups a store's charges into
// one line (via the normalize resolver) so a merchant appearing under several raw
// descriptors, or through many small charges, surfaces as its true combined total —
// not lost below single larger one-off charges.
func TestTopDriversAggregatesByMerchant(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "b1", CategoryID: "groceries", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD"), Scope: domain.ScopeShared}
	day := func(d int) time.Time { return time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC) }
	// Six $60 coffee runs (two raw descriptors for the same shop) total $360 — more
	// than the single $200 one-off, but no single run would crack a top-N by charge.
	txns := []domain.Transaction{
		{ID: "oneoff", Payee: "Butcher", CategoryID: "groceries", Amount: money.New(-20000, "USD"), Date: day(2)},
		{ID: "c1", Payee: "Blue Bottle", CategoryID: "groceries", Amount: money.New(-6000, "USD"), Date: day(3)},
		{ID: "c2", Payee: "BLUE BOTTLE COFFEE #12", CategoryID: "groceries", Amount: money.New(-6000, "USD"), Date: day(5)},
		{ID: "c3", Payee: "Blue Bottle", CategoryID: "groceries", Amount: money.New(-6000, "USD"), Date: day(8)},
		{ID: "c4", Payee: "Blue Bottle", CategoryID: "groceries", Amount: money.New(-6000, "USD"), Date: day(11)},
		{ID: "c5", Payee: "BLUE BOTTLE COFFEE #12", CategoryID: "groceries", Amount: money.New(-6000, "USD"), Date: day(14)},
		{ID: "c6", Payee: "Blue Bottle", CategoryID: "groceries", Amount: money.New(-6000, "USD"), Date: day(17)},
	}
	// A resolver that maps both raw descriptors to one clean merchant name.
	normalize := func(raw string) string {
		if raw == "Blue Bottle" || raw == "BLUE BOTTLE COFFEE #12" {
			return "Blue Bottle Coffee"
		}
		return raw
	}
	drivers, err := TopDrivers(b, txns, start, end, rates, b.TracksCategory, 5, normalize)
	if err != nil {
		t.Fatalf("TopDrivers: %v", err)
	}
	if len(drivers) != 2 {
		t.Fatalf("want 2 merchants (coffee + butcher), got %d: %+v", len(drivers), drivers)
	}
	if drivers[0].Label != "Blue Bottle Coffee" || drivers[0].Amount != 36000 {
		t.Errorf("driver[0] = %+v, want Blue Bottle Coffee $360 aggregated", drivers[0])
	}
	if drivers[1].Label != "Butcher" || drivers[1].Amount != 20000 {
		t.Errorf("driver[1] = %+v, want Butcher $200", drivers[1])
	}
}
