// SPDX-License-Identifier: MIT

// Package whyover decides WHY one budget is over (SM-4).
//
// "Over by $180" is a fact the meter already shows. The useful sentence is the
// one after it — whether the month was blown by a single charge, by shopping more
// often than usual, by the same shopping costing more, or by nothing in
// particular. Each of those has a different fix, and guessing wrong sends the user
// to the wrong one.
//
// This package holds ONLY that judgment. The arithmetic it judges — spend, limit,
// pace, the per-merchant drivers — is already owned by internal/budgeting, and the
// caller passes the results in, the way internal/catsuggest takes the rules
// engine's answer rather than running it. That keeps the thresholds here readable
// and testable in isolation, which is the whole point of separating them.
//
// The result is DATA, never a sentence: the UI formats it through en_*.go so the
// copy stays translatable and the screenlint ratchet stays at zero. Pure Go, no
// syscall/js, unit-tested on native Go.
package whyover

// Reason is the dominant explanation for an overage.
type Reason int

const (
	// ReasonNone: nothing to explain.
	ReasonNone Reason = iota
	// ReasonOneCharge: a single merchant accounts for most of the overage. The
	// fix is about that one purchase, not about the household's habits.
	ReasonOneCharge
	// ReasonMoreOften: about the same size of charge, but noticeably more of them.
	ReasonMoreOften
	// ReasonPricier: about the same number of charges, each costing more.
	ReasonPricier
	// ReasonEarlyPace: not over yet, but spending this fast lands over by the end
	// of the period. The only reason that fires BEFORE the limit is breached.
	ReasonEarlyPace
	// ReasonSteady: genuinely spread out — no single charge, count change, or
	// price change explains it. Saying so is more honest than inventing a cause.
	ReasonSteady
)

// String returns a stable, non-localized token for logs and test names. The
// user-facing sentence lives in the i18n layer, never here.
func (r Reason) String() string {
	switch r {
	case ReasonOneCharge:
		return "one-charge"
	case ReasonMoreOften:
		return "more-often"
	case ReasonPricier:
		return "pricier"
	case ReasonEarlyPace:
		return "early-pace"
	case ReasonSteady:
		return "steady"
	}
	return "none"
}

// Thresholds. They are named constants rather than inline numbers because each
// one is a product judgment about when a signal is strong enough to lead with.
const (
	// oneChargeShareBP: the top merchant must account for this much of the
	// overage before the overage is "that one purchase".
	oneChargeShareBP = 6000
	// countJumpMin: how many extra charges read as "more often" at all.
	countJumpMin = 2
	// countJumpBP: ...and by what proportion, so a household with 40 small
	// charges is not flagged for having 42.
	countJumpBP = 12500
	// avgJumpBP: how much bigger the average charge must be to read as "pricier".
	avgJumpBP = 12500
	// bpScale is the basis-point denominator (10000 = 100%).
	bpScale = 10000
)

// Input is the already-computed picture of one budget's period. All money is
// positive magnitude in minor units of the budget's own currency — the caller
// converts, so this package never touches a rate table.
type Input struct {
	// LimitMinor is the budget's limit; SpentMinor is spend to date.
	LimitMinor, SpentMinor int64
	// TopDriverMinor is the largest single MERCHANT's contribution this period
	// (budgeting.TopDrivers aggregates a merchant's charges, so ten coffee runs
	// count as one driver — which is what "one charge blew it" should mean).
	TopDriverMinor int64
	// Count is how many charges contributed this period.
	Count int
	// PriorSpentMinor / PriorCount describe the SAME budget over the previous
	// comparable period. Both zero means there is no comparison to draw, and the
	// count/price reasons are skipped rather than computed against nothing.
	PriorSpentMinor int64
	PriorCount      int
	// ElapsedBP is how far through the period we are (0..10000), and
	// ProjectedMinor is the end-of-period forecast at the current rate. Together
	// they drive ReasonEarlyPace.
	ElapsedBP      int
	ProjectedMinor int64
}

// Result is the judgment plus the figures behind it, so the UI can state the
// evidence rather than assert the conclusion.
type Result struct {
	Reason Reason
	// OverMinor is how far past the limit spending already is (0 when not over).
	OverMinor int64
	// ProjectedOverMinor is how far past the limit the period is forecast to land
	// (0 when the forecast stays within). Only meaningful for ReasonEarlyPace.
	ProjectedOverMinor int64
	// TopDriverShareBP is the top merchant's share of the OVERAGE when over, or
	// of total spend when the finding is a pace warning.
	TopDriverShareBP int
	// CountDelta is this period's charge count minus the prior period's.
	CountDelta int
	// AvgMinor / PriorAvgMinor are the average charge each period. Zero when the
	// corresponding count is zero.
	AvgMinor, PriorAvgMinor int64
	// HasComparison reports whether a prior period was supplied at all. Without
	// it, ReasonMoreOften and ReasonPricier are unreachable by construction, and
	// the UI should not imply a comparison it never made.
	HasComparison bool
}

// Explain returns the dominant reason for the overage, or ok=false when there is
// nothing to explain — the budget is within its limit and not projected to breach
// it, or it has no limit at all.
func Explain(in Input) (Result, bool) {
	if in.LimitMinor <= 0 {
		return Result{}, false
	}
	res := Result{HasComparison: in.PriorCount > 0 || in.PriorSpentMinor > 0}
	if in.Count > 0 {
		res.AvgMinor = in.SpentMinor / int64(in.Count)
	}
	if in.PriorCount > 0 {
		res.PriorAvgMinor = in.PriorSpentMinor / int64(in.PriorCount)
	}
	res.CountDelta = in.Count - in.PriorCount

	over := in.SpentMinor - in.LimitMinor
	if over <= 0 {
		// Not over yet. The only finding available is that the current rate lands
		// over — and only once enough of the period has elapsed for the linear
		// projection to mean anything. Very early in a period one large purchase
		// projects to an absurd total, and leading with that is crying wolf.
		projOver := in.ProjectedMinor - in.LimitMinor
		if projOver <= 0 || in.ElapsedBP < 2500 {
			return Result{}, false
		}
		res.Reason = ReasonEarlyPace
		res.ProjectedOverMinor = projOver
		if in.SpentMinor > 0 {
			res.TopDriverShareBP = int(in.TopDriverMinor * bpScale / in.SpentMinor)
		}
		return res, true
	}

	res.OverMinor = over
	res.TopDriverShareBP = int(in.TopDriverMinor * bpScale / over)
	// Capping keeps the share readable: a single merchant can easily exceed the
	// overage itself (spend 300 over a 100 limit on one charge), and "412% of the
	// overage" is a true statement nobody can use.
	if res.TopDriverShareBP > bpScale {
		res.TopDriverShareBP = bpScale
	}

	// 1. One purchase did it. Checked first because when it is true, the count and
	// price comparisons are describing the noise around it.
	if in.TopDriverMinor > 0 && res.TopDriverShareBP >= oneChargeShareBP {
		res.Reason = ReasonOneCharge
		return res, true
	}

	// 2 & 3 need something to compare against.
	if in.PriorCount > 0 && in.Count > 0 {
		countJump := res.CountDelta >= countJumpMin &&
			int64(in.Count)*bpScale >= int64(in.PriorCount)*countJumpBP
		avgJump := res.PriorAvgMinor > 0 && res.AvgMinor*bpScale >= res.PriorAvgMinor*avgJumpBP

		// More often, without the price moving enough to claim otherwise. The test
		// is deliberately NOT "and the average held steady": an average that drifted
		// a little while the trip count went up 2.5× is still a count story, and
		// requiring a tight band on the average sent that case to "no single cause".
		if countJump && !avgJump {
			res.Reason = ReasonMoreOften
			return res, true
		}
		// The same trips, costing more.
		if avgJump && !countJump {
			res.Reason = ReasonPricier
			return res, true
		}
		// Both moved: lead with whichever moved further, so the sentence still
		// names the bigger lever instead of falling through to "no single cause".
		if countJump && avgJump {
			countBP := int64(in.Count) * bpScale / int64(in.PriorCount)
			avgBP := res.AvgMinor * bpScale / res.PriorAvgMinor
			if countBP >= avgBP {
				res.Reason = ReasonMoreOften
			} else {
				res.Reason = ReasonPricier
			}
			return res, true
		}
	}

	res.Reason = ReasonSteady
	return res, true
}
