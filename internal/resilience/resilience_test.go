// SPDX-License-Identifier: MIT

package resilience

import "testing"

// A worked household: $3,000 buffer, $4,000 income, $2,600 living expenses,
// $400 debt minimums, $5,000 on cards at 24% APR. (Amounts in cents.)
func base() Input {
	return Input{
		LiquidCash:       300000,
		MonthlyIncome:    400000,
		MonthlySpend:     260000,
		MinDebtPayments:  40000,
		RevolvingBalance: 500000,
		AvgCardAPR:       24,
	}
}

func TestMonthlySurplusAndRunway(t *testing.T) {
	in := base()
	// Outflow = 2600 + 400 = 3000; surplus = 4000 - 3000 = 1000 → 100000 cents.
	if got := in.MonthlySurplus(); got != 100000 {
		t.Errorf("surplus = %d, want 100000", got)
	}
	// Runway = 3000 buffer / 3000 outflow = 1.0 month.
	if got := RunwayMonths(in); got != 1.0 {
		t.Errorf("runway = %v, want 1.0", got)
	}
}

func TestRunwayEdges(t *testing.T) {
	if got := RunwayMonths(Input{LiquidCash: 100000}); got != 0 {
		t.Errorf("no outflow → runway 0, got %v", got)
	}
	// Tiny outflow is capped at 120 months rather than reporting centuries.
	if got := RunwayMonths(Input{LiquidCash: 100000000, MonthlySpend: 1}); got != 120 {
		t.Errorf("huge buffer / tiny outflow → capped 120, got %v", got)
	}
}

func TestIncomeDrop(t *testing.T) {
	in := base()
	// A 20% cut: income 4000 → 3200; surplus 3200 - 3000 = 200 (still positive).
	o := IncomeDrop(in, 20)
	if o.NewMonthlyIncome != 320000 || o.NewSurplus != 20000 || o.GoesNegative {
		t.Errorf("20%% drop = %+v, want income 320000, surplus 20000, not negative", o)
	}
	// A 40% cut: income → 2400; surplus 2400 - 3000 = -600 → burns the buffer.
	// Buffer 3000 / 600 = 5 months to negative.
	o2 := IncomeDrop(in, 40)
	if !o2.GoesNegative || o2.NewSurplus != -60000 || o2.MonthsToNegative != 5 {
		t.Errorf("40%% drop = %+v, want negative, surplus -60000, 5 months", o2)
	}
	// Clamps out-of-range percents.
	if IncomeDrop(in, 250).DropPct != 100 || IncomeDrop(in, -5).DropPct != 0 {
		t.Error("income-drop percent not clamped to 0..100")
	}
}

func TestSurpriseExpense(t *testing.T) {
	in := base()
	// A $1,000 surprise: buffer 3000 → 2000, nothing pushed to debt.
	o := SurpriseExpense(in, 100000)
	if o.BufferAfter != 200000 || o.PushedToDebt != 0 || o.ExtraMonthlyInterest != 0 {
		t.Errorf("$1k surprise = %+v, want buffer 200000, no debt", o)
	}
	// A $5,000 surprise: buffer 3000 → -2000; $2,000 lands on cards.
	// Extra monthly interest = 2000 * 24% / 12 = $40 → 4000 cents.
	o2 := SurpriseExpense(in, 500000)
	if o2.BufferAfter != -200000 || o2.PushedToDebt != 200000 || o2.ExtraMonthlyInterest != 4000 {
		t.Errorf("$5k surprise = %+v, want buffer -200000, pushed 200000, interest 4000", o2)
	}
}

func TestRateHike(t *testing.T) {
	in := base()
	// +5 points on a $5,000 balance = 5000 * 5% / 12 = $20.83/mo → 2083 cents.
	o := RateHike(in, 5)
	if o.ExtraMonthlyInterest != 2083 || o.ExtraAnnualInterest != 2083*12 {
		t.Errorf("rate hike = %+v, want ~2083/mo", o)
	}
	// No revolving balance → no extra interest.
	noCards := base()
	noCards.RevolvingBalance = 0
	if RateHike(noCards, 5).ExtraMonthlyInterest != 0 {
		t.Error("no revolving balance → no rate-hike interest")
	}
}
