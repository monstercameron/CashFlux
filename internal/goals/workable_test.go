// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func aug2026() time.Time { return time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC) }

// ─── C401: the date a goal could actually be met by ──────────────────────────

func TestWorkableDateRoundsUpToACompleteMonth(t *testing.T) {
	from := aug2026()
	// $1,000 still needed at $300/mo is four months, not three-and-a-third: the
	// household cannot save a third of a month.
	got, ok := WorkableDate(100_000, 30_000, from)
	if !ok {
		t.Fatal("no date for a fundable goal")
	}
	if want := time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("date = %s, want %s", got.Format(time.DateOnly), want.Format(time.DateOnly))
	}
	// Exactly divisible: no spurious extra month.
	got, _ = WorkableDate(90_000, 30_000, from)
	if want := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("date = %s, want %s", got.Format(time.DateOnly), want.Format(time.DateOnly))
	}
}

// Saving nothing reaches no date. Returning a far-future one would dress that
// up as a plan, which is the failure this whole feature exists to avoid.
func TestWorkableDateHasNoAnswerWithoutContribution(t *testing.T) {
	for _, monthly := range []int64{0, -100} {
		if _, ok := WorkableDate(100_000, monthly, aug2026()); ok {
			t.Errorf("offered a date at %d/mo", monthly)
		}
	}
}

// Beyond the cap the answer is not a date. A suggestion of 2061 looks like
// advice and is not.
func TestWorkableDateRefusesAbsurdHorizons(t *testing.T) {
	if _, ok := WorkableDate(100_000_00, 1_00, aug2026()); ok {
		t.Error("offered a date 100 years out — that is not a plan")
	}
}

func TestWorkableDateOnAMetGoalIsNow(t *testing.T) {
	got, ok := WorkableDate(0, 0, aug2026())
	if !ok {
		t.Fatal("a goal already met has no date")
	}
	if !got.Equal(time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date = %s, want this month", got.Format(time.DateOnly))
	}
}

// The date uses the goal's FAIR SHARE, not the whole surplus: solving one goal
// by assuming every other goal gets nothing replaces one infeasible plan with
// several.
func TestWorkableTargetDateUsesTheFairShare(t *testing.T) {
	g := domain.Goal{
		ID: "g1", TargetAmount: money.New(120_000, "USD"), CurrentAmount: money.New(0, "USD"),
	}
	// $600/mo of slack across 3 dated goals = $200 fair share → 6 months.
	got, ok := WorkableTargetDate(g, 60_000, 3, aug2026())
	if !ok {
		t.Fatal("no date")
	}
	if want := time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("date = %s, want %s — using the whole surplus would have said Oct 2026",
			got.Format(time.DateOnly), want.Format(time.DateOnly))
	}
	// A zero goal count must not divide by zero; it means "this one".
	if _, ok := WorkableTargetDate(g, 60_000, 0, aug2026()); !ok {
		t.Error("no date when the goal count is zero")
	}
}
