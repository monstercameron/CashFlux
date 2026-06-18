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

// SetResolution switches the resolution and re-anchors to the single period
// that contains now (this week/month/quarter). It deliberately does NOT re-snap
// the existing window anchors: those sit at or before now, so truncating them to
// the new unit drifts the view backward in time and compounds across switches
// (e.g. Month "Jun" → Week would land on June's *first* week, not the current
// one) — the C41 bug. Anchoring on now matches the "This period" reset.
func (w Window) SetResolution(r Resolution, now time.Time) Window {
	return NewWindow(r, now, w.WeekStart)
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

// Shift moves the whole window by delta units of its resolution, preserving the
// span (both anchors move together) — paging a single period or a range back or
// forward as a unit. The anchors are assumed to sit at unit boundaries.
func (w Window) Shift(delta int) Window {
	return Window{
		Res:       w.Res,
		From:      Step(w.Res, w.From, delta),
		To:        Step(w.Res, w.To, delta),
		WeekStart: w.WeekStart,
	}
}

// IsCurrent reports whether the window is the single current period for its
// resolution at now — used to flag when the view has paged away from "now".
func (w Window) IsCurrent(now time.Time) bool {
	cur := Truncate(w.Res, now, w.WeekStart)
	return w.From.Equal(cur) && w.To.Equal(cur)
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

// IsSinglePeriod reports whether the window covers exactly one unit (its anchors
// coincide). The common case — "this month" — so the UI can show one label
// instead of a redundant "Jun 2026 – Jun 2026".
func (w Window) IsSinglePeriod() bool { return w.From.Equal(w.To) }

// Single collapses the window to the single period at its from anchor (To := From).
// It's the primary path for the redesigned control, where one period is the
// default and ranges are opt-in.
func (w Window) Single() Window {
	return Window{Res: w.Res, From: w.From, To: w.From, WeekStart: w.WeekStart}
}

// Label renders the window as one string: a single unit label when it covers one
// period ("Jun 2026"), or "from – to" for a multi-unit range ("Jun 2026 – Aug
// 2026"). This is what the redesigned single-stepper control shows.
func (w Window) Label() string {
	if w.IsSinglePeriod() {
		return w.FromLabel()
	}
	return w.FromLabel() + " – " + w.ToLabel()
}
