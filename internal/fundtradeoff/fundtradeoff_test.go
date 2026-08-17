// SPDX-License-Identifier: MIT

package fundtradeoff

import "testing"

func g(id string, remaining, monthly int64) Goal {
	return Goal{ID: id, Name: id, RemainingMinor: remaining, MonthlyMinor: monthly}
}

// If the pot covers everything, funding one goal delays nothing, and saying it
// does is scaremongering that teaches people to ignore the warning.
func TestNoTradeOffWhenTheMoneyIsNotContested(t *testing.T) {
	costs := Assess(50_000, 50_000, []Goal{g("vacation", 300_000, 25_000)})
	if Any(costs) {
		t.Errorf("spending slack was reported as a trade-off: %+v", costs)
	}
	// Partial slack: only the uncovered part comes out of anything.
	costs = Assess(50_000, 40_000, []Goal{g("vacation", 300_000, 25_000)})
	if len(costs) != 1 || costs[0].LostMonthlyMinor != 10_000 {
		t.Errorf("costs = %+v, want just the $100 that was not covered", costs)
	}
}

// "This will slow your other goals" is unfalsifiable and unactionable.
func TestItNamesTheGoalThatPaysAndByHowLong(t *testing.T) {
	// Vacation needs $3,000 more at $250/mo = 12 months. Take $100/mo and it
	// takes 20 months instead.
	costs := Assess(10_000, 0, []Goal{g("vacation", 300_000, 25_000)})
	if len(costs) != 1 {
		t.Fatalf("costs = %+v, want one", costs)
	}
	if costs[0].GoalName != "vacation" {
		t.Errorf("named %q", costs[0].GoalName)
	}
	if costs[0].MonthsDelayed != 8 {
		t.Errorf("delay = %d months, want 8", costs[0].MonthsDelayed)
	}
	// The lost amount is published so the reader can check the arithmetic rather
	// than trust it.
	if costs[0].LostMonthlyMinor != 10_000 {
		t.Errorf("lost = %d, want 10000", costs[0].LostMonthlyMinor)
	}
}

// "Delayed by 9,999 months" is not information; "this stops it entirely" is.
func TestAGoalThatLosesEverythingStallsRatherThanReportingAHugeNumber(t *testing.T) {
	costs := Assess(25_000, 0, []Goal{g("vacation", 300_000, 25_000)})
	if len(costs) != 1 || !costs[0].Stalls {
		t.Fatalf("costs = %+v, want a stall", costs)
	}
	if costs[0].MonthsDelayed != 0 {
		t.Errorf("a stalled goal reported %d months", costs[0].MonthsDelayed)
	}
}

// A goal nobody is contributing to cannot be made later.
func TestUnfundedGoalsPayNothing(t *testing.T) {
	costs := Assess(10_000, 0, []Goal{g("someday", 500_000, 0), g("vacation", 300_000, 25_000)})
	if len(costs) != 1 || costs[0].GoalID != "vacation" {
		t.Errorf("costs = %+v, want only the funded goal to pay", costs)
	}
}

func TestTheCostCascadesDownThePriorityOrder(t *testing.T) {
	// Taking $400/mo when the next goal only contributes $250: it stalls, and the
	// remaining $150 comes out of the one after.
	costs := Assess(40_000, 0, []Goal{g("vacation", 300_000, 25_000), g("car", 600_000, 30_000)})
	if len(costs) != 2 {
		t.Fatalf("costs = %+v, want both to pay", costs)
	}
	var car Cost
	for _, c := range costs {
		if c.GoalID == "car" {
			car = c
		}
	}
	if car.LostMonthlyMinor != 15_000 {
		t.Errorf("car lost %d, want the remaining 15000", car.LostMonthlyMinor)
	}
}

// The reader can stop as soon as they have seen the thing that would put them
// off — the same ordering rule the action preview uses.
func TestWorstComesFirst(t *testing.T) {
	costs := Assess(35_000, 0, []Goal{
		g("small", 100_000, 10_000), // loses everything → stalls
		g("big", 900_000, 30_000),   // loses the remainder
	})
	if len(costs) < 2 {
		t.Fatalf("costs = %+v", costs)
	}
	if !costs[0].Stalls {
		t.Errorf("first = %+v, want the stalled goal", costs[0])
	}
}

func TestNothingTakenIsNoTradeOff(t *testing.T) {
	if Any(Assess(0, 0, []Goal{g("vacation", 300_000, 25_000)})) {
		t.Error("taking nothing produced a cost")
	}
	if Any(Assess(10_000, 0, nil)) {
		t.Error("a cost was reported with nothing to take it from")
	}
}

// A goal that loses money but still lands in the same month is not worth a
// sentence.
func TestATrivialLossIsNotReported(t *testing.T) {
	// $1,000 remaining at $500/mo is 2 months; at $499/mo it is still 3... so use
	// a case that genuinely rounds to the same month.
	costs := Assess(1, 0, []Goal{g("vacation", 100_000, 50_000)})
	for _, c := range costs {
		if c.MonthsDelayed == 0 && !c.Stalls {
			t.Errorf("a zero-month delay was reported: %+v", c)
		}
	}
}
