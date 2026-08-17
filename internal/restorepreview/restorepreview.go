// SPDX-License-Identifier: MIT

// Package restorepreview says what restoring a backup would actually replace
// (WF-BACKUP).
//
// Restoring is the single most destructive thing this app can do: it discards
// the current dataset entirely and puts an older one in its place. Until now the
// only thing standing in front of it was a confirmation box that said, in
// effect, "are you sure" — a question nobody can answer, because it does not
// tell you what you are about to lose.
//
// The comparison is deliberately coarse: counts per entity, plus the dates each
// side covers. A field-level diff of two whole datasets would be more precise
// and much less useful — the question at this moment is not "which of my 4,000
// transactions differ" but "am I about to throw away the last three weeks".
package restorepreview

import (
	"time"
)

// Counts is how much of each thing a dataset holds.
//
// A flat struct rather than a map so a missing entity is a compile error rather
// than a silently absent row in the comparison — an entity nobody remembered to
// count is exactly the one somebody loses.
type Counts struct {
	Accounts     int
	Transactions int
	Budgets      int
	Goals        int
	Categories   int
	Tasks        int
	Holdings     int
	Rules        int
}

// Total is every record in the dataset.
func (c Counts) Total() int {
	return c.Accounts + c.Transactions + c.Budgets + c.Goals +
		c.Categories + c.Tasks + c.Holdings + c.Rules
}

// Side describes one dataset in the comparison.
type Side struct {
	Counts Counts
	// Newest is the most recent transaction date, and is what actually answers
	// "how old is this backup" — a file's own timestamp says when it was WRITTEN,
	// which is not the same thing and is trivially wrong after a file is copied.
	Newest time.Time
}

// Line is one entity's before-and-after.
type Line struct {
	Entity string
	Now    int
	After  int
	// Delta is After minus Now: NEGATIVE means the restore removes records.
	//
	// Negative-is-loss rather than the reverse, because loss is the thing this
	// screen exists to warn about and it should not require a sign convention to
	// notice.
	Delta int
}

// Loses reports whether this entity ends up with fewer records.
func (l Line) Loses() bool { return l.Delta < 0 }

// Preview is the whole comparison.
type Preview struct {
	Lines []Line
	// LostRecords is the total number of records that would disappear, counting
	// only entities that SHRINK.
	//
	// Shrinking entities only, never netted against growth. A restore that adds
	// 50 budgets and drops 400 transactions has not "gained 350 things" — it has
	// lost four hundred transactions, and a net figure would say otherwise.
	LostRecords int
	// GapDays is how much more recent the current data is than the backup, and
	// Stale says whether the backup is behind at all.
	GapDays int
	Stale   bool
	// Newer says the BACKUP is more recent than the current data, which usually
	// means the user is restoring onto a fresh or partly-wiped install — worth
	// saying, because it is the one case where restoring is obviously right.
	Newer bool
}

// AnyLoss reports whether the restore destroys records anywhere.
func (p Preview) AnyLoss() bool { return p.LostRecords > 0 }

// Build compares the current dataset against a backup's.
func Build(now, backup Side) Preview {
	var p Preview
	add := func(entity string, a, b int) {
		if a == 0 && b == 0 {
			// An entity nobody uses is noise in a list somebody is reading under
			// pressure. Omitted rather than shown as "0 → 0".
			return
		}
		l := Line{Entity: entity, Now: a, After: b, Delta: b - a}
		p.Lines = append(p.Lines, l)
		if l.Loses() {
			p.LostRecords += -l.Delta
		}
	}
	add("accounts", now.Counts.Accounts, backup.Counts.Accounts)
	add("transactions", now.Counts.Transactions, backup.Counts.Transactions)
	add("categories", now.Counts.Categories, backup.Counts.Categories)
	add("budgets", now.Counts.Budgets, backup.Counts.Budgets)
	add("goals", now.Counts.Goals, backup.Counts.Goals)
	add("tasks", now.Counts.Tasks, backup.Counts.Tasks)
	add("holdings", now.Counts.Holdings, backup.Counts.Holdings)
	add("rules", now.Counts.Rules, backup.Counts.Rules)

	if !now.Newest.IsZero() && !backup.Newest.IsZero() {
		days := int(now.Newest.Sub(backup.Newest).Hours() / 24)
		switch {
		case days > 0:
			p.GapDays, p.Stale = days, true
		case days < 0:
			p.GapDays, p.Newer = -days, true
		}
	}
	return p
}
