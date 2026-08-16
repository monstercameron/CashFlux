// SPDX-License-Identifier: MIT

// Package goalcoach works out what to say about a savings goal's progress, and
// when to say nothing at all (AG15).
//
// The hard part of goal coaching is not the arithmetic — it is restraint. A goal
// tracker that comments every time you look at it becomes something you stop
// looking at, and the app's own rule is "friendly, never naggy". So this package
// is written around the silences: a paused goal is never nagged, a goal without a
// deadline cannot be behind, a goal seen recently is left alone, and a goal that
// is simply on track gets a quiet acknowledgement rather than advice.
//
// What it produces is a PROPOSAL, never a change. The adjustment maths ("$50 more
// a month finishes by your birthday") is the same maths the goal slider uses; the
// difference is that here it arrives as a sentence somebody can decline.
//
// Pure: no syscall/js, no appstate. Table-tested on native Go.
package goalcoach

import (
	"time"
)

// Goal is one savings goal, reduced to what coaching needs.
type Goal struct {
	ID   string
	Name string
	// TargetMinor is what the goal needs in total; SavedMinor is what it holds.
	TargetMinor, SavedMinor int64
	// MonthlyMinor is the household's planned contribution, or 0 when unplanned.
	MonthlyMinor int64
	// TargetDate is when they want it done. Zero means no deadline — a goal with
	// no date cannot be behind, and saying it is would be inventing a commitment
	// the household never made.
	TargetDate time.Time
	// PausedUntil suspends coaching entirely until that date. A paused goal is a
	// decision, not a lapse.
	PausedUntil time.Time
	// Archived marks a goal that is finished or abandoned.
	Archived bool
	// LastCheckedAt is when this goal was last coached, so the same nudge is not
	// repeated every time the screen is opened.
	LastCheckedAt time.Time
}

// Kind is what the coach has to say.
type Kind int

const (
	// KindSilent means say nothing. It is the most common answer and the whole
	// reason this package exists as a decision rather than a template.
	KindSilent Kind = iota
	// KindOnTrack is a quiet acknowledgement that the goal will land on time.
	KindOnTrack
	// KindAhead is a quiet acknowledgement that it will land early.
	KindAhead
	// KindBehind proposes an adjustment that would bring the date back.
	KindBehind
	// KindUnfunded notes that a goal with a deadline has no monthly contribution
	// planned, which is the one case where "nothing is happening" is the finding.
	KindUnfunded
	// KindReached says the goal is fully funded.
	KindReached
)

// Note is what the coach decided to say.
type Note struct {
	Kind Kind
	// GoalID and GoalName identify the goal.
	GoalID, GoalName string
	// ShortfallMinor is how much the goal is short at the deadline, on the current
	// plan. Set on KindBehind.
	ShortfallMinor int64
	// SuggestedMonthlyMinor is the contribution that would make the deadline, and
	// ExtraMonthlyMinor is how much more that is than the current plan. Both set on
	// KindBehind and KindUnfunded — the proposal, never applied.
	SuggestedMonthlyMinor, ExtraMonthlyMinor int64
	// MonthsRemaining is how many whole months are left before the deadline.
	MonthsRemaining int
	// ProjectedFinish is when the goal lands on the CURRENT plan, when that can be
	// worked out. Zero when the plan never finishes it.
	ProjectedFinish time.Time
}

// Silent reports whether the coach decided to say nothing.
func (n Note) Silent() bool { return n.Kind == KindSilent }

// quietPeriod is how long a goal is left alone after it was last coached. Two
// weeks is long enough that a nudge is news again, short enough to still be useful
// against a monthly contribution rhythm.
const quietPeriod = 14 * 24 * time.Hour

// Coach decides what to say about one goal, or that nothing should be said.
//
// The order of the checks IS the policy, and each early return is a deliberate
// silence rather than a missing case:
//
//  1. archived — the goal is over
//  2. paused — the household already decided to stop, and coaching it is nagging
//  3. reached — celebrate once, quietly
//  4. seen recently — the same nudge twice is noise
//  5. no deadline — cannot be behind, so there is nothing to be behind ON
//  6. deadline passed — there is nothing left to adjust; the household knows
func Coach(g Goal, now time.Time) Note {
	base := Note{GoalID: g.ID, GoalName: g.Name}
	if g.Archived {
		return base
	}
	if !g.PausedUntil.IsZero() && now.Before(g.PausedUntil) {
		return base
	}
	if g.TargetMinor > 0 && g.SavedMinor >= g.TargetMinor {
		if recentlyChecked(g, now) {
			return base
		}
		base.Kind = KindReached
		return base
	}
	if recentlyChecked(g, now) {
		return base
	}
	if g.TargetDate.IsZero() {
		return base
	}
	months := wholeMonthsBetween(now, g.TargetDate)
	if months <= 0 {
		// The deadline is here or gone. Proposing a monthly increase for a goal
		// that is already due would be arithmetic pretending to be advice.
		return base
	}
	base.MonthsRemaining = months

	remaining := g.TargetMinor - g.SavedMinor
	if remaining <= 0 {
		base.Kind = KindReached
		return base
	}
	needed := ceilDiv(remaining, int64(months))

	if g.MonthlyMinor <= 0 {
		base.Kind = KindUnfunded
		base.SuggestedMonthlyMinor = needed
		base.ExtraMonthlyMinor = needed
		base.ShortfallMinor = remaining
		return base
	}

	projected := g.MonthlyMinor * int64(months)
	switch {
	case projected >= remaining:
		base.Kind = KindOnTrack
		base.ProjectedFinish = now.AddDate(0, monthsToFinish(remaining, g.MonthlyMinor), 0)
		// Landing a whole month or more early is worth saying differently: it is
		// the household's own doing, and noticing it is the cheapest encouragement
		// the app can offer.
		if monthsToFinish(remaining, g.MonthlyMinor) < months {
			base.Kind = KindAhead
		}
		return base
	default:
		base.Kind = KindBehind
		base.ShortfallMinor = remaining - projected
		base.SuggestedMonthlyMinor = needed
		base.ExtraMonthlyMinor = needed - g.MonthlyMinor
		base.ProjectedFinish = now.AddDate(0, monthsToFinish(remaining, g.MonthlyMinor), 0)
		return base
	}
}

// CoachAll runs the coach over a set of goals and returns only what is worth
// saying, most urgent first. A screen showing every note would be the naggy
// version of this feature; a screen showing the one that matters is the useful
// one.
func CoachAll(goals []Goal, now time.Time) []Note {
	out := make([]Note, 0, len(goals))
	for _, g := range goals {
		if n := Coach(g, now); !n.Silent() {
			out = append(out, n)
		}
	}
	// Behind first, then unfunded, then the acknowledgements. Within a kind, the
	// larger shortfall leads.
	rank := map[Kind]int{KindBehind: 0, KindUnfunded: 1, KindReached: 2, KindAhead: 3, KindOnTrack: 4}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, b := out[j-1], out[j]
			if rank[a.Kind] < rank[b.Kind] ||
				(rank[a.Kind] == rank[b.Kind] && a.ShortfallMinor >= b.ShortfallMinor) {
				break
			}
			out[j-1], out[j] = b, a
		}
	}
	return out
}

// recentlyChecked reports whether this goal was coached inside the quiet period.
func recentlyChecked(g Goal, now time.Time) bool {
	return !g.LastCheckedAt.IsZero() && now.Sub(g.LastCheckedAt) < quietPeriod
}

// wholeMonthsBetween counts the whole months from now until the deadline. It
// rounds DOWN, so a goal is never told it has more time than it does.
func wholeMonthsBetween(now, target time.Time) int {
	if !target.After(now) {
		return 0
	}
	months := (target.Year()-now.Year())*12 + int(target.Month()) - int(now.Month())
	if target.Day() < now.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}

// monthsToFinish is how many months the current contribution needs to cover what
// is left, rounded up.
func monthsToFinish(remaining, monthly int64) int {
	if monthly <= 0 {
		return 0
	}
	return int(ceilDiv(remaining, monthly))
}

// ceilDiv divides rounding up, so a contribution never lands a penny short.
func ceilDiv(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	q := a / b
	if a%b != 0 {
		q++
	}
	return q
}
