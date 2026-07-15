// SPDX-License-Identifier: MIT

package preflight

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestBuild(t *testing.T) {
	in := Input{
		Now:        d(2026, 7, 1),
		CycleStart: d(2026, 7, 1),
		NextPayday: d(2026, 7, 15),
		Bills: []BillItem{
			{ID: "b1", Name: "Rent", AmountMinor: 120000, Due: d(2026, 7, 3)},
			{ID: "b2", Name: "Netflix", AmountMinor: 1599, Due: d(2026, 7, 10), Autopay: true},
			{ID: "b3", Name: "NextCycle", AmountMinor: 5000, Due: d(2026, 7, 20)}, // after next payday, excluded
			{ID: "b4", Name: "Past", AmountMinor: 5000, Due: d(2026, 6, 25)},      // before now, excluded
		},
		ProjectedLowMinor: -5000,
		ProjectedLowDate:  d(2026, 7, 12),
		KeepFloorMinor:    10000,
		Accounts: []AccountBalance{
			{ID: "a1", Name: "Checking", BalanceMinor: 8000},  // below floor
			{ID: "a2", Name: "Savings", BalanceMinor: 500000}, // fine
		},
	}
	c := Build(in)

	if len(c.Bills) != 2 {
		t.Fatalf("want 2 bills this cycle, got %d", len(c.Bills))
	}
	if c.Bills[0].ID != "b1" || c.Bills[1].ID != "b2" {
		t.Fatalf("bills not sorted by due: %+v", c.Bills)
	}
	if !c.Bills[1].Autopay {
		t.Fatalf("Netflix should be marked autopay")
	}
	if c.TotalDueMinor != 121599 {
		t.Fatalf("total due = %d, want 121599", c.TotalDueMinor)
	}
	if !c.BelowFloor {
		t.Fatalf("projected low -5000 is below floor 10000; BelowFloor should be true")
	}
	if c.ShortfallMinor != 15000 {
		t.Fatalf("shortfall = %d, want 15000", c.ShortfallMinor)
	}
	if len(c.DippingAccounts) != 1 || c.DippingAccounts[0].ID != "a1" {
		t.Fatalf("want only Checking dipping, got %+v", c.DippingAccounts)
	}
	if c.DippingAccounts[0].ShortfallMinor != 2000 {
		t.Fatalf("account shortfall = %d, want 2000", c.DippingAccounts[0].ShortfallMinor)
	}
	if !c.HasItems() {
		t.Fatalf("HasItems should be true")
	}
}

func TestBuildSkipsPaidBills(t *testing.T) {
	in := Input{
		Now:        d(2026, 7, 1),
		CycleStart: d(2026, 7, 1),
		NextPayday: d(2026, 7, 15),
		Bills: []BillItem{
			{ID: "b1", Name: "Rent", AmountMinor: 120000, Due: d(2026, 7, 3)},
			{ID: "b2", Name: "Netflix", AmountMinor: 1599, Due: d(2026, 7, 10)},
		},
		Paid: map[string]bool{"b2": true}, // already settled by a matched txn (TX9)
	}
	c := Build(in)
	if len(c.Bills) != 1 || c.Bills[0].ID != "b1" {
		t.Fatalf("paid bill should be dropped, got %+v", c.Bills)
	}
	if c.TotalDueMinor != 120000 {
		t.Fatalf("total due = %d, want 120000 (paid bill excluded)", c.TotalDueMinor)
	}
}

func TestBuildEmpty(t *testing.T) {
	c := Build(Input{Now: d(2026, 7, 1), NextPayday: d(2026, 7, 15), KeepFloorMinor: 100, ProjectedLowMinor: 5000})
	if c.HasItems() {
		t.Fatalf("empty checklist should have no items")
	}
	if c.BelowFloor {
		t.Fatalf("low 5000 above floor 100 should not be below")
	}
}

func TestResolveForBill(t *testing.T) {
	row := BillRow{Name: "Rent", AmountMinor: 120000, Currency: "USD"}
	r := ResolveForBill(row)
	if r.MatchPayee != "Rent" {
		t.Fatalf("payee = %q", r.MatchPayee)
	}
	if r.MatchAmountMinor != 120000 {
		t.Fatalf("amount = %d", r.MatchAmountMinor)
	}
	if r.MatchToleranceMinor != 2400 { // 2% of 120000
		t.Fatalf("tolerance = %d, want 2400", r.MatchToleranceMinor)
	}
	if !r.HasMatcher() {
		t.Fatalf("should have matcher")
	}
}
