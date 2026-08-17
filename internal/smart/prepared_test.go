// SPDX-License-Identifier: MIT

package smart

import (
	"strings"
	"testing"
)

func priced(label string, impact int64, rec bool) Choice {
	return Choice{
		Label: label, ImpactMinor: impact, ImpactCurrency: "USD", HasImpact: true,
		Action: Action{Kind: ActionNavigate, Route: "/budgets"}, Recommended: rec,
	}
}

func TestPreparedShapeHelpers(t *testing.T) {
	if !(Prepared{}).Empty() {
		t.Error("the zero payload claimed to carry something")
	}
	p := Prepared{}.
		WithEvidence(EvidenceTransaction, "t-1", "Kroger, Aug 14").
		WithEvidence(EvidenceTransaction, "t-2", "Kroger, Aug 21").
		WithEvidence(EvidenceBudget, "b-1", "Dining").
		WithImpact(-18000, "USD").
		WithAssumption("assume.payContinues", "assumes your current pay continues")

	if p.Empty() {
		t.Error("a populated payload reported empty")
	}
	if p.Decision() {
		t.Error("a payload with no choices reported a decision")
	}
	if got := p.EvidenceIDs(EvidenceTransaction); len(got) != 2 || got[0] != "t-1" {
		t.Errorf("EvidenceIDs = %v", got)
	}
	if got := p.EvidenceIDs(EvidenceGoal); len(got) != 0 {
		t.Errorf("EvidenceIDs for an absent kind = %v", got)
	}
	if !p.HasImpact || p.ImpactMinor != -18000 {
		t.Errorf("impact = %d,%v", p.ImpactMinor, p.HasImpact)
	}
	if len(p.Assumptions) != 1 || len(p.AssumptionKeys) != 1 {
		t.Errorf("assumptions = %v / %v", p.Assumptions, p.AssumptionKeys)
	}
}

func TestDecisionNeedsAtLeastTwoChoices(t *testing.T) {
	one := Prepared{}.WithChoice(priced("Move $180", 0, false))
	if one.Decision() {
		t.Error("one choice reported as a decision")
	}
	two := one.WithChoice(priced("Raise the limit", -18000, false))
	if !two.Decision() {
		t.Error("two choices did not report as a decision")
	}
}

// One option is an action, not a decision — Insight.Action already says that
// without the ceremony.
func TestValidateRejectsASingleChoice(t *testing.T) {
	p := Prepared{}.WithChoice(priced("Do the thing", 100, false))
	errs := p.Validate(ConfidenceCertain)
	if !hasErr(errs, "single choice") {
		t.Errorf("errs = %v, want a single-choice complaint", errs)
	}
}

// A fourth option turns a prepared decision back into the menu it was meant to
// compress away.
func TestValidateRejectsMoreThanThreeChoices(t *testing.T) {
	p := Prepared{}.
		WithChoice(priced("a", 1, false)).WithChoice(priced("b", 2, false)).
		WithChoice(priced("c", 3, false)).WithChoice(priced("d", 4, false)).
		WithEvidence(EvidenceBudget, "b", "Dining")
	if errs := p.Validate(ConfidenceCertain); !hasErr(errs, "menu") {
		t.Errorf("errs = %v, want a too-many-choices complaint", errs)
	}
}

// Two defaults is no default.
func TestValidateRejectsTwoRecommendations(t *testing.T) {
	p := Prepared{}.WithChoice(priced("a", 1, true)).WithChoice(priced("b", 2, true))
	if errs := p.Validate(ConfidenceCertain); !hasErr(errs, "recommended") {
		t.Errorf("errs = %v, want a two-defaults complaint", errs)
	}
	// Exactly one is fine and resolvable.
	ok := Prepared{}.WithChoice(priced("a", 1, true)).WithChoice(priced("b", 2, false))
	if errs := ok.Validate(ConfidenceCertain); len(errs) != 0 {
		t.Errorf("errs = %v, want none", errs)
	}
	if c, found := ok.RecommendedChoice(); !found || c.Label != "a" {
		t.Errorf("RecommendedChoice = %+v,%v", c, found)
	}
}

// An unpriced option reads as a free one, which is the most misleading thing a
// prepared decision can do.
func TestValidateRejectsAnUnquantifiedChoice(t *testing.T) {
	p := Prepared{}.
		WithChoice(Choice{Label: "a", Action: Action{Kind: ActionNavigate, Route: "/x"}}).
		WithChoice(priced("b", 2, false))
	if errs := p.Validate(ConfidenceCertain); !hasErr(errs, "quantified impact") {
		t.Errorf("errs = %v, want an unpriced-choice complaint", errs)
	}
	// A zero impact is legitimate — it just has to be DECLARED as zero rather
	// than left as the zero value nobody set.
	declared := Prepared{}.
		WithChoice(Choice{Label: "leave it", HasImpact: true, ImpactCurrency: "USD",
			Action: Action{Kind: ActionNavigate, Route: "/x"}}).
		WithChoice(priced("b", 2, false))
	if errs := declared.Validate(ConfidenceCertain); len(errs) != 0 {
		t.Errorf("a declared zero impact was rejected: %v", errs)
	}
}

// A description is not a decision.
func TestValidateRejectsAChoiceWithNoAction(t *testing.T) {
	p := Prepared{}.
		WithChoice(Choice{Label: "a", HasImpact: true}).
		WithChoice(priced("b", 2, false))
	if errs := p.Validate(ConfidenceCertain); !hasErr(errs, "no action") {
		t.Errorf("errs = %v, want a no-action complaint", errs)
	}
}

func TestValidateRejectsAnUnlabelledChoice(t *testing.T) {
	p := Prepared{}.
		WithChoice(Choice{HasImpact: true, Action: Action{Kind: ActionNavigate, Route: "/x"}}).
		WithChoice(priced("b", 2, false))
	if errs := p.Validate(ConfidenceCertain); !hasErr(errs, "no label") {
		t.Errorf("errs = %v, want an unlabelled-choice complaint", errs)
	}
	// A catalog key alone is a label.
	keyed := Prepared{}.
		WithChoice(Choice{LabelKey: "x.y", HasImpact: true, Action: Action{Kind: ActionNavigate, Route: "/x"}}).
		WithChoice(priced("b", 2, false))
	if errs := keyed.Validate(ConfidenceCertain); len(errs) != 0 {
		t.Errorf("a key-only label was rejected: %v", errs)
	}
}

// An inference is a claim that could be wrong, so it must be checkable. A
// restatement of recorded data does not need evidence — it IS the data.
func TestValidateDemandsEvidenceOnlyForUncertainFindings(t *testing.T) {
	noEvidence := Prepared{}.WithChoice(priced("a", 1, false)).WithChoice(priced("b", 2, false))

	if errs := noEvidence.Validate(ConfidenceCertain); len(errs) != 0 {
		t.Errorf("a certain finding was made to justify itself: %v", errs)
	}
	for _, c := range []Confidence{ConfidenceLikely, ConfidencePossible} {
		if errs := noEvidence.Validate(c); !hasErr(errs, "no evidence") {
			t.Errorf("confidence %v with no evidence produced %v", c, errs)
		}
	}
	withEvidence := noEvidence.WithEvidence(EvidenceTransaction, "t-1", "Kroger")
	if errs := withEvidence.Validate(ConfidencePossible); len(errs) != 0 {
		t.Errorf("an evidenced uncertain finding was rejected: %v", errs)
	}
	// An INFORMATIONAL uncertain finding (no choices) is not asked for evidence:
	// it proposes nothing, so there is nothing to check before acting.
	if errs := (Prepared{}).Validate(ConfidencePossible); len(errs) != 0 {
		t.Errorf("an informational finding was made to justify itself: %v", errs)
	}
}

func TestValidateCatchesMismatchedAssumptionKeys(t *testing.T) {
	p := Prepared{Assumptions: []string{"a", "b"}, AssumptionKeys: []string{"k"}}
	if errs := p.Validate(ConfidenceCertain); !hasErr(errs, "disagree in length") {
		t.Errorf("errs = %v", errs)
	}
}

func hasErr(errs []string, want string) bool {
	for _, e := range errs {
		if strings.Contains(e, want) {
			return true
		}
	}
	return false
}

// The resolved confidence is what must be validated against: an insight that
// did not state one inherits its feature's default, and validating the raw zero
// value would treat every silent detector as certain — exactly the case where a
// missing evidence trail matters most.
func TestValidatePreparedUsesTheResolvedConfidence(t *testing.T) {
	noEvidence := Prepared{}.
		WithChoice(priced("a", 1, false)).
		WithChoice(priced("b", 2, false))

	// Explicitly certain: no evidence demanded.
	certain := Insight{}.WithPrepared(noEvidence)
	certain.Confidence, certain.confidenceSet = ConfidenceCertain, true
	if errs := certain.ValidatePrepared(); len(errs) != 0 {
		t.Errorf("a certain insight was made to justify itself: %v", errs)
	}

	// Explicitly uncertain: evidence demanded.
	unsure := Insight{}.WithPrepared(noEvidence)
	unsure.Confidence, unsure.confidenceSet = ConfidencePossible, true
	if errs := unsure.ValidatePrepared(); !hasErr(errs, "no evidence") {
		t.Errorf("errs = %v, want an evidence complaint", errs)
	}
}

func TestWithPreparedIsAValueBuilder(t *testing.T) {
	base := Insight{Title: "x"}
	got := base.WithPrepared(Prepared{}.WithImpact(500, "USD"))
	if !got.Prepared.HasImpact || got.Prepared.ImpactMinor != 500 {
		t.Errorf("Prepared = %+v", got.Prepared)
	}
	// The receiver is untouched — detectors build insights in expressions and a
	// mutating builder would surprise them.
	if base.Prepared.HasImpact {
		t.Error("WithPrepared mutated its receiver")
	}
}
