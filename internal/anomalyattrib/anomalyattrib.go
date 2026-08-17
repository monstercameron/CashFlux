// SPDX-License-Identifier: MIT

// Package anomalyattrib says how much of an overspend a few purchases explain
// (WF-SM1).
//
// The app already detects that a category is above its usual and gathers the
// matching transactions. What it could not say is the sentence the ticket asks
// for: "three purchases explain 82% of it".
//
// # Why that sentence matters more than the flag
//
// "Dining is $135 above normal" prompts one question — what did I buy — and
// leaves the reader to scan a list and do arithmetic. Answering it turns a flag
// into a finding.
//
// But the more useful half is the case where the answer is NO. If the overspend
// is spread across thirty ordinary purchases, there is no culprit to find, and a
// flag that implies there is sends somebody hunting for a mistake that does not
// exist. One big purchase is a one-off; thirty small ones are a habit, and they
// call for completely different responses. A tool that cannot tell them apart is
// worse than one that stays quiet.
package anomalyattrib

import "sort"

// ExplainedPct is the share of an overspend a set of purchases must account for
// before it counts as explaining it.
//
// Sixty percent. Below that the "explanation" leaves most of the overspend
// unaccounted for, and presenting it as the cause would point at the wrong
// thing with the confidence of a diagnosis.
const ExplainedPct = 60.0

// MaxCulprits is the largest number of purchases that can still be called a
// culprit.
//
// Five. Beyond that "these purchases explain it" stops being a shortcut and
// becomes a list the reader has to work through anyway — at which point the
// honest answer is that the spending was diffuse.
const MaxCulprits = 5

// Purchase is one transaction considered as a possible cause.
type Purchase struct {
	ID     string
	Payee  string
	Minor  int64 // magnitude, always positive
	Larger bool  // materially larger than this payee's usual
}

// Attribution is what explains an overspend, or the finding that nothing does.
type Attribution struct {
	// Culprits are the purchases that account for the overspend, largest first.
	// Empty when the spending was diffuse.
	Culprits []Purchase
	// ExplainedPct is the share of the overspend the culprits account for.
	ExplainedPct float64
	// Concentrated says a small number of purchases explain most of it.
	//
	// The distinction the whole package exists for: concentrated means go and
	// look at those; diffuse means the habit moved, and there is nothing to point
	// at.
	Concentrated bool
	// UnusuallyLarge counts culprits that are materially bigger than that payee's
	// usual — the ones worth a second look even inside a concentrated finding.
	UnusuallyLarge int
	// Everything says the culprits account for the whole overspend — removing
	// them would have left the period at or below its usual.
	Everything bool
	// Known is false when there is nothing to attribute: no overspend, or no
	// purchases to attribute it to.
	Known bool
}

// Explain works out what accounts for an overspend.
//
// overMinor is how far above normal the period ran; purchases are that period's
// spending in the category. Reports Known=false rather than an empty
// attribution when there is nothing to explain — "0% explained" reads as a
// failed analysis of a real problem, when in fact there was no problem.
func Explain(overMinor int64, purchases []Purchase) Attribution {
	var a Attribution
	if overMinor <= 0 || len(purchases) == 0 {
		return a
	}
	sorted := append([]Purchase(nil), purchases...)
	// Largest first, ties by ID so the same period always attributes identically.
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Minor != sorted[j].Minor {
			return sorted[i].Minor > sorted[j].Minor
		}
		return sorted[i].ID < sorted[j].ID
	})

	a.Known = true
	var running int64
	for i, p := range sorted {
		if i >= MaxCulprits {
			break
		}
		running += p.Minor
		a.Culprits = append(a.Culprits, p)
		if pct(running, overMinor) >= ExplainedPct {
			break
		}
	}
	a.ExplainedPct = pct(running, overMinor)
	// A purchase cannot explain more than the whole of an overspend. Uncapped,
	// a category whose entire month is one $420 purchase against a $270 usual
	// reported "1 purchase explains 280% of it", which reads as a broken
	// calculation because it is one: the culprits are being measured against the
	// OVERAGE, and a single receipt can easily exceed it.
	//
	// Capped, the claim is the true one — those purchases fully account for the
	// overspend — and Everything lets a caller say so in words rather than
	// printing a suspiciously round 100%.
	if a.ExplainedPct >= 100 {
		a.ExplainedPct = 100
		a.Everything = true
	}
	// Concentration is few purchases explaining MOST — not merely a cumulative
	// share, which any set of equal purchases reaches given enough of them. Six
	// identical dinners "explaining 67%" is not a culprit, it is arithmetic, and
	// reporting it as a cause would point at four ordinary meals.
	//
	// So the culprits must also be a minority of the period's purchases. Naming
	// half of what somebody bought is not pointing at a cause, it is handing back
	// the list they already have. A single dominant purchase always qualifies:
	// one item explaining most of an overspend is the clearest finding there is.
	minority := len(sorted) / 2
	if minority < 1 {
		minority = 1
	}
	a.Concentrated = a.ExplainedPct >= ExplainedPct && len(a.Culprits) <= minority

	if !a.Concentrated {
		// Nothing to point at. The culprit list is dropped rather than shown as a
		// weak suggestion: a list labelled "these might explain it" is read as
		// "these explain it", and the reader goes looking for a cause that is not
		// there.
		a.Culprits = nil
		return a
	}
	for _, p := range a.Culprits {
		if p.Larger {
			a.UnusuallyLarge++
		}
	}
	return a
}

// pct is a share, guarding the division.
func pct(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

// TotalMinor sums a set of purchases.
func TotalMinor(ps []Purchase) int64 {
	var n int64
	for _, p := range ps {
		n += p.Minor
	}
	return n
}
