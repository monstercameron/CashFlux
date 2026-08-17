// SPDX-License-Identifier: MIT

package trust

import "testing"

func present(name string) Input { return Input{Name: name, Required: true, AgeDays: 5} }

func TestEverythingPresentAndCurrentIsSolid(t *testing.T) {
	a := Assess([]Input{present("your balance"), present("the card's APR")})
	if a.Level != LevelSolid {
		t.Errorf("level = %q, want %q", a.Level, LevelSolid)
	}
	if !a.Trustworthy() || len(a.Reasons()) != 0 {
		t.Errorf("a solid figure should have nothing to explain: %+v", a)
	}
}

// "Computed from a balance you last updated in March" and "computed with no
// interest rate at all" are different kinds of wrong. Collapsing them into one
// warning teaches people to ignore both.
func TestAMissingRequiredInputIsUnreliableNotMerelyQualified(t *testing.T) {
	a := Assess([]Input{
		present("your balance"),
		{Name: "the card's APR", Required: true, Missing: true},
	})
	if a.Level != LevelUnreliable {
		t.Errorf("level = %q, want %q", a.Level, LevelUnreliable)
	}
	if len(a.MissingRequired) != 1 || a.MissingRequired[0] != "the card's APR" {
		t.Errorf("missing required = %v, want the APR named", a.MissingRequired)
	}
}

func TestAStaleInputQualifiesRatherThanInvalidates(t *testing.T) {
	a := Assess([]Input{present("the APR"), {Name: "your balance", Required: true, AgeDays: StaleDays + 1}})
	if a.Level != LevelQualified {
		t.Errorf("level = %q, want %q", a.Level, LevelQualified)
	}
	if len(a.Stale) != 1 || a.Stale[0] != "your balance" {
		t.Errorf("stale = %v, want the balance named", a.Stale)
	}
}

func TestTheStalenessEdge(t *testing.T) {
	fresh := Assess([]Input{{Name: "x", Required: true, AgeDays: StaleDays}})
	if fresh.Level != LevelSolid {
		t.Errorf("exactly at the limit = %q, want still solid", fresh.Level)
	}
	old := Assess([]Input{{Name: "x", Required: true, AgeDays: StaleDays + 1}})
	if old.Level != LevelQualified {
		t.Errorf("a day past = %q, want qualified", old.Level)
	}
}

// An input with no known age is exactly the kind of thing that quietly rots, so
// it must not be treated as fresh by default.
func TestAnUnknownAgeIsNotAssumedFresh(t *testing.T) {
	a := Assess([]Input{{Name: "x", Required: true}})
	if a.Level == LevelUnreliable {
		t.Error("an unknown age is not the same as a missing input")
	}
	// Age zero is not stale by the day count, but it is also not evidence of
	// freshness — the caller is expected to supply a real age. This test pins the
	// current behaviour so a future change to it is deliberate.
	if a.Level != LevelSolid {
		t.Errorf("level = %q; if this changes, it should be a considered decision", a.Level)
	}
}

func TestAnAssumedValueQualifiesTheFigure(t *testing.T) {
	a := Assess([]Input{present("your balance"), {Name: "an assumed 7% return", Required: true, AgeDays: 1, Assumed: true}})
	if a.Level != LevelQualified {
		t.Errorf("level = %q, want %q", a.Level, LevelQualified)
	}
	if len(a.Assumed) != 1 {
		t.Errorf("assumed = %v, want the assumption named", a.Assumed)
	}
}

func TestAMissingOptionalInputOnlyQualifies(t *testing.T) {
	a := Assess([]Input{present("your balance"), {Name: "a nickname", Missing: true}})
	if a.Level != LevelQualified {
		t.Errorf("level = %q, want %q — an optional gap does not invalidate", a.Level, LevelQualified)
	}
	if len(a.MissingOptional) != 1 {
		t.Errorf("missing optional = %v, want it named", a.MissingOptional)
	}
}

// A figure whose dependencies nobody declared has not been shown to be
// trustworthy — it has merely not been examined.
func TestNoDeclaredInputsIsUnreliableNotSolid(t *testing.T) {
	if a := Assess(nil); a.Level != LevelUnreliable {
		t.Errorf("level = %q, want %q for an unexamined figure", a.Level, LevelUnreliable)
	}
}

// The order in which reasons change what the reader should DO.
func TestReasonsAreOrderedByWhatToDoAboutThem(t *testing.T) {
	a := Assess([]Input{
		{Name: "an assumption", Required: true, AgeDays: 1, Assumed: true},
		{Name: "a stale balance", Required: true, AgeDays: StaleDays + 10},
		{Name: "a missing APR", Required: true, Missing: true},
		{Name: "an optional note", Missing: true},
	})
	got := a.Reasons()
	want := []string{"a missing APR", "a stale balance", "an assumption", "an optional note"}
	if len(got) != len(want) {
		t.Fatalf("reasons = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("reason %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReasonsAreStable(t *testing.T) {
	// A trust panel that reshuffles between renders is one nobody can re-read to
	// check they read it right.
	in := []Input{
		{Name: "zebra", Required: true, Missing: true},
		{Name: "apple", Required: true, Missing: true},
	}
	for i := range 5 {
		a := Assess(in)
		if a.MissingRequired[0] != "apple" {
			t.Fatalf("run %d ordered %v", i, a.MissingRequired)
		}
	}
}

func TestBlankNamesAreIgnored(t *testing.T) {
	// A reason with no name is not a reason; it is a bullet point saying nothing.
	a := Assess([]Input{present("real"), {Name: "  ", Required: true, Missing: true}})
	if a.Level != LevelSolid {
		t.Errorf("level = %q — an unnamed input cannot be reported, so it must not silently invalidate", a.Level)
	}
}

// Averaging confidence would let two solid inputs hide one that is missing
// entirely — precisely the failure this package exists to prevent.
func TestWorstTakesTheLeastTrustworthy(t *testing.T) {
	solid := Assess([]Input{present("a")})
	qualified := Assess([]Input{{Name: "b", Required: true, AgeDays: StaleDays + 1}})
	unreliable := Assess([]Input{{Name: "c", Required: true, Missing: true}})
	if got := Worst(solid, qualified, unreliable); got.Level != LevelUnreliable {
		t.Errorf("worst = %q, want %q", got.Level, LevelUnreliable)
	}
	if got := Worst(solid, qualified); got.Level != LevelQualified {
		t.Errorf("worst = %q, want %q", got.Level, LevelQualified)
	}
	if got := Worst(solid, solid); got.Level != LevelSolid {
		t.Errorf("worst = %q, want %q", got.Level, LevelSolid)
	}
}

func TestWorstOfNothingIsUnreliable(t *testing.T) {
	if got := Worst(); got.Level != LevelUnreliable {
		t.Errorf("worst of nothing = %q, want %q", got.Level, LevelUnreliable)
	}
}
