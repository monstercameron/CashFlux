// SPDX-License-Identifier: MIT

package goalcoach

import (
	"testing"
	"time"
)

var now = time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)

func on(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// The silences are the feature. Each of these is a case where a tracker that
// commented would become one you stop opening.

func TestAPausedGoalIsNeverNagged(t *testing.T) {
	g := Goal{ID: "g", Name: "Holiday", TargetMinor: 100000, MonthlyMinor: 1000,
		TargetDate: on(2026, time.October, 1), PausedUntil: on(2026, time.December, 1)}
	if got := Coach(g, now); !got.Silent() {
		t.Fatalf("a paused goal produced %v — pausing was the household's decision", got.Kind)
	}
}

func TestAGoalWithNoDeadlineCannotBeBehind(t *testing.T) {
	// Saying it is behind would invent a commitment nobody made.
	g := Goal{ID: "g", Name: "Rainy day", TargetMinor: 500000, SavedMinor: 1000, MonthlyMinor: 100}
	if got := Coach(g, now); !got.Silent() {
		t.Fatalf("a goal with no deadline produced %v", got.Kind)
	}
}

func TestAGoalSeenRecentlyIsLeftAlone(t *testing.T) {
	g := Goal{ID: "g", Name: "Car", TargetMinor: 100000, MonthlyMinor: 100,
		TargetDate: on(2026, time.December, 1), LastCheckedAt: now.Add(-48 * time.Hour)}
	if got := Coach(g, now); !got.Silent() {
		t.Fatalf("a goal coached two days ago produced %v again", got.Kind)
	}
	// Past the quiet period it speaks again.
	g.LastCheckedAt = now.Add(-30 * 24 * time.Hour)
	if got := Coach(g, now); got.Silent() {
		t.Fatal("a goal not seen for a month stayed silent")
	}
}

func TestAnArchivedGoalSaysNothing(t *testing.T) {
	g := Goal{ID: "g", Name: "Old", TargetMinor: 100, Archived: true, TargetDate: on(2026, time.December, 1)}
	if got := Coach(g, now); !got.Silent() {
		t.Fatalf("an archived goal produced %v", got.Kind)
	}
}

func TestAPassedDeadlineProposesNothing(t *testing.T) {
	// Proposing a monthly increase for a goal already due is arithmetic pretending
	// to be advice — the household can see the date has gone.
	g := Goal{ID: "g", Name: "Late", TargetMinor: 100000, SavedMinor: 1000,
		MonthlyMinor: 500, TargetDate: on(2026, time.July, 1)}
	if got := Coach(g, now); !got.Silent() {
		t.Fatalf("a passed deadline produced %v", got.Kind)
	}
}

func TestABehindGoalProposesTheExtraThatWouldFixIt(t *testing.T) {
	// $1,000 needed over 4 months on $100/month: short by $600, needs $250/month,
	// which is $150 more than the current plan.
	g := Goal{ID: "g", Name: "Car", TargetMinor: 100000, SavedMinor: 0,
		MonthlyMinor: 10000, TargetDate: on(2026, time.December, 16)}
	got := Coach(g, now)
	if got.Kind != KindBehind {
		t.Fatalf("kind = %v, want behind", got.Kind)
	}
	if got.MonthsRemaining != 4 {
		t.Fatalf("months = %d, want 4", got.MonthsRemaining)
	}
	if got.ShortfallMinor != 60000 {
		t.Fatalf("shortfall = %d, want 60000", got.ShortfallMinor)
	}
	if got.SuggestedMonthlyMinor != 25000 || got.ExtraMonthlyMinor != 15000 {
		t.Fatalf("proposal = %d/mo (%d more), want 25000/15000", got.SuggestedMonthlyMinor, got.ExtraMonthlyMinor)
	}
}

func TestTheProposalNeverLandsAPennyShort(t *testing.T) {
	// $1,000 over 3 months is $333.34, not $333.33 — rounding down would miss the
	// deadline by a cent, which is the one outcome the proposal exists to prevent.
	g := Goal{ID: "g", Name: "Odd", TargetMinor: 100000, MonthlyMinor: 1,
		TargetDate: on(2026, time.November, 16)}
	got := Coach(g, now)
	if got.SuggestedMonthlyMinor*int64(got.MonthsRemaining) < 100000-1 {
		t.Fatalf("%d/mo × %d months does not cover 100000", got.SuggestedMonthlyMinor, got.MonthsRemaining)
	}
}

func TestAnUnfundedGoalWithADeadlineIsTheOneCaseWorthRaising(t *testing.T) {
	g := Goal{ID: "g", Name: "Wedding", TargetMinor: 120000, TargetDate: on(2026, time.December, 16)}
	got := Coach(g, now)
	if got.Kind != KindUnfunded {
		t.Fatalf("kind = %v, want unfunded", got.Kind)
	}
	if got.SuggestedMonthlyMinor != 30000 {
		t.Fatalf("suggested = %d, want 30000 over 4 months", got.SuggestedMonthlyMinor)
	}
}

func TestAnOnTrackGoalIsAcknowledgedNotAdvised(t *testing.T) {
	g := Goal{ID: "g", Name: "Laptop", TargetMinor: 100000, MonthlyMinor: 25000,
		TargetDate: on(2026, time.December, 16)}
	got := Coach(g, now)
	if got.Kind != KindOnTrack {
		t.Fatalf("kind = %v, want on track", got.Kind)
	}
	if got.ExtraMonthlyMinor != 0 {
		t.Fatalf("an on-track goal was given an adjustment to make: %+v", got)
	}
}

func TestAGoalLandingEarlyIsNoticed(t *testing.T) {
	// Finishing ahead is the household's own doing, and noticing it is the
	// cheapest encouragement the app can offer.
	g := Goal{ID: "g", Name: "Laptop", TargetMinor: 100000, MonthlyMinor: 90000,
		TargetDate: on(2026, time.December, 16)}
	if got := Coach(g, now); got.Kind != KindAhead {
		t.Fatalf("kind = %v, want ahead", got.Kind)
	}
}

func TestAFundedGoalIsCelebratedOnce(t *testing.T) {
	g := Goal{ID: "g", Name: "Done", TargetMinor: 100000, SavedMinor: 100000,
		TargetDate: on(2026, time.December, 16)}
	if got := Coach(g, now); got.Kind != KindReached {
		t.Fatalf("kind = %v, want reached", got.Kind)
	}
	// And then left alone.
	g.LastCheckedAt = now.Add(-time.Hour)
	if got := Coach(g, now); !got.Silent() {
		t.Fatalf("a goal celebrated an hour ago said %v again", got.Kind)
	}
}

func TestCoachAllReturnsOnlyWhatIsWorthSayingMostUrgentFirst(t *testing.T) {
	goals := []Goal{
		{ID: "ok", Name: "On track", TargetMinor: 100000, MonthlyMinor: 25000, TargetDate: on(2026, time.December, 16)},
		{ID: "paused", Name: "Paused", TargetMinor: 100000, TargetDate: on(2026, time.December, 16), PausedUntil: on(2027, time.January, 1)},
		{ID: "small", Name: "Slightly behind", TargetMinor: 100000, MonthlyMinor: 24000, TargetDate: on(2026, time.December, 16)},
		{ID: "big", Name: "Well behind", TargetMinor: 400000, MonthlyMinor: 10000, TargetDate: on(2026, time.December, 16)},
		{ID: "none", Name: "No deadline", TargetMinor: 100000},
	}
	got := CoachAll(goals, now)
	if len(got) != 3 {
		t.Fatalf("notes = %d (%+v), want the two behind and the one on track", len(got), got)
	}
	if got[0].GoalID != "big" || got[1].GoalID != "small" {
		t.Fatalf("order = %s,%s — the biggest shortfall should lead", got[0].GoalID, got[1].GoalID)
	}
	if got[2].Kind != KindOnTrack {
		t.Fatalf("last note = %v, want the acknowledgement last", got[2].Kind)
	}
}

func TestWholeMonthsRoundsDownSoNoGoalIsToldItHasLonger(t *testing.T) {
	// 16 August to 15 December is three whole months and most of a fourth; telling
	// the household four would understate what they need to put aside each month.
	if got := wholeMonthsBetween(now, on(2026, time.December, 15)); got != 3 {
		t.Fatalf("months = %d, want 3", got)
	}
	if got := wholeMonthsBetween(now, on(2026, time.December, 16)); got != 4 {
		t.Fatalf("months = %d, want 4 on the exact day", got)
	}
}
