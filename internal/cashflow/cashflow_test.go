// SPDX-License-Identifier: MIT

package cashflow

import "testing"

func TestDailyBalancesOverdraftBeforePayday(t *testing.T) {
	// $500 now; rent $1,800 hits day 1; payday $2,000 lands day 5.
	p := DailyBalances(50000, []Event{
		{Day: 1, Amount: -180000, Label: "Rent"},
		{Day: 5, Amount: 200000, Label: "Payday"},
	}, 10, 0)

	if !p.WillBreach() || p.BreachDay != 1 {
		t.Fatalf("expected overdraft on day 1, got BreachDay=%d (WillBreach=%v)", p.BreachDay, p.WillBreach())
	}
	if p.BreachShortfall != 130000 { // 0 - (50000-180000)
		t.Errorf("shortfall = %d, want 130000", p.BreachShortfall)
	}
	if p.MinBalance != -130000 || p.MinDay != 1 {
		t.Errorf("min = %d on day %d, want -130000 on day 1", p.MinBalance, p.MinDay)
	}
	// Recovers after payday.
	if p.Daily[5].Balance != 70000 {
		t.Errorf("day 5 balance = %d, want 70000 (recovered after payday)", p.Daily[5].Balance)
	}
	if len(p.Daily) != 10 {
		t.Errorf("daily series length = %d, want 10", len(p.Daily))
	}
}

func TestDailyBalancesBufferThreshold(t *testing.T) {
	// A $600 bill on day 2 drops the $1,000 balance below a $500 buffer.
	p := DailyBalances(100000, []Event{{Day: 2, Amount: -60000, Label: "Card"}}, 7, 50000)
	if !p.WillBreach() || p.BreachDay != 2 {
		t.Fatalf("expected a buffer breach on day 2, got %d", p.BreachDay)
	}
	if p.BreachShortfall != 10000 { // 50000 - 40000
		t.Errorf("shortfall = %d, want 10000", p.BreachShortfall)
	}
}

func TestDailyBalancesNoBreach(t *testing.T) {
	p := DailyBalances(500000, []Event{{Day: 3, Amount: -50000, Label: "Bill"}}, 14, 0)
	if p.WillBreach() {
		t.Errorf("a comfortable balance should not breach: BreachDay=%d", p.BreachDay)
	}
	if p.MinBalance != 450000 {
		t.Errorf("min = %d, want 450000", p.MinBalance)
	}
}

func TestDailyBalancesEdges(t *testing.T) {
	// Multiple events on the same day net together.
	p := DailyBalances(0, []Event{{Day: 0, Amount: 100000}, {Day: 0, Amount: -30000}}, 1, 0)
	if p.Daily[0].Balance != 70000 {
		t.Errorf("same-day events should net: got %d, want 70000", p.Daily[0].Balance)
	}
	// Events beyond the horizon are ignored.
	p2 := DailyBalances(100000, []Event{{Day: 99, Amount: -100000}}, 5, 0)
	if p2.WillBreach() || p2.MinBalance != 100000 {
		t.Errorf("out-of-horizon event should be ignored: %+v", p2)
	}
	// Non-positive horizon yields an empty projection.
	p3 := DailyBalances(12345, nil, 0, 0)
	if len(p3.Daily) != 0 || p3.WillBreach() || p3.MinBalance != 12345 {
		t.Errorf("empty horizon: %+v", p3)
	}
	// Starting already below the buffer breaches on day 0.
	p4 := DailyBalances(-500, nil, 3, 0)
	if p4.BreachDay != 0 || p4.BreachShortfall != 500 {
		t.Errorf("starting overdrawn should breach day 0: %+v", p4)
	}
}
