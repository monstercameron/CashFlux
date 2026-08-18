// SPDX-License-Identifier: MIT

package importaudit

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func at(day int) time.Time { return time.Date(2026, 7, day, 9, 0, 0, 0, time.UTC) }

func doc(id string, day, imported, dupes, failed int) domain.Document {
	return domain.Document{
		ID: id, Filename: id + ".csv", Kind: domain.DocCSV, UploadedAt: at(day),
		Status: domain.DocImported, RowCount: imported,
		DuplicateCount: dupes, FailedCount: failed, SkippedCount: dupes + failed,
	}
}

func rows(docID string, n int) []domain.Transaction {
	out := make([]domain.Transaction, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, domain.Transaction{
			ID: docID + "-" + string(rune('a'+i)), AccountID: "chk", Desc: "row",
			Amount: money.New(-100, "USD"), Date: at(1), SourceDocID: docID,
		})
	}
	return out
}

// The whole point: a run that imported 10 rows and has 7 left says so, and names
// the 3 that went rather than leaving a silent gap.
func TestTallyCountsWhatSurvived(t *testing.T) {
	d := doc("d1", 3, 10, 2, 1)
	got := For(d, rows("d1", 7))

	if got.Imported != 10 || got.Duplicates != 2 || got.Failed != 1 {
		t.Errorf("counts = %+v, want 10/2/1", got)
	}
	if got.Presented != 13 {
		t.Errorf("Presented = %d, want 13", got.Presented)
	}
	if !got.ActiveKnown {
		t.Fatalf("ActiveKnown = false, but the rows carry the run id")
	}
	if got.Active != 7 || got.Removed != 3 {
		t.Errorf("Active/Removed = %d/%d, want 7/3", got.Active, got.Removed)
	}
	if got.Clean() {
		t.Errorf("Clean() = true, but a row failed to parse and three rows went")
	}
}

// A run whose rows carry no link back is genuinely untraceable. Reporting zero
// live rows would be a confident lie, in the direction that alarms people most.
func TestLegacyRunReportsThatItCannotBeTraced(t *testing.T) {
	d := domain.Document{ID: "old", Kind: domain.DocCSV, UploadedAt: at(1),
		Status: domain.DocImported, RowCount: 120, SkippedCount: 5}

	// Plenty of rows in the ledger — none of them stamped.
	unlinked := rows("somewhere-else", 200)
	got := For(d, unlinked)

	if got.ActiveKnown {
		t.Fatalf("ActiveKnown = true, but no row carries this run's id")
	}
	if got.Active != 0 || got.Removed != 0 {
		t.Errorf("Active/Removed = %d/%d, want 0/0 alongside ActiveKnown=false", got.Active, got.Removed)
	}
	if !got.Clean() {
		t.Errorf("Clean() = false — an untraceable run has not been shown to have lost anything")
	}
	// The conflated legacy figure is reported as itself, not split into invented
	// duplicate and failure counts.
	if got.Duplicates != 0 || got.Failed != 0 {
		t.Errorf("legacy skipped count was split into %d/%d — that is an invented fact", got.Duplicates, got.Failed)
	}
	if got.Skipped != 5 {
		t.Errorf("Skipped = %d, want the recorded 5", got.Skipped)
	}
	if got.Split() {
		t.Errorf("Split() = true for a run that never recorded the split")
	}
}

// A run that imported nothing is trivially traceable: there is nothing to find.
func TestRunThatImportedNothingIsKnown(t *testing.T) {
	d := doc("empty", 2, 0, 12, 0)
	got := For(d, nil)
	if !got.ActiveKnown {
		t.Errorf("ActiveKnown = false for a run that wrote no rows")
	}
	if got.Removed != 0 {
		t.Errorf("Removed = %d, want 0", got.Removed)
	}
	if !got.Clean() {
		t.Errorf("Clean() = false — every row being a duplicate is the safeguard working, not a fault")
	}
}

// Duplicating an imported row makes more rows carry the id than the run wrote.
// That is not a loss and must not report as a negative.
func TestMoreLiveRowsThanImportedIsNotALoss(t *testing.T) {
	d := doc("d1", 3, 5, 0, 0)
	got := For(d, rows("d1", 8))
	if got.Removed != 0 {
		t.Errorf("Removed = %d, want 0 — more rows than imported is not a loss", got.Removed)
	}
	if got.Active != 8 {
		t.Errorf("Active = %d, want 8", got.Active)
	}
}

// The history reads newest first, and the order must not depend on how the store
// happened to return the documents.
func TestAllOrdersNewestFirstAndStably(t *testing.T) {
	docs := []domain.Document{doc("b", 1, 1, 0, 0), doc("a", 9, 1, 0, 0), doc("c", 5, 1, 0, 0)}
	got := All(docs, nil)
	if len(got) != 3 {
		t.Fatalf("got %d tallies", len(got))
	}
	if got[0].DocumentID != "a" || got[1].DocumentID != "c" || got[2].DocumentID != "b" {
		t.Errorf("order = %s/%s/%s, want a/c/b", got[0].DocumentID, got[1].DocumentID, got[2].DocumentID)
	}

	same := []domain.Document{doc("z", 4, 1, 0, 0), doc("y", 4, 1, 0, 0)}
	tied := All(same, nil)
	if tied[0].DocumentID != "y" {
		t.Errorf("a tie broke to %q; it must be deterministic", tied[0].DocumentID)
	}
}

// The reported symptom, reproduced: the history claims more than the ledger
// holds, and the totals now say exactly where the difference went.
func TestTotalsExplainTheReportedDiscrepancy(t *testing.T) {
	docs := []domain.Document{doc("d1", 1, 400, 10, 0), doc("d2", 2, 288, 3, 2)}
	ledger := append(rows("d1", 330), rows("d2", 235)...)

	tallies := All(docs, ledger)
	got := Sum(tallies)

	if got.Imported != 688 {
		t.Errorf("Imported = %d, want the 688 the history claimed", got.Imported)
	}
	if got.Active != 565 {
		t.Errorf("Active = %d, want the 565 the ledger holds", got.Active)
	}
	if got.Removed != 123 {
		t.Errorf("Removed = %d, want 123 — the difference, now named", got.Removed)
	}
	if !got.Explains() {
		t.Errorf("Explains() = false, but every run is traceable")
	}
	if got.Failed != 2 || got.Duplicates != 13 {
		t.Errorf("Failed/Duplicates = %d/%d, want 2/13", got.Failed, got.Duplicates)
	}
}

// One untraceable run means the totals cover less than the whole history, and a
// reader has to be told rather than shown a figure that silently omits it.
func TestUntraceableRunsAreCountedAndDisclosed(t *testing.T) {
	legacy := domain.Document{ID: "old", Kind: domain.DocCSV, UploadedAt: at(1),
		Status: domain.DocImported, RowCount: 100}
	current := doc("new", 5, 20, 0, 0)

	got := Sum(All([]domain.Document{legacy, current}, rows("new", 20)))
	if got.Untraceable != 1 {
		t.Errorf("Untraceable = %d, want 1", got.Untraceable)
	}
	if got.Explains() {
		t.Errorf("Explains() = true while a run cannot be traced")
	}
	if got.Active != 20 {
		t.Errorf("Active = %d, want only the traceable run's rows", got.Active)
	}
	if got.Imported != 120 {
		t.Errorf("Imported = %d, want both runs counted", got.Imported)
	}
}

func TestDocumentWithoutAnIDIsNotMatchedAgainstUnstampedRows(t *testing.T) {
	d := domain.Document{Kind: domain.DocCSV, UploadedAt: at(1), RowCount: 3}
	unstamped := []domain.Transaction{{ID: "x", AccountID: "chk", Amount: money.New(-1, "USD"), Date: at(1)}}
	got := For(d, unstamped)
	if got.Active != 0 || got.ActiveKnown {
		t.Errorf("tally = %+v, want no match and no claim for an id-less document", got)
	}
}
