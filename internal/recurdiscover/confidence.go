// SPDX-License-Identifier: MIT

package recurdiscover

import "time"

// Confidence tier thresholds on the combined 0..1 score.
const (
	likelyThreshold = 0.60
	reviewThreshold = 0.33
)

// scoreConfidence combines the four signals — rhythm fit × cost stability ×
// occurrence-count sufficiency × liveness — into a 0..1 confidence and a review
// tier. count is the occurrence count, floor the evidence floor for the cadence,
// and last/now bound liveness (has the most recent expected occurrence arrived?).
// A pattern that has missed two or more expected cycles is forced to Silent
// (it reads as stopped).
func scoreConfidence(rh rhythm, cost costResult, count, floor int, last, now time.Time) (float64, Tier) {
	countFactor := clamp01(float64(count) / float64(floor+3))
	cyclesLate := lateCycles(rh.cadence, last, now)

	var liveness float64
	switch {
	case cyclesLate <= 1.3:
		liveness = 1.0
	case cyclesLate <= 2.3:
		liveness = 0.6
	default:
		liveness = 0.3
	}

	conf := rh.fit * cost.stability * countFactor * liveness

	var tier Tier
	switch {
	case count < floor:
		tier = TierSilent
	case cyclesLate > 2.3:
		tier = TierSilent // appears stopped
	case conf >= likelyThreshold:
		tier = TierLikely
	case conf >= reviewThreshold:
		tier = TierNeedsReview
	default:
		tier = TierSilent
	}
	return conf, tier
}

// lateCycles returns how many cadence cycles have elapsed since the last
// occurrence relative to now, as a fraction (1.0 = exactly one cadence period
// late). A non-positive nominal gap or a now before last yields 0.
func lateCycles(c Cadence, last, now time.Time) float64 {
	nominal := c.nominalGap()
	if nominal <= 0 {
		return 0
	}
	elapsed := float64(daysBetween(last, now))
	if elapsed <= 0 {
		return 0
	}
	return elapsed / nominal
}
