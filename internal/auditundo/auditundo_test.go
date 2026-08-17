// SPDX-License-Identifier: MIT

package auditundo

import (
	"slices"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/checkpoint"
)

func at(h int) time.Time {
	return time.Date(2026, time.August, 17, h, 0, 0, 0, time.UTC)
}

func entry(id string, h int) auditlog.Entry {
	return auditlog.Entry{ID: id, At: at(h), Action: "update", EntityType: "account", EntityID: "a1"}
}

func cp(id string, h int) checkpoint.Checkpoint {
	return checkpoint.Checkpoint{ID: id, At: at(h), Label: "before something"}
}

func TestACheckpointBeforeTheChangeIsTheRoute(t *testing.T) {
	e := entry("e2", 12)
	a := Assess(e, []auditlog.Entry{e}, []checkpoint.Checkpoint{cp("c1", 9), cp("c2", 11)})
	if a.Route != RouteCheckpoint {
		t.Fatalf("route = %q, want %q", a.Route, RouteCheckpoint)
	}
	if a.CheckpointID != "c2" {
		t.Errorf("checkpoint = %q, want the NEWEST one before the change (c2)", a.CheckpointID)
	}
}

// A snapshot taken AFTER a change already contains it, so restoring it would not
// undo anything — it would look like the undo silently failed.
func TestACheckpointAfterTheChangeIsNotOffered(t *testing.T) {
	e := entry("e1", 10)
	a := Assess(e, []auditlog.Entry{e}, []checkpoint.Checkpoint{cp("c1", 11), cp("c2", 14)})
	if a.Possible() {
		t.Errorf("a later checkpoint was offered as an undo route: %+v", a)
	}
	if !hasReason(a, ReasonNoCheckpoint) {
		t.Errorf("reasons = %v, want it to say no snapshot predates the change", a.Reasons)
	}
}

func TestACheckpointAtTheExactMomentCounts(t *testing.T) {
	// At-or-before, not strictly before: a checkpoint taken as part of the same
	// action is the one that was deliberately placed to make it reversible.
	e := entry("e1", 10)
	a := Assess(e, []auditlog.Entry{e}, []checkpoint.Checkpoint{cp("c1", 10)})
	if a.Route != RouteCheckpoint || a.CheckpointID != "c1" {
		t.Errorf("assessment = %+v, want the same-instant checkpoint offered", a)
	}
}

func TestNoCheckpointsMeansNothingCanBeDone(t *testing.T) {
	e := entry("e1", 10)
	a := Assess(e, []auditlog.Entry{e}, nil)
	if a.Possible() {
		t.Error("no checkpoints must mean no route")
	}
	if !hasReason(a, ReasonValuesNotStored) {
		t.Error("the reason the log itself cannot revert must always be stated")
	}
}

// The figure that decides whether a coarse undo is acceptable: "also reverses 2"
// and "also reverses 340" are the same mechanism and different decisions.
func TestItCountsWhatElseTheRestoreWouldReverse(t *testing.T) {
	target := entry("e2", 12)
	all := []auditlog.Entry{
		entry("e1", 11), // before — untouched by the restore
		target,
		entry("e3", 13), entry("e4", 14), // after — collateral
	}
	a := Assess(target, all, []checkpoint.Checkpoint{cp("c1", 10)})
	if a.AlsoReverses != 2 {
		t.Errorf("also reverses = %d, want 2", a.AlsoReverses)
	}
}

func TestTheEntryItselfIsNotCountedAsCollateral(t *testing.T) {
	target := entry("e2", 12)
	a := Assess(target, []auditlog.Entry{target}, []checkpoint.Checkpoint{cp("c1", 10)})
	if a.AlsoReverses != 0 {
		t.Errorf("also reverses = %d, want 0 — the change being undone is not collateral", a.AlsoReverses)
	}
}

func TestSameInstantEntriesAreNotCountedAsCollateral(t *testing.T) {
	// Same-instant writes are usually one action the recorder split into several
	// rows; counting them would make every undo look more destructive than it is.
	target := entry("e2", 12)
	sibling := entry("e2b", 12)
	a := Assess(target, []auditlog.Entry{target, sibling}, []checkpoint.Checkpoint{cp("c1", 10)})
	if a.AlsoReverses != 0 {
		t.Errorf("also reverses = %d, want 0 for a same-instant sibling", a.AlsoReverses)
	}
}

func TestAnEntryWithNoTimestampIsNeverPlaced(t *testing.T) {
	// Guessing which side of a checkpoint an undated entry falls on is how the
	// wrong day gets rolled back.
	e := auditlog.Entry{ID: "x", Action: "update"}
	a := Assess(e, []auditlog.Entry{e}, []checkpoint.Checkpoint{cp("c1", 10)})
	if a.Possible() {
		t.Errorf("an undated entry produced a route: %+v", a)
	}
}

func TestUndatedCheckpointsAreIgnored(t *testing.T) {
	e := entry("e1", 12)
	a := Assess(e, []auditlog.Entry{e}, []checkpoint.Checkpoint{{ID: "broken"}})
	if a.Possible() {
		t.Errorf("a checkpoint with no timestamp was offered: %+v", a)
	}
}

// The coarseness has to lead: it is what the reader has to decide about.
func TestACoarseRouteSaysSoFirst(t *testing.T) {
	e := entry("e1", 12)
	a := Assess(e, []auditlog.Entry{e}, []checkpoint.Checkpoint{cp("c1", 10)})
	if len(a.Reasons) == 0 || a.Reasons[0] != ReasonCoarse {
		t.Errorf("reasons = %v, want coarseness stated first", a.Reasons)
	}
}

func TestTheValuesNotStoredReasonIsAlwaysPresent(t *testing.T) {
	// It is the standing fact about this log, true whether or not a checkpoint
	// happens to exist, and a surface should never imply a field-level revert.
	withCP := Assess(entry("a", 12), nil, []checkpoint.Checkpoint{cp("c", 10)})
	without := Assess(entry("b", 12), nil, nil)
	for _, a := range []Assessment{withCP, without} {
		if !hasReason(a, ReasonValuesNotStored) {
			t.Errorf("assessment %+v omitted the standing reason", a)
		}
	}
}

func hasReason(a Assessment, r Reason) bool { return slices.Contains(a.Reasons, r) }
