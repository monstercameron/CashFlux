// SPDX-License-Identifier: MIT

package smart

// Confidence says how sure a finding is about itself (C391).
//
// SMART findings are not all the same kind of claim. "This budget is £40 over" is
// arithmetic over rows the household typed in — it is not a guess and treating it
// as one wastes the reader's attention. "These two charges look like duplicates" is
// an inference from a pattern, and it is sometimes wrong. Presented identically,
// the reliable findings get discounted at the same rate as the speculative ones,
// which is the worst of both: the guesses are trusted too much and the facts too
// little.
//
// So each finding carries the kind of claim it is making, and the surface can say
// so. The tiers are deliberately three, and named for how a person would hedge in
// conversation.
type Confidence int

const (
	// ConfidenceCertain is a restatement of recorded data — arithmetic over the
	// ledger. If it is wrong, the data is wrong, not the finding.
	ConfidenceCertain Confidence = iota
	// ConfidenceLikely is a strong pattern with a known shape ("this bill has
	// posted on the 3rd for nine months and hasn't this month").
	ConfidenceLikely
	// ConfidencePossible is a heuristic that is sometimes wrong ("these two look
	// like duplicates"). Worth surfacing, never worth acting on unchecked.
	ConfidencePossible
)

// Label renders the confidence as the hedge a person would actually say.
func (c Confidence) Label() string {
	switch c {
	case ConfidenceLikely:
		return "Likely"
	case ConfidencePossible:
		return "Worth a look"
	default:
		return "From your data"
	}
}

// Hedged reports whether the finding is an inference rather than a fact. Surfaces
// use this to decide whether the tier is worth showing at all: labelling the
// arithmetic "From your data" on every row is noise, while leaving a guess
// unmarked is the failure this exists to prevent.
func (c Confidence) Hedged() bool { return c != ConfidenceCertain }

// inferredFeatures are the detectors whose findings are inferences. Everything
// absent from this table is arithmetic over recorded data and reports as certain.
//
// The table lists the guessers rather than the facts on purpose: it is the shorter
// list, it is the one that matters, and a new deterministic detector added without
// touching this file gets the right answer by default. A new HEURISTIC detector
// added without touching this file gets the wrong answer — which is why the
// package test walks the catalog and names any detector whose summary talks about
// looking, seeming, or predicting but which is missing here.
var inferredFeatures = map[string]Confidence{
	// Duplicate detection compares payee/amount/date proximity. Two genuine
	// same-day charges at the same merchant are common and read identically.
	"SMART-T2": ConfidencePossible,
	// A spending "spike" is a threshold over a rolling average — a real but
	// intended splurge trips it exactly as an anomaly does.
	"SMART-T6": ConfidencePossible,
	// A missing transaction is inferred from a schedule the household never
	// declared; a bill genuinely skipped this month looks the same as one missed.
	"SMART-T7": ConfidenceLikely,
	// A balance anomaly compares this month's movement to previous months.
	"SMART-A1": ConfidenceLikely,
	// A new subscription is guessed from a merchant's first two charges.
	"SMART-T20": ConfidencePossible,
	// Forecasts are arithmetic applied to a future that has not happened yet. The
	// method is sound and the answer is still a projection, so they hedge — a
	// runway figure quoted with the certainty of a bank balance invites decisions
	// the number cannot support.
	"SMART-P3":  ConfidenceLikely,
	"SMART-P5":  ConfidenceLikely,
	"SMART-BL1": ConfidenceLikely,
}

// ConfidenceFor returns the confidence a feature's findings carry by default.
func ConfidenceFor(feature string) Confidence {
	if c, ok := inferredFeatures[feature]; ok {
		return c
	}
	return ConfidenceCertain
}

// WithConfidence returns a copy of the insight carrying an explicit confidence,
// for a detector that knows more about a particular finding than its feature code
// implies (a duplicate match on an identical reference number, say).
func (i Insight) WithConfidence(c Confidence) Insight {
	i.Confidence = c
	i.confidenceSet = true
	return i
}

// ResolvedConfidence returns the insight's own confidence when a detector set one,
// and otherwise the default for its feature. Callers use this rather than reading
// the field, so a zero value means "not stated" instead of silently meaning
// "certain" — the one wrong answer to default to.
func (i Insight) ResolvedConfidence() Confidence {
	if i.confidenceSet {
		return i.Confidence
	}
	return ConfidenceFor(i.Feature)
}
