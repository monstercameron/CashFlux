// SPDX-License-Identifier: MIT

package clearwatch

import "testing"

func rep(v, n int) []int {
	out := make([]int, 0, n)
	for range n {
		out = append(out, v)
	}
	return out
}

func TestItLearnsTheAccountsOwnWindow(t *testing.T) {
	w := Learn(rep(1, 10))
	if !w.Known || w.TypicalDays != 1 {
		t.Errorf("window = %+v, want a 1-day normal", w)
	}
	if w.Samples != 10 {
		t.Errorf("samples = %d, want the evidence carried", w.Samples)
	}
	slow := Learn(rep(4, 12))
	if !slow.Known || slow.TypicalDays != 4 {
		t.Errorf("window = %+v, want a 4-day normal", slow)
	}
	// A single rule would be wrong for both: 5 days is late on the first account
	// and perfectly ordinary on the second.
	if _, over := w.OverdueBy(5); !over {
		t.Error("5 days was not late for a 1-day account")
	}
	if _, over := slow.OverdueBy(5); over {
		t.Error("5 days was called late for a 4-day account")
	}
}

// The output is "this is unusual for you", which is a claim about a normal that
// has to exist first.
func TestTooLittleHistoryMeansNoVerdict(t *testing.T) {
	w := Learn(rep(2, MinSamples-1))
	if w.Known {
		t.Errorf("a window was learned from %d samples: %+v", MinSamples-1, w)
	}
	if _, over := w.OverdueBy(90); over {
		t.Error("an account with no normal reported something abnormal")
	}
	if Learn(nil).Known {
		t.Error("a window was learned from nothing")
	}
}

// One charge that took six weeks — a dispute, a hold, a correction — would drag
// a mean far enough that nothing ever looks late again.
func TestOneStuckChargeDoesNotMoveTheWindow(t *testing.T) {
	obs := append(rep(1, 11), 42)
	w := Learn(obs)
	if !w.Known || w.TypicalDays != 1 {
		t.Errorf("window = %+v, want the median to hold at 1", w)
	}
	if _, over := w.OverdueBy(10); !over {
		t.Error("a 10-day-old charge was not late for a 1-day account")
	}
}

// Half the reason a charge is late is a weekend, and flagging on the first day
// past the median would fire on most Fridays.
func TestTheGraceIsRealSlack(t *testing.T) {
	w := Learn(rep(1, 10))
	for _, age := range []int{1, 2, 3, 4} {
		if _, over := w.OverdueBy(age); over {
			t.Errorf("%d days was flagged against a 1-day normal with %d days grace", age, GraceDays)
		}
	}
	days, over := w.OverdueBy(6)
	if !over {
		t.Fatal("6 days was not flagged")
	}
	if days != 2 {
		t.Errorf("overdue by %d, want 2 past the 1+%d limit", days, GraceDays)
	}
}

// An account whose median is longer than a month is not telling us about
// clearing time — it is telling us its entries are entered late.
func TestAnAbsurdWindowIsNoWindow(t *testing.T) {
	if w := Learn(rep(MaxWindowDays+1, 20)); w.Known {
		t.Errorf("a %d-day median was accepted as a clearing window: %+v", MaxWindowDays+1, w)
	}
	if w := Learn(rep(MaxWindowDays, 20)); !w.Known {
		t.Errorf("a %d-day median was rejected at the boundary", MaxWindowDays)
	}
}

func TestNegativeObservationsAreDropped(t *testing.T) {
	// A row that cleared before it happened is data disagreeing with itself; it
	// must not pull the median down.
	obs := append(rep(3, 10), -5, -9)
	w := Learn(obs)
	if !w.Known || w.TypicalDays != 3 {
		t.Errorf("window = %+v, want 3", w)
	}
	if w.Samples != 10 {
		t.Errorf("samples = %d, want the impossible ones excluded", w.Samples)
	}
}

func TestAnEvenSampleTakesTheMidpoint(t *testing.T) {
	w := Learn([]int{1, 1, 1, 1, 3, 3, 3, 3})
	if !w.Known || w.TypicalDays != 2 {
		t.Errorf("window = %+v, want the midpoint of 1 and 3", w)
	}
}

func TestAFutureDatedChargeIsNotOverdue(t *testing.T) {
	w := Learn(rep(1, 10))
	if _, over := w.OverdueBy(-2); over {
		t.Error("a charge dated in the future was reported overdue")
	}
}

func TestLearnIsStable(t *testing.T) {
	obs := []int{5, 1, 3, 2, 9, 1, 2, 3, 4, 1}
	first := Learn(obs)
	for i := range 5 {
		if got := Learn(obs); got != first {
			t.Fatalf("run %d differed: %+v vs %+v", i, got, first)
		}
	}
}
