package period

import "time"

// Window is the dashboard's current time selection: a resolution, a from/to
// anchor pair, and the week-start convention. It is an immutable value — the
// mutators return a new Window — so it drops straight into UI state. All the
// stepping/clamping rules live here (pure, tested) rather than in view code.
type Window struct {
	Res       Resolution
	From      time.Time
	To        time.Time
	WeekStart time.Weekday
}

// NewWindow builds a single-unit window around t at the given resolution, with
// both anchors snapped to the unit start.
func NewWindow(r Resolution, t time.Time, weekStart time.Weekday) Window {
	a := Truncate(r, t, weekStart)
	return Window{Res: r, From: a, To: a, WeekStart: weekStart}
}

// SetResolution switches the resolution and re-snaps both anchors to the new
// unit (so e.g. a month range collapses sensibly into quarters).
func (w Window) SetResolution(r Resolution) Window {
	return Window{
		Res:       r,
		From:      Truncate(r, w.From, w.WeekStart),
		To:        Truncate(r, w.To, w.WeekStart),
		WeekStart: w.WeekStart,
	}
}

// WithWeekStart returns the window using a different week-start convention,
// re-snapping both anchors to their unit under the new convention. Only Week
// resolution is sensitive to it; Month and Quarter anchors are unaffected. It
// is a no-op (returns the same window) when the convention is unchanged.
func (w Window) WithWeekStart(weekStart time.Weekday) Window {
	if weekStart == w.WeekStart {
		return w
	}
	return Window{
		Res:       w.Res,
		From:      Truncate(w.Res, w.From, weekStart),
		To:        Truncate(w.Res, w.To, weekStart),
		WeekStart: weekStart,
	}
}

// StepFrom moves the from anchor by delta units, pushing the to anchor forward
// if from would otherwise pass it (keeps from <= to).
func (w Window) StepFrom(delta int) Window {
	from := Step(w.Res, w.From, delta)
	to := w.To
	if to.Before(from) {
		to = from
	}
	return Window{Res: w.Res, From: from, To: to, WeekStart: w.WeekStart}
}

// StepTo moves the to anchor by delta units, pulling the from anchor back if to
// would otherwise fall before it (keeps from <= to).
func (w Window) StepTo(delta int) Window {
	to := Step(w.Res, w.To, delta)
	from := w.From
	if to.Before(from) {
		from = to
	}
	return Window{Res: w.Res, From: from, To: to, WeekStart: w.WeekStart}
}

// Range returns the half-open reporting range [start, end) the window covers.
func (w Window) Range() (start, end time.Time) {
	return Range(w.Res, w.From, w.To, w.WeekStart)
}

// FromLabel and ToLabel render the two anchors for the stepper pills.
func (w Window) FromLabel() string { return Label(w.Res, w.From, w.WeekStart) }
func (w Window) ToLabel() string   { return Label(w.Res, w.To, w.WeekStart) }
