// SPDX-License-Identifier: MIT

package goalcompare

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func fin(id string, target int64) domain.Goal {
	return domain.Goal{ID: id, Name: id, Kind: domain.GoalKindFinancial, TargetAmount: money.New(target, "USD")}
}

func TestEligibilityNamesTheMostActionableReason(t *testing.T) {
	cases := []struct {
		name string
		goal domain.Goal
		want Reason
	}{
		{"ready", fin("a", 100000), ReasonNone},
		{"no target", fin("b", 0), ReasonNoTarget},
		{"archived", func() domain.Goal { g := fin("c", 100000); g.Archived = true; return g }(), ReasonArchived},
		// An archived habit goal fails two rules. Telling the reader to un-archive
		// it would be useless advice — it still would not be comparable.
		{"archived habit", domain.Goal{ID: "d", Kind: domain.GoalKindHabit, Archived: true}, ReasonNotFinancial},
	}
	for _, c := range cases {
		if got := Eligibility(c.goal); got != c.want {
			t.Errorf("%s: Eligibility = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPartitionKeepsOrderAndExplainsEveryExclusion(t *testing.T) {
	archived := fin("arch", 5000)
	archived.Archived = true
	goals := []domain.Goal{fin("one", 1000), archived, fin("two", 2000), fin("empty", 0)}

	ok, out := Partition(goals)
	if len(ok) != 2 || ok[0].ID != "one" || ok[1].ID != "two" {
		t.Fatalf("eligible = %v", ids(ok))
	}
	if len(out) != 2 {
		t.Fatalf("excluded %d, want 2", len(out))
	}
	if out[0].Reason != ReasonArchived || out[1].Reason != ReasonNoTarget {
		t.Errorf("reasons = %q,%q", out[0].Reason, out[1].Reason)
	}
	for _, e := range out {
		if e.Reason == ReasonNone {
			t.Errorf("%s was excluded with no reason", e.Goal.ID)
		}
	}
}

// The plain case: one surplus, two uncapped goals. First finishes, then the
// whole surplus moves to the second.
func TestRaceHandsOverWhenTheFirstGoalFinishes(t *testing.T) {
	got := Race(10000, Fund{RemainingMinor: 30000}, Fund{RemainingMinor: 20000})
	if !got.First.Reached || got.First.Months != 3 {
		t.Errorf("first = %+v, want 3 months", got.First)
	}
	if !got.Second.Reached || got.Second.Months != 5 {
		t.Errorf("second = %+v, want 5 months", got.Second)
	}
}

// A capped first goal cannot absorb the whole surplus, so the remainder must
// spill the SAME month. Dropping the spill would make a capped goal look free.
func TestRaceSpillsTheUnabsorbedRemainderImmediately(t *testing.T) {
	got := Race(10000, Fund{RemainingMinor: 30000, MonthlyCapMinor: 6000}, Fund{RemainingMinor: 20000})
	// A takes 6000/month → 5 months. B takes the leftover 4000/month → 5 months.
	if got.First.Months != 5 || !got.First.Reached {
		t.Errorf("first = %+v, want 5", got.First)
	}
	if got.Second.Months != 5 || !got.Second.Reached {
		t.Errorf("second = %+v, want 5 — the spill must not be discarded", got.Second)
	}
}

// A goal never takes more than it is short: the overshoot has to spill too.
func TestRaceDoesNotOverfundTheFirstGoal(t *testing.T) {
	got := Race(10000, Fund{RemainingMinor: 2500}, Fund{RemainingMinor: 7500})
	if got.First.Months != 1 {
		t.Errorf("first = %+v, want 1", got.First)
	}
	// 10000 - 2500 = 7500 spills, which finishes B in the same month.
	if got.Second.Months != 1 || !got.Second.Reached {
		t.Errorf("second = %+v, want 1 — the overshoot must spill", got.Second)
	}
}

func TestRaceWithAnAlreadyFundedGoal(t *testing.T) {
	got := Race(10000, Fund{RemainingMinor: 0}, Fund{RemainingMinor: 20000})
	if !got.First.Reached || got.First.Months != 0 {
		t.Errorf("first = %+v, want reached at month 0", got.First)
	}
	if got.Second.Months != 2 {
		t.Errorf("second = %+v, want 2", got.Second)
	}
}

// No surplus means no order to choose. Reporting a landing would invent one.
func TestRaceWithNoSurplusReachesNothing(t *testing.T) {
	got := Race(0, Fund{RemainingMinor: 100}, Fund{RemainingMinor: 100})
	if got.First.Reached || got.Second.Reached {
		t.Errorf("got %+v, want nothing reached", got)
	}
}

// A capped first goal that absorbs the entire surplus starves the second one.
// It must report unreached rather than a fabricated 600-month date.
func TestRaceReportsAStarvedGoalAsUnreached(t *testing.T) {
	got := Race(10000, Fund{RemainingMinor: 1 << 40, MonthlyCapMinor: 10000}, Fund{RemainingMinor: 5000})
	if got.Second.Reached {
		t.Errorf("second = %+v, want unreached", got.Second)
	}
	if got.First.Reached {
		t.Errorf("first = %+v, want unreached inside the cap", got.First)
	}
}

func TestCompareShowsTheTrade(t *testing.T) {
	a := Fund{RemainingMinor: 30000}
	b := Fund{RemainingMinor: 20000}
	s := Compare(10000, a, b)

	// A first: A lands at 3, B at 5. B first: B lands at 2, A at 5.
	if s.AFirst.First.Months != 3 || s.AFirst.Second.Months != 5 {
		t.Errorf("AFirst = %+v", s.AFirst)
	}
	if s.BFirst.First.Months != 2 || s.BFirst.Second.Months != 5 {
		t.Errorf("BFirst = %+v", s.BFirst)
	}
	if !s.Matters() {
		t.Error("the order changes both dates but Matters said no")
	}
}

// When the surplus covers both plans in full, order changes nothing — and
// presenting a trade-off would manufacture a decision the user does not have.
func TestMattersIsFalseWhenOrderChangesNothing(t *testing.T) {
	// Both capped low enough that the surplus funds both every month.
	a := Fund{RemainingMinor: 20000, MonthlyCapMinor: 5000}
	b := Fund{RemainingMinor: 20000, MonthlyCapMinor: 5000}
	if s := Compare(10000, a, b); s.Matters() {
		t.Errorf("Matters said yes for %+v", s)
	}
	// And with no money at all there is no order to choose.
	if s := Compare(0, Fund{RemainingMinor: 100}, Fund{RemainingMinor: 100}); s.Matters() {
		t.Error("Matters said yes with no surplus")
	}
}

func ids(gs []domain.Goal) []string {
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		out = append(out, g.ID)
	}
	return out
}
