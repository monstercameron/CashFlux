// SPDX-License-Identifier: MIT

package subscriptions

import (
	"fmt"
	"strings"
)

// Confidence grades how sure the detector is that a pattern is a real
// subscription (task #52): Confirmed (the user said so), Likely (strong
// evidence), or Review (the user should look before the figure is trusted).
type Confidence string

const (
	ConfidenceConfirmed Confidence = "confirmed"
	ConfidenceLikely    Confidence = "likely"
	ConfidenceReview    Confidence = "review"
)

// Assessment is a detection's confidence tier plus the plain-English WHY —
// every reason names concrete evidence (charge count, cadence regularity,
// amount spread) so the tier is explainable, never a black box.
type Assessment struct {
	Level   Confidence
	Reasons []string
}

// Evidence thresholds for the Likely tier: at least likelyMinCount charges,
// amounts within likelyAmountVarPct of the median, gaps within the cadence's
// steady tolerance.
const (
	likelyMinCount     = 4
	likelyAmountVarPct = 10
)

// steadyGapTolerance is how many days a charge gap may drift from the median
// before the cadence stops reading as "steady".
func steadyGapTolerance(c Cadence) int {
	switch c {
	case CadenceWeekly:
		return 2
	case CadenceYearly:
		return 20
	default: // monthly
		return 4
	}
}

// ConfirmKey normalizes a subscription name for the confirmed-names set
// (case-insensitive, trimmed) — the same key rule the ignore list uses.
func ConfirmKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// Assess grades one detection. confirmedNames holds ConfirmKey'd names the
// user has explicitly confirmed; everything else is graded on evidence:
// enough charges + steady cadence + stable amounts → Likely, anything
// weaker → Review, with each signal spelled out either way.
func Assess(s Subscription, confirmedNames map[string]bool) Assessment {
	if confirmedNames[ConfirmKey(s.Name)] {
		return Assessment{Level: ConfidenceConfirmed, Reasons: []string{"You confirmed this subscription."}}
	}

	steady := s.GapVarDays <= steadyGapTolerance(s.Cadence)
	similar := s.AmountVarPct <= likelyAmountVarPct
	enough := s.Count >= likelyMinCount

	reasons := make([]string, 0, 3)
	if enough {
		reasons = append(reasons, fmt.Sprintf("%d charges from the same merchant", s.Count))
	} else {
		reasons = append(reasons, fmt.Sprintf("only %d charges so far", s.Count))
	}
	if steady {
		reasons = append(reasons, fmt.Sprintf("a steady %s cadence", s.Cadence))
	} else {
		reasons = append(reasons, fmt.Sprintf("renewal gaps vary by up to %d days", s.GapVarDays))
	}
	switch {
	case s.AmountVarPct == 0:
		reasons = append(reasons, "the same amount every time")
	case similar:
		reasons = append(reasons, fmt.Sprintf("amounts within %d%% of each other", s.AmountVarPct))
	default:
		reasons = append(reasons, fmt.Sprintf("amounts vary by up to %d%%", s.AmountVarPct))
	}

	if enough && steady && similar {
		return Assessment{Level: ConfidenceLikely, Reasons: reasons}
	}
	return Assessment{Level: ConfidenceReview, Reasons: reasons}
}

// ReasonLine joins an assessment's reasons into one sentence for tooltips and
// review rows ("6 charges from the same merchant · a steady monthly cadence ·
// the same amount every time").
func (a Assessment) ReasonLine() string { return strings.Join(a.Reasons, " · ") }
