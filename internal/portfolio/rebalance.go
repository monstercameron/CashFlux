// SPDX-License-Identifier: MIT

package portfolio

import (
	"math"
	"sort"
)

// ─── C379: targets, drift, and moves nobody has to make ──────────────────────
//
// A target allocation is a household's own statement of how it wants to be
// invested. Drift is the gap between that and where the market has left them,
// and the suggested moves are the arithmetic that closes it.
//
// Everything here is VIRTUAL, in the same sense as the goals set-asides: the app
// has no brokerage connection and will never place a trade, so a "move" is a
// sentence describing what the household could do, not an instruction it can
// carry out. Naming that plainly in the model — rather than letting a view
// decide how to phrase it — is what keeps the feature honest.

// Target is a household's desired weight for one asset class.
type Target struct {
	AssetClass string
	Pct        float64 // desired share of portfolio value, 0–100
}

// TargetsValid reports whether a target set is usable: every weight
// non-negative and the whole summing to about 100.
//
// "About" is 0.05pp of slack, so a set entered as thirds (33.3/33.3/33.4) is
// accepted. Rejecting that would make the feature unusable for the most obvious
// case anyone tries first.
func TargetsValid(ts []Target) bool {
	if len(ts) == 0 {
		return false
	}
	var sum float64
	seen := make(map[string]bool, len(ts))
	for _, t := range ts {
		if t.Pct < 0 || t.AssetClass == "" || seen[t.AssetClass] {
			return false
		}
		seen[t.AssetClass] = true
		sum += t.Pct
	}
	return math.Abs(sum-100) <= 0.05
}

// Drift is one asset class's distance from its target.
type Drift struct {
	AssetClass string
	// TargetPct / CurrentPct are the desired and actual shares.
	TargetPct, CurrentPct float64
	// DriftPct is CurrentPct − TargetPct: positive means overweight.
	DriftPct float64
	// CurrentMinor is what is held, TargetMinor what the target implies at the
	// portfolio's current total, and DeltaMinor the signed difference —
	// NEGATIVE means "this much would move out", positive "this much would move
	// in". A view can render the sentence; the sign is decided here so two views
	// cannot disagree about which way the money goes.
	CurrentMinor, TargetMinor, DeltaMinor int64
}

// Overweight reports whether the class holds more than its target.
func (d Drift) Overweight() bool { return d.DeltaMinor < 0 }

// Plan is the whole rebalancing read: per-class drift plus the total that would
// change hands.
type Plan struct {
	Drifts []Drift
	// TotalMinor is the sum of the positive deltas — the money that would move,
	// counted once rather than twice (every dollar leaving one class arrives in
	// another, so adding both sides would double it).
	TotalMinor int64
	// MaxDriftPct is the largest absolute drift, the single number that answers
	// "how far off am I".
	MaxDriftPct float64
}

// Balanced reports whether every class sits within tolerance of its target.
func (p Plan) Balanced(tolerancePct float64) bool { return p.MaxDriftPct <= tolerancePct }

// Rebalance computes drift against a target set.
//
// Classes appear if they are in the targets OR held, so a position the household
// never planned for shows up as pure overweight rather than being silently
// dropped — the case where drift matters most. A holding's blank asset class
// buckets as "other", matching AllocationByAssetClass, so the two views agree.
func Rebalance(hs []Holding, targets []Target) Plan {
	current := make(map[string]int64)
	var total int64
	for _, h := range hs {
		cls := h.AssetClass
		if cls == "" {
			cls = "other"
		}
		v := HoldingValueMinor(h)
		current[cls] += v
		total += v
	}
	targetPct := make(map[string]float64, len(targets))
	for _, t := range targets {
		targetPct[t.AssetClass] = t.Pct
	}

	classes := make([]string, 0, len(current)+len(targetPct))
	seen := make(map[string]bool, len(current)+len(targetPct))
	for cls := range current {
		if !seen[cls] {
			seen[cls] = true
			classes = append(classes, cls)
		}
	}
	for cls := range targetPct {
		if !seen[cls] {
			seen[cls] = true
			classes = append(classes, cls)
		}
	}

	var out Plan
	for _, cls := range classes {
		cur := current[cls]
		tp := targetPct[cls]
		var curPct float64
		if total != 0 {
			curPct = float64(cur) / float64(total) * 100
		}
		want := int64(math.Round(float64(total) * tp / 100))
		d := Drift{
			AssetClass:   cls,
			TargetPct:    tp,
			CurrentPct:   curPct,
			DriftPct:     curPct - tp,
			CurrentMinor: cur,
			TargetMinor:  want,
			DeltaMinor:   want - cur,
		}
		if a := math.Abs(d.DriftPct); a > out.MaxDriftPct {
			out.MaxDriftPct = a
		}
		if d.DeltaMinor > 0 {
			out.TotalMinor += d.DeltaMinor
		}
		out.Drifts = append(out.Drifts, d)
	}
	// Largest drift first: the row worth acting on leads.
	sort.SliceStable(out.Drifts, func(i, j int) bool {
		di, dj := math.Abs(out.Drifts[i].DriftPct), math.Abs(out.Drifts[j].DriftPct)
		if di != dj {
			return di > dj
		}
		return out.Drifts[i].AssetClass < out.Drifts[j].AssetClass
	})
	return out
}
