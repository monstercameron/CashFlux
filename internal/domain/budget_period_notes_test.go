// SPDX-License-Identifier: MIT

package domain

import (
	"testing"
	"time"
)

func TestBudgetPeriodNote(t *testing.T) {
	p := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	q := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	b := Budget{ID: "b1"}
	if got := b.PeriodNote(p); got != "" {
		t.Fatalf("empty budget: want no note, got %q", got)
	}

	// Set a note for December only.
	b = b.WithPeriodNote(p, "  December was high because we hosted  ")
	if got := b.PeriodNote(p); got != "December was high because we hosted" {
		t.Fatalf("Dec note: got %q", got)
	}
	if got := b.PeriodNote(q); got != "" {
		t.Fatalf("Nov note: want empty, got %q", got)
	}

	// A second period keeps both.
	b = b.WithPeriodNote(q, "quiet month")
	if got := b.PeriodNote(q); got != "quiet month" {
		t.Fatalf("Nov note after set: got %q", got)
	}
	if got := b.PeriodNote(p); got != "December was high because we hosted" {
		t.Fatalf("Dec note preserved: got %q", got)
	}

	// Clearing to empty drops the entry.
	b = b.WithPeriodNote(p, "   ")
	if got := b.PeriodNote(p); got != "" {
		t.Fatalf("cleared Dec note: got %q", got)
	}
	if _, ok := b.PeriodNotes["2026-12-01"]; ok {
		t.Fatalf("cleared entry should be deleted from map")
	}

	// Clearing the last note drops the map entirely.
	b = b.WithPeriodNote(q, "")
	if b.PeriodNotes != nil {
		t.Fatalf("emptied map should be nil, got %v", b.PeriodNotes)
	}
}

func TestBudgetWithPeriodNoteDoesNotMutateOriginal(t *testing.T) {
	p := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	orig := Budget{ID: "b1", PeriodNotes: map[string]string{"2026-12-01": "one"}}
	updated := orig.WithPeriodNote(p, "two")
	if orig.PeriodNote(p) != "one" {
		t.Fatalf("original mutated: %q", orig.PeriodNote(p))
	}
	if updated.PeriodNote(p) != "two" {
		t.Fatalf("copy not updated: %q", updated.PeriodNote(p))
	}
}
