// SPDX-License-Identifier: MIT

package flagverdict

import (
	"testing"
	"time"
)

var day0 = time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

// The whole point of the package: five buttons that all dismiss would be worse
// than one Dismiss button, because it would look like the app understood.
func TestTwoVerdictsMustNotHideTheFlag(t *testing.T) {
	for _, v := range []Verdict{WrongCategory, Investigate} {
		if Effect(v).Hides {
			t.Errorf("%q hid the flag", v)
		}
	}
	for _, v := range []Verdict{OneTime, Expected, NewNormal} {
		if !Effect(v).Hides {
			t.Errorf("%q did not hide the flag", v)
		}
	}
}

// The money is real and filed in the wrong place — hiding it leaves the
// miscategorised spending exactly where it was.
func TestWrongCategoryLeavesWorkBehind(t *testing.T) {
	e := Effect(WrongCategory)
	if e.Follow != FollowRecategorize {
		t.Errorf("follow = %q, want %q", e.Follow, FollowRecategorize)
	}
	if e.Scope != ScopeNone {
		t.Errorf("scope = %q, want nothing suppressed", e.Scope)
	}
	if Effect(Investigate).Follow != FollowTrack {
		t.Error("investigate resolved the flag instead of keeping it open")
	}
}

func TestAnUnknownVerdictSilencesNothing(t *testing.T) {
	// The app does not hide a flag on the strength of a judgement it cannot read.
	if Effect(Verdict("shrug")).Hides {
		t.Error("an unrecognised verdict hid a flag")
	}
	if Verdict("shrug").Valid() {
		t.Error("an unrecognised verdict reported itself valid")
	}
}

// A one-off is a statement about ONE month. Next month's flag is new
// information, and the app that hides it has been told something it did not
// hear.
func TestAOneOffOnlySilencesItsOwnMonth(t *testing.T) {
	m := Memory{}.Record(Record{Key: "dining|2026-03", Subject: "Dining", Verdict: OneTime, At: day0})
	if !m.Suppressed("dining|2026-03", "Dining", day0) {
		t.Error("the judged month was still shown")
	}
	if m.Suppressed("dining|2026-04", "Dining", day0.AddDate(0, 1, 0)) {
		t.Error("next month's flag was hidden by last month's one-off")
	}
}

func TestExpectedSilencesTheRecurringFlagButNotForever(t *testing.T) {
	m := Memory{}.Record(Record{Key: "ins|2026-03", Subject: "Insurance", Verdict: Expected, At: day0})
	// A different month, same subject: an annual charge is expected again.
	if !m.Suppressed("ins|2027-02", "Insurance", day0.AddDate(1, 0, 0)) {
		t.Error("an expected annual charge came back within the year")
	}
	// Past the expiry the question is asked again — a suppression with no end is
	// how a real change goes unnoticed.
	if m.Suppressed("ins|2027-06", "Insurance", day0.AddDate(0, 0, ExpectedDays+1)) {
		t.Error("an expected verdict silenced the flag past its expiry")
	}
}

// Ninety days is the detector's own trailing window: after it, a baseline that
// absorbed the new level stops flagging by itself, and one that is still
// flagging is telling you something new.
func TestNewNormalExpiresWithTheBaseline(t *testing.T) {
	m := Memory{}.Record(Record{Key: "gro|2026-03", Subject: "Groceries", Verdict: NewNormal, At: day0})
	if !m.Suppressed("gro|2026-04", "Groceries", day0.AddDate(0, 0, 30)) {
		t.Error("a new normal was re-flagged while the baseline was still catching up")
	}
	if m.Suppressed("gro|2026-07", "Groceries", day0.AddDate(0, 0, NewNormalDays+1)) {
		t.Error("a new normal stayed suppressed after the baseline had caught up")
	}
	if NewNormalDays >= ExpectedDays {
		t.Error("a new normal outlasts an expected charge, which inverts the two claims")
	}
}

func TestANonHidingVerdictNeverSuppresses(t *testing.T) {
	for _, v := range []Verdict{WrongCategory, Investigate} {
		m := Memory{}.Record(Record{Key: "k|2026-03", Subject: "Travel", Verdict: v, At: day0})
		if m.Suppressed("k|2026-03", "Travel", day0) {
			t.Errorf("%q suppressed the flag it was recorded against", v)
		}
	}
}

// One wrong tap must not permanently blind the app to a category.
func TestEverySuppressionIsReversible(t *testing.T) {
	m := Memory{}.Record(Record{Key: "k|2026-03", Subject: "Travel", Verdict: Expected, At: day0})
	if !m.Suppressed("k|2026-04", "Travel", day0) {
		t.Fatal("setup: expected the flag suppressed")
	}
	m = m.Forget("k|2026-03")
	if m.Suppressed("k|2026-04", "Travel", day0) {
		t.Error("forgetting a judgement left the flag suppressed")
	}
}

func TestChangingYourMindReplacesTheVerdict(t *testing.T) {
	m := Memory{}.Record(Record{Key: "k", Subject: "Travel", Verdict: Expected, At: day0})
	m = m.Record(Record{Key: "k", Subject: "Travel", Verdict: Investigate, At: day0})
	if len(m.Records) != 1 {
		t.Fatalf("records = %d, want the earlier verdict replaced", len(m.Records))
	}
	if r, _ := m.For("k"); r.Verdict != Investigate {
		t.Errorf("verdict = %q, want the later one", r.Verdict)
	}
	if m.Suppressed("k", "Travel", day0) {
		t.Error("the replaced verdict was still suppressing")
	}
}

// The two non-hiding verdicts only earn their place if the work they leave
// behind is visible somewhere; otherwise they are a Dismiss button with a label.
func TestOpenListsTheWorkLeftBehind(t *testing.T) {
	m := Memory{}.
		Record(Record{Key: "a", Verdict: OneTime, At: day0}).
		Record(Record{Key: "b", Verdict: WrongCategory, At: day0}).
		Record(Record{Key: "c", Verdict: Investigate, At: day0}).
		Record(Record{Key: "d", Verdict: Expected, At: day0})
	open := m.Open()
	if len(open) != 2 {
		t.Fatalf("open = %d, want the two that resolved nothing: %+v", len(open), open)
	}
}

func TestRoundTrip(t *testing.T) {
	m := Memory{}.Record(Record{Key: "k|2026-03", Subject: "Dining", Verdict: Expected, At: day0, Note: "annual"})
	got := Load(m.Marshal())
	if len(got.Records) != 1 {
		t.Fatalf("records = %d after round-trip", len(got.Records))
	}
	r := got.Records[0]
	if r.Key != "k|2026-03" || r.Verdict != Expected || r.Note != "annual" || !r.At.Equal(day0) {
		t.Errorf("round-trip lost data: %+v", r)
	}
}

// A memory that cannot be read means nothing was decided, which shows every
// flag — the failure that costs the least.
func TestUnreadableMemoryShowsEverything(t *testing.T) {
	for _, raw := range []string{"", "   ", "{{{", "null"} {
		if m := Load(raw); len(m.Records) != 0 {
			t.Errorf("Load(%q) invented records: %+v", raw, m.Records)
		}
	}
}

func TestRecordsThatCannotBeActedOnAreDropped(t *testing.T) {
	m := Load(`{"records":[{"key":"","verdict":"one_time"},{"key":"k","verdict":"maybe"},{"key":"ok","verdict":"expected"}]}`)
	if len(m.Records) != 1 || m.Records[0].Key != "ok" {
		t.Errorf("kept records this package cannot act on: %+v", m.Records)
	}
	if m2 := (Memory{}).Record(Record{Key: " ", Verdict: OneTime}); len(m2.Records) != 0 {
		t.Error("recorded a judgement against a blank key")
	}
}

func TestVerdictsAreOfferedConclusiveFirst(t *testing.T) {
	vs := Verdicts()
	if len(vs) != 5 {
		t.Fatalf("verdicts = %d, want 5", len(vs))
	}
	if vs[len(vs)-1] != Investigate {
		t.Errorf("last = %q, want the one that resolves nothing offered last", vs[len(vs)-1])
	}
	for _, v := range vs {
		if !v.Valid() {
			t.Errorf("%q is offered but not valid", v)
		}
	}
}
