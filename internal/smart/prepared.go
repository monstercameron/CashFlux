// SPDX-License-Identifier: MIT

package smart

// ─── E5: the insight schema and the prepared-action primitive ────────────────
//
// The E-series contract asks every finding to carry evidence, confidence, a
// dollar impact, its assumptions, and two or three QUANTIFIED choices — applied
// as a changeset, undoable — rather than generic advice. An Insight already
// carries confidence and an optional single Action. This file adds the rest, and
// it is a separate file because the shape is a contract other engines are built
// against: it should be readable in one sitting.
//
// The distinction that drives everything here is between an ACTION and a
// DECISION. An action is "add a to-do" — one obvious follow-up, and Insight.Action
// already covers it. A decision is "you are $180 over on Dining: move $180 from
// Entertainment, raise the Dining limit to $520, or leave it and absorb it" —
// two or three options, each with its consequence attached. Presenting a decision
// as a single button hides the alternatives; presenting it as prose hands the
// arithmetic back to the reader, which is the work the engine was supposed to do.

// EvidenceKind names what a piece of evidence points at.
type EvidenceKind string

const (
	// EvidenceTransaction points at a ledger row.
	EvidenceTransaction EvidenceKind = "transaction"
	// EvidenceAccount points at an account.
	EvidenceAccount EvidenceKind = "account"
	// EvidenceBudget points at a budget.
	EvidenceBudget EvidenceKind = "budget"
	// EvidenceGoal points at a goal.
	EvidenceGoal EvidenceKind = "goal"
	// EvidenceRecurring points at a recurring flow or bill.
	EvidenceRecurring EvidenceKind = "recurring"
	// EvidenceTask points at a to-do.
	EvidenceTask EvidenceKind = "task"
	// EvidenceCategory points at a category.
	EvidenceCategory EvidenceKind = "category"
)

// Evidence is a checkable pointer to the data a finding rests on.
//
// It is an ID and a kind, not a rendered sentence, because the point of evidence
// is that the reader can GO THERE. A finding that says "based on 14 transactions"
// without being able to show them is asking for trust, which is the thing
// evidence exists to replace.
type Evidence struct {
	Kind EvidenceKind
	// ID is the entity's id, for navigation and filtering.
	ID string
	// Label is how to name it in a list ("Kroger, Aug 14, $84.20").
	Label string
	// LabelKey / LabelArgs are Label in re-renderable form (C362).
	LabelKey  string
	LabelArgs []any
}

// Choice is one prepared, quantified option in a decision.
type Choice struct {
	// Label is the option as a person would phrase it.
	Label string
	// LabelKey / LabelArgs are Label in re-renderable form (C362).
	LabelKey  string
	LabelArgs []any
	// ImpactMinor is what taking this option does, in minor units, signed from
	// the household's point of view: positive is money kept or gained.
	ImpactMinor int64
	// ImpactCurrency is ImpactMinor's currency.
	ImpactCurrency string
	// HasImpact distinguishes "this option costs nothing" from "nobody worked out
	// what it costs". A zero that means the second is the failure this flag
	// exists to prevent — it reads as a free option.
	HasImpact bool
	// Action is what applying this choice does. A choice with no action is a
	// description, not a decision.
	Action Action
	// Recommended marks the option the engine would take. At most one may be
	// set; see Prepared.Validate.
	Recommended bool
}

// Prepared is the E5 payload: everything a finding needs to be checkable and
// actionable without leaving the page it appears on.
type Prepared struct {
	// Evidence is what the finding rests on.
	Evidence []Evidence
	// Assumptions are the things taken as given that the reader might not accept
	// ("assumes your current pay continues"). Stated, because an unstated
	// assumption is what makes a correct number feel wrong.
	Assumptions []string
	// AssumptionKeys mirrors Assumptions as catalog keys, same order (C362).
	AssumptionKeys []string
	// ImpactMinor / ImpactCurrency / HasImpact are the finding's own headline
	// consequence, distinct from any individual choice's.
	ImpactMinor    int64
	ImpactCurrency string
	HasImpact      bool
	// Choices are the prepared options. Empty means the finding is informational
	// or carries a single Insight.Action instead.
	Choices []Choice
}

// MinChoices and MaxChoices bound a prepared decision.
//
// Two is the floor because one option is not a decision — it is an action, and
// Insight.Action already expresses that without the ceremony. Three is the
// ceiling because the promise of the E-series is a decision the reader can make
// in seconds; a fourth option turns a prepared decision back into a menu, which
// is the work being compressed away.
const (
	MinChoices = 2
	MaxChoices = 3
)

// Empty reports whether the payload carries nothing.
func (p Prepared) Empty() bool {
	return len(p.Evidence) == 0 && len(p.Choices) == 0 &&
		len(p.Assumptions) == 0 && !p.HasImpact
}

// Decision reports whether the payload offers a choice to make.
func (p Prepared) Decision() bool { return len(p.Choices) >= MinChoices }

// RecommendedChoice returns the option the engine would take.
func (p Prepared) RecommendedChoice() (Choice, bool) {
	for _, c := range p.Choices {
		if c.Recommended {
			return c, true
		}
	}
	return Choice{}, false
}

// EvidenceIDs returns the ids of one kind of evidence, for building a filter or
// a drill-through.
func (p Prepared) EvidenceIDs(k EvidenceKind) []string {
	var out []string
	for _, e := range p.Evidence {
		if e.Kind == k && e.ID != "" {
			out = append(out, e.ID)
		}
	}
	return out
}

// Validate reports every way a payload breaks the E5 contract, so a detector
// fails in a test rather than shipping a finding a reader cannot act on.
//
// The rules are deliberately strict about the things that LOOK fine on screen
// and mislead: a second recommended option (which reads as no recommendation), a
// choice with an unquantified impact (which reads as free), and evidence missing
// from a finding that admits it might be wrong (which asks for trust exactly
// where trust has not been earned).
func (p Prepared) Validate(conf Confidence) []string {
	var errs []string
	if n := len(p.Choices); n > 0 {
		if n < MinChoices {
			errs = append(errs, "a single choice is an action, not a decision — use Insight.Action")
		}
		if n > MaxChoices {
			errs = append(errs, "more than three choices is a menu, not a prepared decision")
		}
	}
	var rec int
	for i, c := range p.Choices {
		if c.Recommended {
			rec++
		}
		if c.Label == "" && c.LabelKey == "" {
			errs = append(errs, "choice "+itoa(int64(i))+" has no label")
		}
		if !c.HasImpact {
			errs = append(errs, "choice "+itoa(int64(i))+" has no quantified impact — an unpriced option reads as a free one")
		}
		if c.Action.Kind == "" {
			errs = append(errs, "choice "+itoa(int64(i))+" has no action — a description is not a decision")
		}
	}
	if rec > 1 {
		errs = append(errs, "more than one recommended choice — two defaults is no default")
	}
	// An inference is a claim that could be wrong, so it has to be checkable.
	// A restatement of recorded data does not need evidence: it IS the data.
	if conf != ConfidenceCertain && len(p.Choices) > 0 && len(p.Evidence) == 0 {
		errs = append(errs, "an uncertain finding with prepared choices carries no evidence")
	}
	if len(p.AssumptionKeys) > 0 && len(p.AssumptionKeys) != len(p.Assumptions) {
		errs = append(errs, "assumption keys and text disagree in length")
	}
	return errs
}

// WithEvidence appends a piece of evidence and returns the payload, so a
// detector can build one in an expression.
func (p Prepared) WithEvidence(kind EvidenceKind, id, label string) Prepared {
	p.Evidence = append(p.Evidence, Evidence{Kind: kind, ID: id, Label: label})
	return p
}

// WithImpact sets the finding's headline consequence.
func (p Prepared) WithImpact(minor int64, currency string) Prepared {
	p.ImpactMinor, p.ImpactCurrency, p.HasImpact = minor, currency, true
	return p
}

// WithAssumption records something the finding takes as given.
func (p Prepared) WithAssumption(key, text string) Prepared {
	p.Assumptions = append(p.Assumptions, text)
	p.AssumptionKeys = append(p.AssumptionKeys, key)
	return p
}

// WithChoice appends a prepared option.
func (p Prepared) WithChoice(c Choice) Prepared {
	p.Choices = append(p.Choices, c)
	return p
}

// WithPrepared attaches an E5 payload to an insight, so a detector can build one
// in an expression alongside WithTitle / WithDetail.
func (i Insight) WithPrepared(p Prepared) Insight {
	i.Prepared = p
	return i
}

// ValidatePrepared checks an insight's payload against the E5 contract using the
// insight's own resolved confidence.
//
// It takes the resolved confidence rather than the raw field because an insight
// that did not state one inherits its feature's default, and validating against
// the zero value would treat every silent detector as certain — which is exactly
// the case where a missing evidence trail matters most.
func (i Insight) ValidatePrepared() []string {
	return i.Prepared.Validate(i.ResolvedConfidence())
}
