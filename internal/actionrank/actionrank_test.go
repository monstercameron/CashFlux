// SPDX-License-Identifier: MIT

package actionrank

import "testing"

func act(id string, monthly, once int64) Action {
	return Action{ID: id, Name: id, MonthlyImpactMinor: monthly, OneTimeImpactMinor: once,
		Reversible: true, Confidence: 1, UrgencyDays: -1}
}

// A month's saving repeats and a one-off does not, so comparing them at face
// value would rank a single $200 rebate above $50 a month forever.
func TestARecurringSavingBeatsAOneOffOfTheSameSize(t *testing.T) {
	r := Rank([]Action{act("once", 0, 20_000), act("monthly", 20_000, 0)})
	if r[0].Action.ID != "monthly" {
		t.Errorf("first = %q, want the recurring one", r[0].Action.ID)
	}
	// And a large enough one-off still wins, or the rule has become a dogma.
	r = Rank([]Action{act("once", 0, 500_000), act("monthly", 2_000, 0)})
	if r[0].Action.ID != "once" {
		t.Errorf("first = %q, want the much larger one-off", r[0].Action.ID)
	}
}

// Added, a pointless action somebody is perfectly sure about would climb the
// list on certainty alone.
func TestConfidenceScalesTheBenefitAndIsNotOneItself(t *testing.T) {
	sure := act("sure", 0, 0)
	sure.Confidence = 1
	unsure := act("unsure", 5_000, 0)
	unsure.Confidence = 0.5
	r := Rank([]Action{sure, unsure})
	if r[0].Action.ID != "unsure" {
		t.Errorf("first = %q — certainty about nothing outranked a real saving", r[0].Action.ID)
	}

	// Between equal savings, the surer one leads.
	a, b := act("certain", 10_000, 0), act("doubtful", 10_000, 0)
	b.Confidence = 0.3
	r = Rank([]Action{b, a})
	if r[0].Action.ID != "certain" {
		t.Errorf("first = %q, want the one we are sure about", r[0].Action.ID)
	}
}

// Effort and reversibility are COSTS. An action that is easy and reversible is
// not thereby worth doing; it is just cheap to try.
func TestEffortAndIrreversibilityOnlySubtract(t *testing.T) {
	easy := act("easy", 0, 0)
	if s, _ := score(easy); s > 0 {
		t.Errorf("a zero-impact, zero-effort action scored %v — cheapness is not value", s)
	}
	hard := act("hard", 10_000, 0)
	hard.Effort = EffortHigh
	simple := act("simple", 10_000, 0)
	r := Rank([]Action{hard, simple})
	if r[0].Action.ID != "simple" {
		t.Errorf("first = %q, want the same saving for less work", r[0].Action.ID)
	}
	oneWay := act("oneway", 10_000, 0)
	oneWay.Reversible = false
	r = Rank([]Action{oneWay, simple})
	if r[0].Action.ID != "simple" {
		t.Errorf("first = %q, want the reversible one", r[0].Action.ID)
	}
}

// "No deadline" is not "a deadline far away", which is why it is not just a
// large number.
func TestUrgencyNeedsAnActualDeadline(t *testing.T) {
	none := act("none", 5_000, 0)
	none.UrgencyDays = 0 // zero days, but no deadline was ever set
	soon := act("soon", 5_000, 0)
	soon.HasDeadline, soon.UrgencyDays = true, 2
	r := Rank([]Action{none, soon})
	if r[0].Action.ID != "soon" {
		t.Errorf("first = %q, want the one with a real deadline", r[0].Action.ID)
	}
	if none.UrgencyDays == 0 {
		if _, b := score(none); b[CritUrgency] != 0 {
			t.Error("an action with no deadline scored on urgency")
		}
	}
	// A distant deadline is not urgency either.
	far := act("far", 5_000, 0)
	far.HasDeadline, far.UrgencyDays = true, UrgentWithinDays+10
	if _, b := score(far); b[CritUrgency] != 0 {
		t.Error("a deadline beyond the window counted as urgent")
	}
}

// The ticket's actual requirement: not the order, the sentence after it.
func TestWhyNamesTheOneThingThatDecidedIt(t *testing.T) {
	big := act("big", 40_000, 0)
	small := act("small", 1_000, 0)
	r := Rank([]Action{small, big})
	why := Why(r[0], r[1])
	if why.TooClose {
		t.Fatalf("a 40x difference was called a close call: %+v", why)
	}
	if why.Criterion != CritMonthly {
		t.Errorf("criterion = %q, want %q", why.Criterion, CritMonthly)
	}
	if why.MarginPct <= 0 {
		t.Errorf("margin = %v, want a positive gap", why.MarginPct)
	}
}

func TestWhyPointsAtEffortWhenEffortDecidedIt(t *testing.T) {
	easy := act("easy", 10_000, 0)
	hard := act("hard", 10_000, 0)
	hard.Effort = EffortHigh
	r := Rank([]Action{hard, easy})
	if why := Why(r[0], r[1]); why.Criterion != CritEffort {
		t.Errorf("criterion = %q, want %q (identical savings, different work)", why.Criterion, CritEffort)
	}
}

func TestWhyPointsAtUrgencyWhenTheDeadlineDecidedIt(t *testing.T) {
	soon := act("soon", 10_000, 0)
	soon.HasDeadline, soon.UrgencyDays = true, 1
	later := act("later", 10_000, 0)
	r := Rank([]Action{later, soon})
	if why := Why(r[0], r[1]); why.Criterion != CritUrgency {
		t.Errorf("criterion = %q, want %q", why.Criterion, CritUrgency)
	}
}

// Inventing a reason for a coin-flip is dressing up an artifact of the weights
// as analysis.
func TestATooCloseCallSaysSo(t *testing.T) {
	a := act("a", 10_000, 0)
	b := act("b", 10_050, 0)
	r := Rank([]Action{a, b})
	why := Why(r[0], r[1])
	if !why.TooClose {
		t.Errorf("a fractional difference was given a reason: %+v", why)
	}
	if why.Criterion != "" {
		t.Errorf("a close call named a criterion: %q", why.Criterion)
	}
}

func TestWhyRefusesTheWrongWayRound(t *testing.T) {
	r := Rank([]Action{act("small", 1_000, 0), act("big", 40_000, 0)})
	if why := Why(r[1], r[0]); !why.TooClose || why.Criterion != "" {
		t.Errorf("explained why the LOSER outranked the winner: %+v", why)
	}
}

// A ranking that reshuffles between renders cannot be re-read, and a reader who
// cannot re-read it will not trust it.
func TestTheOrderIsStable(t *testing.T) {
	in := []Action{act("z", 5_000, 0), act("a", 5_000, 0), act("m", 5_000, 0)}
	first := Rank(in)
	if first[0].Action.ID != "a" {
		t.Errorf("tie broke to %q, want the stable choice", first[0].Action.ID)
	}
	for i := range 5 {
		got := Rank(in)
		for j := range got {
			if got[j].Action.ID != first[j].Action.ID {
				t.Fatalf("run %d reordered at %d: %q vs %q", i, j, got[j].Action.ID, first[j].Action.ID)
			}
		}
	}
}

func TestEmptyInput(t *testing.T) {
	if got := Rank(nil); len(got) != 0 {
		t.Errorf("Rank(nil) = %+v", got)
	}
}

func TestBreakdownIsPublishedForEveryCriterion(t *testing.T) {
	// Every criterion the ranking uses must be readable, or "why" cannot be
	// checked by the person it is shown to.
	_, b := score(act("a", 1_000, 500))
	for _, c := range []Criterion{CritMonthly, CritOneTime, CritEffort, CritReversible, CritUrgency, CritConfidence} {
		if _, ok := b[c]; !ok {
			t.Errorf("criterion %q is used but not published", c)
		}
	}
}
