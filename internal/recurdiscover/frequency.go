// SPDX-License-Identifier: MIT

package recurdiscover

// A commitment charges you once per cycle. Habitual spending at one merchant
// does not — it lands whenever the household happens to go there, and the
// rhythm stage will still hand back SOME cadence for it, because a median gap
// always exists. "7 payments · yearly" is not a yearly commitment; it is a
// merchant visited seven times, and the cadence label is an artefact.
//
// So a candidate is checked against its own arithmetic: over the span it was
// actually observed, how many charges does its claimed cadence account for? A
// genuine monthly bill seen across two years has ~25 charges, and the ratio is
// 1.0 by construction. A cluster carrying several times what its rhythm can
// explain is describing frequency, not obligation.
const (
	// overFrequentFactor is how far past its cadence's own arithmetic a candidate
	// may run before it reads as habitual spending. Kept well clear of 1.0:
	// weekend shifts, early posting, and the occasional double charge all nudge a
	// real commitment slightly above its nominal count, and none of that should
	// cost it a place in the review queue.
	overFrequentFactor = 1.5
	// overFrequentMinCount is the smallest count worth judging this way. Below it
	// the expected-count arithmetic is dominated by the "+1" term (the first
	// occurrence), so the ratio says more about rounding than about behaviour.
	overFrequentMinCount = 4
)

// OverFrequent reports whether a candidate's occurrence count far exceeds what
// its detected cadence implies over the span it was observed — the signature of
// habitual spending at a merchant rather than a commitment owed on a schedule.
//
// It is deliberately a demotion signal, never a drop: an over-frequent pattern
// is still a real pattern, and the household may well want it tracked. It just
// does not belong at the top of a queue that asks "is this a bill?".
func OverFrequent(ev Evidence) bool {
	nominal := ev.Cadence.nominalGap()
	if nominal <= 0 || ev.Count < overFrequentMinCount {
		return false
	}
	span := daysBetween(ev.FirstSeen, ev.LastSeen)
	if span <= 0 {
		return false
	}
	// The span between the first and last sighting holds span/nominal gaps, and
	// one more charge than gaps.
	expected := float64(span)/nominal + 1
	return float64(ev.Count) > expected*overFrequentFactor
}
