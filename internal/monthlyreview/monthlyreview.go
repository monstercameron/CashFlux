// SPDX-License-Identifier: MIT

// Package monthlyreview models the guided month-end review (AG10): which steps
// there are, which of them are worth showing this month, and where somebody is up
// to in them.
//
// Copilot and Origin bet their products on the monthly review, and the reason
// theirs work is not the content — it is that each step ends in something. A review
// that shows five charts and stops is a report; a review where each step ends in a
// change applied or an explicit skip is a ritual people finish.
//
// The other half of finishing is being interruptible. A ten-minute review nobody
// has ten minutes for gets abandoned at step two and never reopened, so progress
// is a value that persists and every step can be skipped without penalty. Skipping
// is a first-class outcome here, not a failure state: the point is to reach the end
// having decided about each step, and "not this month" is a decision.
//
// Pure: no syscall/js, no appstate. Table-tested on native Go.
package monthlyreview

import (
	"fmt"
	"time"
)

// Step is one stage of the review.
type Step string

const (
	// StepRecap opens with what the month cost against what is typical.
	StepRecap Step = "recap"
	// StepFindings is what the app noticed and has not been told about.
	StepFindings Step = "findings"
	// StepBudgets is the budgets that need truing up.
	StepBudgets Step = "budgets"
	// StepGoals is the goals that need a decision.
	StepGoals Step = "goals"
	// StepClose is the summary of what the review actually changed.
	StepClose Step = "close"
)

// order is the sequence of steps. It runs from what happened, through what to do
// about it, to what was done — orientation before decisions, and never the other
// way round: asking somebody to adjust a budget before showing them the month is
// asking them to decide blind.
var order = []Step{StepRecap, StepFindings, StepBudgets, StepGoals, StepClose}

// Order returns the review's steps in sequence.
func Order() []Step { return append([]Step(nil), order...) }

// Availability says whether each step has anything to show this month. A step with
// nothing in it is SKIPPED rather than shown empty: "no budgets need attention" is
// worth one line in the close, not a screen of its own with a Next button.
type Availability struct {
	// HasFindings reports whether anything is flagged and undismissed.
	HasFindings bool
	// BudgetsNeedingAttention is how many budgets are over or near their limit.
	BudgetsNeedingAttention int
	// GoalsNeedingAttention is how many goals the coach has something to say about.
	GoalsNeedingAttention int
}

// Progress is where somebody is up to, and what they have decided so far. It is
// the value that persists between sittings.
type Progress struct {
	// Month identifies which month's review this is ("2026-08"). Progress from a
	// previous month never carries into a new one.
	Month string
	// StepIndex is the position in the visible-step list.
	StepIndex int
	// Done marks the review as finished for this month.
	Done bool
	// DismissedAt is when the household closed the review without finishing. A
	// dismissal is respected for the rest of the month — offering it again the next
	// day is exactly the nagging the app promises not to do.
	DismissedAt time.Time
	// Skipped records which steps were passed over, so the close can say what was
	// left rather than implying everything was handled.
	Skipped []Step
}

// MonthKey renders a time as the key a review is stored under.
func MonthKey(t time.Time) string { return t.UTC().Format("2006-01") }

// VisibleSteps returns the steps worth showing this month, in order. The recap
// and the close are always present: the recap is the orientation the rest depends
// on, and the close is what makes the review feel finished rather than abandoned.
func VisibleSteps(a Availability) []Step {
	out := make([]Step, 0, len(order))
	for _, s := range order {
		switch s {
		case StepFindings:
			if !a.HasFindings {
				continue
			}
		case StepBudgets:
			if a.BudgetsNeedingAttention == 0 {
				continue
			}
		case StepGoals:
			if a.GoalsNeedingAttention == 0 {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

// Worthwhile reports whether the review is worth offering at all. A month where
// nothing is flagged, no budget is off and no goal needs a decision has nothing to
// review, and opening a ritual to say so wastes the ten minutes it asked for.
func Worthwhile(a Availability) bool {
	return a.HasFindings || a.BudgetsNeedingAttention > 0 || a.GoalsNeedingAttention > 0
}

// State is the resolved position: which steps exist, where in them somebody is,
// and what the controls should offer.
type State struct {
	Steps   []Step
	Index   int
	Current Step
	// Done reports that the review is finished for this month.
	Done bool
	// Dismissed reports that it was closed early and should stay closed.
	Dismissed bool
}

// AtLast reports whether the current step is the final one.
func (s State) AtLast() bool { return s.Index >= len(s.Steps)-1 }

// Resolve works out the state from stored progress and this month's availability.
//
// Progress from a DIFFERENT month is discarded rather than carried forward: a
// half-finished July review has nothing to say about August, and resuming into it
// would show last month's figures under this month's heading. The index is clamped
// because the set of visible steps can shrink between sittings — a budget trued up
// on Tuesday removes its own step by Thursday.
func Resolve(p Progress, a Availability, now time.Time) State {
	steps := VisibleSteps(a)
	st := State{Steps: steps}
	if len(steps) == 0 {
		st.Done = true
		return st
	}
	if p.Month != MonthKey(now) {
		st.Current = steps[0]
		return st
	}
	st.Done = p.Done
	st.Dismissed = !p.DismissedAt.IsZero()
	st.Index = p.StepIndex
	if st.Index < 0 {
		st.Index = 0
	}
	if st.Index > len(steps)-1 {
		st.Index = len(steps) - 1
	}
	st.Current = steps[st.Index]
	return st
}

// Advance moves to the next step, marking the review done at the end. skipped
// records that the step was passed over rather than acted on.
func Advance(p Progress, a Availability, now time.Time, skipped bool) Progress {
	st := Resolve(p, a, now)
	next := p
	next.Month = MonthKey(now)
	if p.Month != MonthKey(now) {
		// A new month starts clean, including its skip list.
		next.StepIndex, next.Done, next.Skipped, next.DismissedAt = 0, false, nil, time.Time{}
		st = Resolve(next, a, now)
	}
	if skipped && st.Current != "" {
		next.Skipped = appendUnique(next.Skipped, st.Current)
	}
	if st.AtLast() {
		next.Done = true
		return next
	}
	next.StepIndex = st.Index + 1
	return next
}

// Dismiss closes the review for the rest of the month without finishing it.
func Dismiss(p Progress, now time.Time) Progress {
	p.Month = MonthKey(now)
	p.DismissedAt = now
	return p
}

// Reopen clears a dismissal so the review can be picked up again — the household
// asked for it back, which outranks their earlier "not now".
func Reopen(p Progress) Progress {
	p.DismissedAt = time.Time{}
	p.Done = false
	return p
}

// ShouldOffer reports whether the review should be offered right now. It is the
// one place the never-naggy rule is enforced, so there is exactly one answer to
// "why am I seeing this?".
func ShouldOffer(p Progress, a Availability, now time.Time) bool {
	if !Worthwhile(a) {
		return false
	}
	if p.Month != MonthKey(now) {
		return true
	}
	return p.DismissedAt.IsZero() && !p.Done
}

// CloseSummary describes how the review ended, in the household's terms.
func CloseSummary(p Progress, a Availability, now time.Time) string {
	steps := VisibleSteps(a)
	handled := len(steps) - len(p.Skipped)
	if handled < 0 {
		handled = 0
	}
	switch {
	case len(p.Skipped) == 0:
		return "You went through everything."
	case handled == 0:
		return "You skipped the lot — that's a decision too. It'll be here next month."
	default:
		return fmt.Sprintf("You handled %d and left %d for another time.", handled, len(p.Skipped))
	}
}

// appendUnique adds a step to a list once.
func appendUnique(list []Step, s Step) []Step {
	for _, e := range list {
		if e == s {
			return list
		}
	}
	return append(list, s)
}
