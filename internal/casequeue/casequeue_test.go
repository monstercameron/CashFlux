// SPDX-License-Identifier: MIT

package casequeue

import "testing"

func sig(kind SignalKind, id, subjKind, subjID, title string, sev Severity) Signal {
	return Signal{Kind: kind, ID: id, SubjectKind: subjKind, SubjectID: subjID,
		Title: title, Severity: sev}
}

// The whole point: one situation, four surfaces, one row.
func TestSignalsAboutOneSubjectBecomeOneCase(t *testing.T) {
	sigs := []Signal{
		sig(SignalNotification, "n1", "account", "a-check", "Checking is low", SeverityWarning),
		sig(SignalTask, "t1", "account", "a-check", "Move money to Checking", SeverityInfo),
		sig(SignalInsight, "i1", "account", "a-check", "Checking runs out on the 23rd", SeverityCritical),
		sig(SignalReview, "r1", "account", "a-check", "Unmatched payment", SeverityInfo),
	}
	cases := Build(sigs)
	if len(cases) != 1 {
		t.Fatalf("got %d cases, want 1: %+v", len(cases), cases)
	}
	c := cases[0]
	if len(c.Signals) != 4 {
		t.Errorf("case holds %d signals, want 4", len(c.Signals))
	}
	if c.SurfaceCount() != 4 {
		t.Errorf("SurfaceCount = %d, want 4", c.SurfaceCount())
	}
	// The worst thing about a situation is what should name it.
	if c.Title != "Checking runs out on the 23rd" {
		t.Errorf("Title = %q, want the critical signal's", c.Title)
	}
	if c.Severity != SeverityCritical {
		t.Errorf("Severity = %v, want critical", c.Severity)
	}
	if c.Key != "account:a-check" {
		t.Errorf("Key = %q", c.Key)
	}
}

// Same kind, different subjects: two cases. Grouping by kind would merge
// unrelated problems into one unreadable row.
func TestDifferentSubjectsStaySeparate(t *testing.T) {
	cases := Build([]Signal{
		sig(SignalNotification, "n1", "account", "a", "A is low", SeverityWarning),
		sig(SignalNotification, "n2", "account", "b", "B is low", SeverityWarning),
	})
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}
}

// An unattributed signal is not evidence that two unattributed signals are about
// the same thing; merging them would produce a case that means nothing.
func TestUnattributedSignalsAreNotMerged(t *testing.T) {
	cases := Build([]Signal{
		{Kind: SignalInsight, ID: "i1", Title: "Something general", Severity: SeverityInfo},
		{Kind: SignalInsight, ID: "i2", Title: "Something else general", Severity: SeverityInfo},
	})
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2 — unattributed signals were merged", len(cases))
	}
}

// The same money reported by three surfaces is ONE amount. Summing would triple
// it, which is the exact "the app disagrees with itself" failure the E-series is
// trying to eliminate.
func TestAmountIsTheLargestNotTheSum(t *testing.T) {
	a := sig(SignalNotification, "n1", "account", "a", "Low", SeverityWarning)
	a.AmountMinor, a.HasAmount = -18000, true
	b := sig(SignalInsight, "i1", "account", "a", "Low", SeverityWarning)
	b.AmountMinor, b.HasAmount = -18000, true
	c := sig(SignalTask, "t1", "account", "a", "Fix it", SeverityInfo)
	c.AmountMinor, c.HasAmount = -5000, true

	got := Build([]Signal{a, b, c})[0]
	if !got.HasAmount || got.AmountMinor != -18000 {
		t.Errorf("AmountMinor = %d,%v want -18000,true", got.AmountMinor, got.HasAmount)
	}
}

// A case closes only when EVERY signal has cleared. Closing on the first would
// hide the parts that had not.
func TestACaseClosesOnlyWhenEverySignalHasCleared(t *testing.T) {
	one := sig(SignalNotification, "n1", "bill", "b1", "Overdue", SeverityWarning)
	one.Cleared = true
	two := sig(SignalTask, "t1", "bill", "b1", "Pay it", SeverityInfo)

	partial := Build([]Signal{one, two})[0]
	if partial.Closed {
		t.Error("a case closed while one signal was still outstanding")
	}
	two.Cleared = true
	full := Build([]Signal{one, two})[0]
	if !full.Closed {
		t.Error("a case with every signal cleared did not close itself")
	}
	// Open / SelfClosed partition it.
	all := Build([]Signal{one, two})
	if len(Open(all)) != 0 || len(SelfClosed(all)) != 1 {
		t.Errorf("Open=%d SelfClosed=%d", len(Open(all)), len(SelfClosed(all)))
	}
}

// A queue is a list of WORK. A critical situation with nothing to do about it
// belongs below a warning the reader can clear in one click — an unactionable
// item at the top is a wall.
func TestActionabilityOutranksSeverity(t *testing.T) {
	stuck := sig(SignalInsight, "i1", "account", "a", "Critical but nothing to do", SeverityCritical)
	doable := sig(SignalNotification, "n1", "bill", "b", "Warning you can clear", SeverityWarning)
	doable.Actionable = true

	cases := Build([]Signal{stuck, doable})
	if len(cases) != 2 {
		t.Fatalf("got %d cases", len(cases))
	}
	if !cases[0].Actionable {
		t.Errorf("the actionable case is not first: %q then %q", cases[0].Title, cases[1].Title)
	}
	// …and among actionable cases, severity still decides.
	worse := sig(SignalNotification, "n2", "bill", "c", "Critical and doable", SeverityCritical)
	worse.Actionable = true
	ranked := Build([]Signal{doable, worse, stuck})
	if ranked[0].SubjectID != "c" {
		t.Errorf("ranking = %q first, want the critical actionable one", ranked[0].SubjectID)
	}
}

func TestQueueOrderIsStable(t *testing.T) {
	sigs := []Signal{
		sig(SignalNotification, "n1", "account", "b", "B", SeverityWarning),
		sig(SignalNotification, "n2", "account", "a", "A", SeverityWarning),
	}
	first, second := Build(sigs), Build(sigs)
	for i := range first {
		if first[i].Key != second[i].Key {
			t.Fatalf("order changed between identical runs at %d", i)
		}
	}
	// Equal rank falls back to the key, so the list does not reshuffle.
	if first[0].Key != "account:a" {
		t.Errorf("tie broken as %q, want the lower key", first[0].Key)
	}
}

func TestIDsForLetsEachSurfaceDismissItsOwn(t *testing.T) {
	c := Build([]Signal{
		sig(SignalNotification, "n1", "bill", "b", "x", SeverityWarning),
		sig(SignalNotification, "n2", "bill", "b", "y", SeverityWarning),
		sig(SignalTask, "t1", "bill", "b", "z", SeverityInfo),
	})[0]

	if got := c.IDsFor(SignalNotification); len(got) != 2 {
		t.Errorf("IDsFor(notification) = %v", got)
	}
	if got := c.IDsFor(SignalTask); len(got) != 1 || got[0] != "t1" {
		t.Errorf("IDsFor(task) = %v", got)
	}
	if got := c.IDsFor(SignalReview); len(got) != 0 {
		t.Errorf("IDsFor for an absent kind = %v", got)
	}
	if !c.Has(SignalTask) || c.Has(SignalReview) {
		t.Error("Has disagrees with IDsFor")
	}
}

func TestTop(t *testing.T) {
	var sigs []Signal
	for _, id := range []string{"a", "b", "c", "d"} {
		s := sig(SignalNotification, "n-"+id, "account", id, id, SeverityWarning)
		s.Actionable = true
		sigs = append(sigs, s)
	}
	if got := Top(Build(sigs), 3); len(got) != 3 {
		t.Errorf("Top(3) returned %d", len(got))
	}
	// A caller asking for zero means zero.
	if got := Top(Build(sigs), 0); got != nil {
		t.Errorf("Top(0) returned %d", len(got))
	}
	if got := Top(Build(sigs), 99); len(got) != 4 {
		t.Errorf("Top(99) returned %d, want all 4", len(got))
	}
	// Closed cases never reach the top strip.
	closed := sig(SignalNotification, "n-z", "account", "z", "z", SeverityCritical)
	closed.Cleared, closed.Actionable = true, true
	for _, c := range Top(Build(append(sigs, closed)), 5) {
		if c.SubjectID == "z" {
			t.Error("a self-closed case reached the top strip")
		}
	}
}

// The gap between signals and cases is the noise the merge removed — the number
// worth showing, because it is what justifies one row standing for four.
func TestSummarizeReportsWhatTheMergeRemoved(t *testing.T) {
	sigs := []Signal{
		sig(SignalNotification, "n1", "account", "a", "x", SeverityCritical),
		sig(SignalTask, "t1", "account", "a", "y", SeverityInfo),
		sig(SignalInsight, "i1", "account", "a", "z", SeverityInfo),
		sig(SignalNotification, "n2", "bill", "b", "w", SeverityWarning),
	}
	sigs[0].Actionable = true
	s := Summarize(Build(sigs))
	if s.Cases != 2 || s.Signals != 4 {
		t.Errorf("Summary = %+v, want 2 cases from 4 signals", s)
	}
	if s.Merged() != 2 {
		t.Errorf("Merged = %d, want 2", s.Merged())
	}
	if s.Critical != 1 || s.Actionable != 1 {
		t.Errorf("Summary = %+v", s)
	}
	// Merged never goes negative when there is nothing to merge.
	if got := (Summary{Cases: 3, Signals: 3}).Merged(); got != 0 {
		t.Errorf("Merged with nothing merged = %d", got)
	}
}

func TestEmptyInput(t *testing.T) {
	if got := Build(nil); len(got) != 0 {
		t.Errorf("Build(nil) = %+v", got)
	}
	if s := Summarize(nil); s.Cases != 0 || s.Merged() != 0 {
		t.Errorf("Summarize(nil) = %+v", s)
	}
}

// The join key folds case and whitespace, so signals written by different
// surfaces still land in the same case.
func TestSubjectKeyIsForgivingAboutFormatting(t *testing.T) {
	a := Signal{SubjectKind: "Account", SubjectID: "a-check"}
	b := Signal{SubjectKind: "account", SubjectID: "a-check "}
	if a.SubjectKey() != b.SubjectKey() {
		t.Errorf("%q != %q — two surfaces naming the same subject produced two cases",
			a.SubjectKey(), b.SubjectKey())
	}
}
