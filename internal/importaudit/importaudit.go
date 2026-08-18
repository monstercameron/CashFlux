// SPDX-License-Identifier: MIT

// Package importaudit reconciles what an import SAID it did against what is
// actually in the ledger now (C687).
//
// # The discrepancy this exists to explain
//
// The import history said 688 transaction records had been added; the ledger
// showed 565. Both numbers were true and neither was checkable, because they
// count different things and nothing connected them:
//
//   - 688 is the sum of what each import run reported at the time it ran.
//   - 565 is how many rows are in the ledger today.
//
// Rows leave. A duplicate merge removes some, a bulk delete removes more, an
// import gets rolled back. None of that was recorded against the run that created
// the rows, so the gap had no explanation and no way to get one — a person
// looking at it cannot tell whether 123 rows were deliberately cleaned up or
// silently lost.
//
// # What makes the link possible
//
// domain.Transaction.SourceDocID. The field existed and no import path set it —
// only the sample data did — so a row could not say which run it came from. With
// it set, "how many rows from this import are still here" is a count rather than
// a guess, and the difference is a fact rather than a mystery.
//
// # Honesty about what cannot be known
//
// Documents written before that linkage exists carry no id on their rows. For
// those, the live count is genuinely UNKNOWABLE, and reporting zero would be a
// confident lie in exactly the direction that alarms people. Tally says so
// instead, through ActiveKnown.
package importaudit

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Tally is what one import run did, and what has become of it since.
type Tally struct {
	// DocumentID and Filename identify the run.
	DocumentID string
	Filename   string
	// At is when it ran.
	At time.Time
	// Imported is how many rows it wrote at the time.
	Imported int
	// Duplicates is how many rows the ledger already had. Not a failure: the
	// safeguard working.
	Duplicates int
	// Failed is how many rows could not be read. This one IS a problem — that
	// money is not in the ledger and nothing else will notice.
	Failed int
	// Skipped is Duplicates+Failed for a run that recorded them separately, and
	// the single conflated figure for one written before the split.
	Skipped int
	// Presented is how many rows the file offered in total.
	Presented int
	// Active is how many of this run's rows are still in the ledger.
	Active int
	// Removed is how many have gone since, whether merged, deleted or rolled back.
	Removed int
	// ActiveKnown reports whether Active and Removed mean anything. It is false
	// for a run whose rows carry no link back to it, where the honest answer is
	// "cannot tell" rather than zero.
	ActiveKnown bool
}

// Clean reports whether a run needs no explanation: nothing failed and nothing
// has gone missing since.
func (t Tally) Clean() bool {
	return t.Failed == 0 && (!t.ActiveKnown || t.Removed == 0)
}

// Split reports whether this run recorded duplicates and failures separately. A
// run that did not can only be described as "skipped N", and a reader should be
// told which of the two they are looking at.
func (t Tally) Split() bool { return t.Duplicates > 0 || t.Failed > 0 || t.Skipped == 0 }

// For computes one document's tally against the live ledger.
func For(doc domain.Document, txns []domain.Transaction) Tally {
	t := Tally{
		DocumentID: doc.ID,
		Filename:   doc.Filename,
		At:         doc.UploadedAt,
		Imported:   doc.RowCount,
		Duplicates: doc.DuplicateCount,
		Failed:     doc.FailedCount,
	}
	// A document written before the counters were split records only the conflated
	// total, and splitting it after the fact would invent a fact. Duplicates and
	// Failed stay zero for those, Skipped carries the number, and Presented is
	// computed from whichever the document actually recorded.
	t.Skipped = doc.SkippedCount
	if doc.DuplicateCount > 0 || doc.FailedCount > 0 {
		t.Skipped = t.Duplicates + t.Failed
	}
	t.Presented = t.Imported + t.Skipped

	if doc.ID == "" {
		return t
	}
	for _, x := range txns {
		if x.SourceDocID == doc.ID {
			t.Active++
		}
	}
	// The link is only trustworthy when the run claims rows AND at least one of
	// them still carries the id. A run that imported nothing is trivially known.
	t.ActiveKnown = t.Imported == 0 || t.Active > 0
	if t.ActiveKnown {
		t.Removed = t.Imported - t.Active
		if t.Removed < 0 {
			// More live rows carry this id than the run reported importing. That
			// happens when rows were duplicated from an imported one, and it is not
			// a loss, so it is reported as none rather than as a negative.
			t.Removed = 0
		}
	}
	return t
}

// All tallies every document, newest run first — the order an import history
// reads in.
func All(docs []domain.Document, txns []domain.Transaction) []Tally {
	out := make([]Tally, 0, len(docs))
	for _, d := range docs {
		out = append(out, For(d, txns))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.After(out[j].At)
		}
		return out[i].DocumentID < out[j].DocumentID
	})
	return out
}

// Totals is the whole import history at a glance.
type Totals struct {
	// Runs is how many imports there have been.
	Runs int
	// Imported, Duplicates and Failed sum the per-run figures.
	Imported, Duplicates, Failed int
	// Active and Removed cover only the runs whose rows can be traced.
	Active, Removed int
	// Untraceable is how many runs carry no link back to their rows, so a reader
	// knows the Active figure covers less than the whole history.
	Untraceable int
}

// Sum folds a list of tallies.
func Sum(tallies []Tally) Totals {
	var t Totals
	for _, x := range tallies {
		t.Runs++
		t.Imported += x.Imported
		t.Duplicates += x.Duplicates
		t.Failed += x.Failed
		if x.ActiveKnown {
			t.Active += x.Active
			t.Removed += x.Removed
		} else {
			t.Untraceable++
		}
	}
	return t
}

// Explains reports whether the traced figures account for the whole ledger — the
// question "688 imported but 565 present" is really asking. It is false when any
// run is untraceable, because then the sum simply does not cover everything.
func (t Totals) Explains() bool { return t.Untraceable == 0 }
