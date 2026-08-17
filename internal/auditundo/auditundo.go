// SPDX-License-Identifier: MIT

// Package auditundo answers, for one audit entry, "can this be taken back —
// and how" (WF-AUDIT).
//
// The audit trail already records what changed, when, by whom, and the
// field-level before → after. The last clause of the ticket is "undo where
// safe", and the honest answer turned out to be more interesting than a button.
//
// # Why the log cannot reverse a change on its own
//
// `auditlog.FieldChange` holds DISPLAY STRINGS — formatted, localized and
// redacted by the recorder. "$1,240.00" and "Marcus" are what a person needs to
// read; they are not values anything can write back. So the log can describe a
// change precisely and cannot apply its inverse, and no amount of parsing fixes
// that: a redacted field has lost the information entirely, on purpose.
//
// Pretending otherwise would be the worst option — an Undo button that works on
// simple fields and silently mangles money or a redacted note is worse than no
// button, because people would learn to trust it.
//
// # What can honestly be offered
//
// A dataset CHECKPOINT taken before the change rolls the whole dataset back to
// that moment. It is coarse — it takes everything since with it — and it is
// real, which beats a precise-looking control that is not. This package finds
// the newest checkpoint at or before an entry and says so, or says plainly that
// nothing covers it.
package auditundo

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/checkpoint"
)

// Route is how (or whether) an entry can be taken back.
type Route string

const (
	// RouteNone means nothing on hand can reverse it.
	RouteNone Route = "none"
	// RouteCheckpoint means a dataset snapshot from before the change can roll
	// back to it — coarse, and honest about being coarse.
	RouteCheckpoint Route = "checkpoint"
)

// Reason is why an entry cannot be reversed, or why the only route is coarse.
//
// Named reasons rather than a free-text string so a surface can phrase each one
// in the user's language, and so a new reason is a compile-time decision rather
// than a sentence somebody invents at a call site.
type Reason string

const (
	// ReasonNoCheckpoint means no snapshot predates the change.
	ReasonNoCheckpoint Reason = "noCheckpoint"
	// ReasonValuesNotStored means the log holds display text rather than values,
	// so a field-level revert is impossible from the record alone.
	ReasonValuesNotStored Reason = "valuesNotStored"
	// ReasonCoarse means the available checkpoint also reverses everything that
	// happened after the change.
	ReasonCoarse Reason = "coarse"
)

// Assessment is what can be done about one entry.
type Assessment struct {
	Route Route
	// Reasons are ordered most-to-least important — the first is what a surface
	// should lead with if it has room for one line.
	Reasons []Reason
	// CheckpointID and CheckpointAt identify the snapshot that would be restored,
	// when Route is RouteCheckpoint.
	CheckpointID string
	CheckpointAt time.Time
	// AlsoReverses is how many LATER audit entries the same restore would undo.
	//
	// The number that decides whether a coarse undo is acceptable. "This also
	// reverses 2 later changes" and "this also reverses 340" are the same
	// mechanism and completely different decisions, and only this figure tells
	// them apart.
	AlsoReverses int
}

// Possible reports whether anything can be done at all.
func (a Assessment) Possible() bool { return a.Route != RouteNone }

// Assess decides how an entry could be taken back.
//
// entries is the full audit log (any order); cps the available checkpoints. Both
// may be empty.
func Assess(e auditlog.Entry, entries []auditlog.Entry, cps []checkpoint.Checkpoint) Assessment {
	a := Assessment{Route: RouteNone, Reasons: []Reason{ReasonValuesNotStored}}
	if e.At.IsZero() {
		// An entry with no timestamp cannot be placed against any checkpoint, and
		// guessing which side of one it falls on is how the wrong day gets rolled
		// back.
		return a
	}

	cp, ok := newestBefore(cps, e.At)
	if !ok {
		a.Reasons = append(a.Reasons, ReasonNoCheckpoint)
		return a
	}
	a.Route = RouteCheckpoint
	a.CheckpointID, a.CheckpointAt = cp.ID, cp.At
	a.AlsoReverses = countAfter(entries, e.At, e.ID)
	a.Reasons = []Reason{ReasonCoarse, ReasonValuesNotStored}
	return a
}

// newestBefore returns the latest checkpoint taken at or before t.
//
// At-or-before, never after: a snapshot taken AFTER a change already contains
// it, so restoring it would not undo anything — it would look like the undo
// silently failed, which is worse than refusing.
func newestBefore(cps []checkpoint.Checkpoint, t time.Time) (checkpoint.Checkpoint, bool) {
	sorted := append([]checkpoint.Checkpoint(nil), cps...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].At.Before(sorted[j].At) })
	var best checkpoint.Checkpoint
	found := false
	for _, cp := range sorted {
		if cp.At.IsZero() || cp.At.After(t) {
			continue
		}
		best, found = cp, true
	}
	return best, found
}

// countAfter counts entries recorded strictly after t, excluding the entry
// itself.
//
// Strictly after, so an entry sharing a timestamp with the one being assessed is
// not counted as collateral — same-instant writes are usually one action the
// recorder split into several rows, and reporting them as extra losses would
// make every undo look more destructive than it is.
func countAfter(entries []auditlog.Entry, t time.Time, selfID string) int {
	n := 0
	for _, e := range entries {
		if e.ID == selfID || e.At.IsZero() {
			continue
		}
		if e.At.After(t) {
			n++
		}
	}
	return n
}
