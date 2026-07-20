// SPDX-License-Identifier: MIT

package budgeting

// OverTier grades HOW FAR past its cap an over-budget category is, so the row's
// over-budget treatment can scale with the overage instead of painting every
// overspend with one equally-severe full red bar. The magnitude is read from the
// spent-as-percent-of-cap figure (Status.Percent), which may exceed 100.
type OverTier int

const (
	// OverNone is a budget that is not past its cap (Percent < 100). Grading is only
	// meaningful once a row is actually over, so callers guard on the over state
	// first; this is the safe fallback otherwise.
	OverNone OverTier = iota
	// OverMild is a small overspend — under 10% past the cap (100–109%). A gentle
	// nudge, not an alarm: it earns a softer tone and a thinner overflow marker.
	OverMild
	// OverModerate is 10–24% past the cap (110–124%) — the baseline over treatment.
	OverModerate
	// OverSevere is a large overrun — 25% or more past the cap (>=125%). It earns the
	// heaviest tone and the widest overflow marker.
	OverSevere
)

// ClassifyOverage maps a spent-as-percent-of-cap figure to an OverTier. The
// thresholds mirror the design brief: mild is < 10% over the cap, severe is >= 25%
// over, and everything between is the moderate baseline. A percent below 100 is not
// over at all (OverNone).
func ClassifyOverage(percent int) OverTier {
	switch {
	case percent < 100:
		return OverNone
	case percent < 110:
		return OverMild
	case percent < 125:
		return OverModerate
	default:
		return OverSevere
	}
}

// Class returns the CSS modifier token the /budgets bar fill carries for this tier
// (empty for OverNone, so a not-over bar gains no extra class). The style layer
// (rules_budgetovercalm.go) grades the fill tone and overflow marker off these.
func (t OverTier) Class() string {
	switch t {
	case OverMild:
		return "over-mild"
	case OverModerate:
		return "over-mod"
	case OverSevere:
		return "over-severe"
	default:
		return ""
	}
}
