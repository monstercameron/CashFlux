package goals

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Pace is a compact, glanceable classification of where a goal stands, used to
// drive a colored badge and the progress-bar tone on the Goals screen (G5). It
// is derived only from values the app actually has — completion, percent, and
// the target date relative to "now" — never from a contribution-rate guess the
// app cannot know. Order of precedence: complete → overdue → final stretch →
// due soon → on track; goals without a target date have no pace.
type Pace string

const (
	// PaceNone applies to a goal with no target date — nothing to pace against.
	PaceNone Pace = ""
	// PaceComplete is a goal that has reached (or exceeded) its target.
	PaceComplete Pace = "complete"
	// PaceFinalStretch is an incomplete goal at or above 90% — nearly there.
	PaceFinalStretch Pace = "final"
	// PaceOverdue is an incomplete goal whose target date has passed.
	PaceOverdue Pace = "overdue"
	// PaceDueSoon is an incomplete goal whose target date is within the
	// dueSoonDays window and is not yet in the final stretch.
	PaceDueSoon Pace = "soon"
	// PaceOnTrack is a dated, incomplete goal with comfortable runway.
	PaceOnTrack Pace = "ontrack"
)

// dueSoonDays is how close a target date must be (with the goal still well
// short of done) to flag it as "due soon".
const dueSoonDays = 60

// finalStretchPct is the completion threshold at which a goal is "nearly there".
const finalStretchPct = 90

// ClassifyPace returns the Pace for a goal evaluated at the reference time from.
// It is pure and deterministic: same inputs, same output. A goal at or above
// 90% always reads as the final stretch even if its deadline is far off, so the
// most actionable goal surfaces first.
func ClassifyPace(goal domain.Goal, from time.Time) Pace {
	if complete, err := IsComplete(goal); err == nil && complete {
		return PaceComplete
	}
	pct := Percent(goal)
	if goal.TargetDate.IsZero() {
		// Undated: the only signal is progress.
		if pct >= finalStretchPct {
			return PaceFinalStretch
		}
		return PaceNone
	}
	if !goal.TargetDate.After(from) {
		return PaceOverdue
	}
	if pct >= finalStretchPct {
		return PaceFinalStretch
	}
	if goal.TargetDate.Before(from.AddDate(0, 0, dueSoonDays)) {
		return PaceDueSoon
	}
	return PaceOnTrack
}

// SortActive orders active (non-archived) goals for the Goals list so the most
// actionable goal lands first: nearest target date, then highest percent
// complete, then name. Goals without a target date sort after dated ones (a
// goal with a deadline is more time-sensitive than an open-ended one). The sort
// is stable in the caller via sort.SliceStable.
func LessForList(a, b domain.Goal) bool {
	ad, bd := a.TargetDate.IsZero(), b.TargetDate.IsZero()
	if ad != bd {
		return !ad // dated goals before undated
	}
	if !ad && !bd && !a.TargetDate.Equal(b.TargetDate) {
		return a.TargetDate.Before(b.TargetDate)
	}
	if pa, pb := Percent(a), Percent(b); pa != pb {
		return pa > pb // higher completion first
	}
	return a.Name < b.Name
}
