// SPDX-License-Identifier: MIT
package budgetview

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"time"
)

// MarkerState is the hover-intent and grace-window state of one marker.
type MarkerState struct{ Open, Pending, Closing bool }

// MarkerEvent identifies an input from the pointer, focus bridge, or timer.
type MarkerEvent uint8

const (
	// MarkerEnter cancels the close grace window and starts intent when shut.
	MarkerEnter MarkerEvent = iota
	// MarkerLeave cancels intent and arms the close grace window when open.
	MarkerLeave
	// MarkerIntent opens after the intent delay.
	MarkerIntent
	// MarkerGrace closes after the pointer leaves both trigger and portal.
	MarkerGrace
	// MarkerFocus starts intent without changing the grace window.
	MarkerFocus
	// MarkerBlur closes a preview when focus leaves the wrapper.
	MarkerBlur
	// MarkerToggle toggles the explainer for click/touch activation.
	MarkerToggle
	// MarkerAction closes the explainer before running a separate action.
	MarkerAction
	// MarkerDismiss closes only the explainer, preserving existing timer behavior.
	MarkerDismiss
)

// Next returns one deterministic marker transition; unknown events are inert.
func (s MarkerState) Next(event MarkerEvent) MarkerState {
	switch event {
	case MarkerEnter:
		s.Closing = false
		if !s.Open {
			s.Pending = true
		}
	case MarkerLeave:
		s.Pending = false
		if s.Open {
			s.Closing = true
		}
	case MarkerIntent:
		s.Pending, s.Open = false, true
	case MarkerGrace:
		s.Closing, s.Open = false, false
	case MarkerFocus:
		if !s.Open {
			s.Pending = true
		}
	case MarkerBlur:
		s.Pending, s.Open = false, false
	case MarkerToggle:
		s.Pending, s.Closing, s.Open = false, false, !s.Open
	case MarkerAction:
		s.Pending, s.Closing, s.Open = false, false, false
	case MarkerDismiss:
		s.Open = false
	}
	return s
}

// Class returns both modifiers in the original order.
func (s MarkerState) Class() string {
	out := ""
	if s.Pending {
		out = " is-waiting"
	}
	if s.Open {
		out += " is-open"
	}
	return out
}

// PeriodKeyStart resolves a persisted period key with explicit clock and week start.
func PeriodKeyStart(period domain.Period, ref, anchor, now time.Time, week time.Weekday) time.Time {
	if ref.IsZero() {
		ref = now
	}
	key := budgeting.PeriodKeyWithWeekStart(period, ref, anchor, week)
	start, err := dateutil.ParseDate(key)
	if err == nil {
		return start
	}
	start, _ = budgeting.PeriodRangeAnchored(period, ref, week, anchor)
	return start
}
