// SPDX-License-Identifier: MIT

package domain

import (
	"testing"
	"time"
)

// date is a test helper that returns a UTC time.Time for the given date.
func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// paidPtr is a test helper that returns a pointer to a time.Time.
func paidPtr(t time.Time) *time.Time { return &t }

// ── OccurrenceKey ────────────────────────────────────────────────────────────

func TestOccurrenceKey(t *testing.T) {
	tests := []struct {
		name        string
		recurringID string
		due         time.Time
		want        string
	}{
		{
			name:        "standard date",
			recurringID: "sub-1",
			due:         date(2026, time.March, 1),
			want:        "sub-1|2026-03-01",
		},
		{
			name:        "empty id",
			recurringID: "",
			due:         date(2026, time.January, 15),
			want:        "|2026-01-15",
		},
		{
			name:        "time-of-day component is ignored in format",
			recurringID: "r-99",
			due:         time.Date(2026, time.December, 31, 23, 59, 59, 0, time.UTC),
			want:        "r-99|2026-12-31",
		},
		{
			name:        "id with pipe character encodes as-is",
			recurringID: "a|b",
			due:         date(2026, time.June, 1),
			want:        "a|b|2026-06-01",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := OccurrenceKey(tc.recurringID, tc.due)
			if got != tc.want {
				t.Errorf("OccurrenceKey(%q, %v) = %q; want %q", tc.recurringID, tc.due, got, tc.want)
			}
		})
	}
}

// ── IsPaid ───────────────────────────────────────────────────────────────────

func TestIsPaid(t *testing.T) {
	paid := paidPtr(date(2026, time.March, 2))
	occs := []RecurringOccurrence{
		{RecurringID: "r-1", DueDate: date(2026, time.March, 1), PaidAt: paid},
		{RecurringID: "r-1", DueDate: date(2026, time.April, 1), PaidAt: nil},
		{RecurringID: "r-2", DueDate: date(2026, time.March, 1), PaidAt: paid},
	}

	tests := []struct {
		name        string
		recurringID string
		due         time.Time
		want        bool
	}{
		{
			name:        "paid occurrence returns true",
			recurringID: "r-1",
			due:         date(2026, time.March, 1),
			want:        true,
		},
		{
			name:        "unpaid occurrence returns false",
			recurringID: "r-1",
			due:         date(2026, time.April, 1),
			want:        false,
		},
		{
			name:        "missing occurrence returns false",
			recurringID: "r-1",
			due:         date(2026, time.May, 1),
			want:        false,
		},
		{
			name:        "different recurringID same date returns correct result",
			recurringID: "r-2",
			due:         date(2026, time.March, 1),
			want:        true,
		},
		{
			name:        "empty slice returns false",
			recurringID: "r-1",
			due:         date(2026, time.March, 1),
			want:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Use the shared slice for all cases except "empty slice".
			in := occs
			if tc.name == "empty slice returns false" {
				in = nil
			}
			got := IsPaid(in, tc.recurringID, tc.due)
			if got != tc.want {
				t.Errorf("IsPaid(..., %q, %v) = %v; want %v", tc.recurringID, tc.due, got, tc.want)
			}
		})
	}
}

// TestIsPaid_DateOnlyMatching verifies that two times on the same calendar day
// but with different time-of-day components are treated as the same occurrence.
func TestIsPaid_DateOnlyMatching(t *testing.T) {
	noon := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	midnight := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	paid := paidPtr(noon)

	occs := []RecurringOccurrence{
		{RecurringID: "r-1", DueDate: noon, PaidAt: paid},
	}

	// Querying with midnight on the same day should find the noon-keyed entry,
	// because OccurrenceKey formats only the date portion.
	if !IsPaid(occs, "r-1", midnight) {
		t.Error("IsPaid: same calendar day with different time-of-day should match")
	}
}

// ── MarkPaid ─────────────────────────────────────────────────────────────────

func TestMarkPaid(t *testing.T) {
	now := date(2026, time.March, 2)

	t.Run("insert new occurrence when none exists", func(t *testing.T) {
		var occs []RecurringOccurrence
		occs = MarkPaid(occs, "r-1", date(2026, time.March, 1), now)
		if len(occs) != 1 {
			t.Fatalf("want 1 occurrence, got %d", len(occs))
		}
		o := occs[0]
		if o.RecurringID != "r-1" {
			t.Errorf("RecurringID = %q; want %q", o.RecurringID, "r-1")
		}
		if o.PaidAt == nil {
			t.Fatal("PaidAt should not be nil")
		}
		if !o.PaidAt.Equal(now) {
			t.Errorf("PaidAt = %v; want %v", *o.PaidAt, now)
		}
	})

	t.Run("update existing occurrence idempotently", func(t *testing.T) {
		first := date(2026, time.March, 2)
		occs := []RecurringOccurrence{
			{RecurringID: "r-1", DueDate: date(2026, time.March, 1), PaidAt: paidPtr(first)},
		}
		later := date(2026, time.March, 5)
		occs = MarkPaid(occs, "r-1", date(2026, time.March, 1), later)
		if len(occs) != 1 {
			t.Fatalf("slice length should stay 1, got %d", len(occs))
		}
		if occs[0].PaidAt == nil || !occs[0].PaidAt.Equal(later) {
			t.Errorf("PaidAt = %v; want %v", occs[0].PaidAt, later)
		}
	})

	t.Run("does not affect other occurrences", func(t *testing.T) {
		occs := []RecurringOccurrence{
			{RecurringID: "r-2", DueDate: date(2026, time.March, 1), PaidAt: nil},
		}
		occs = MarkPaid(occs, "r-1", date(2026, time.March, 1), now)
		if len(occs) != 2 {
			t.Fatalf("want 2 occurrences, got %d", len(occs))
		}
		// The r-2 entry must remain unpaid.
		for _, o := range occs {
			if o.RecurringID == "r-2" && o.PaidAt != nil {
				t.Error("r-2 occurrence was modified unexpectedly")
			}
		}
	})

	t.Run("calling twice is idempotent on length", func(t *testing.T) {
		var occs []RecurringOccurrence
		occs = MarkPaid(occs, "r-1", date(2026, time.March, 1), now)
		occs = MarkPaid(occs, "r-1", date(2026, time.March, 1), now)
		if len(occs) != 1 {
			t.Errorf("want 1 occurrence after two identical MarkPaid calls, got %d", len(occs))
		}
	})
}

// ── PruneOccurrences ─────────────────────────────────────────────────────────

func TestPruneOccurrences(t *testing.T) {
	paid := paidPtr(date(2026, time.January, 1))
	occs := []RecurringOccurrence{
		{RecurringID: "r-1", DueDate: date(2025, time.January, 1), PaidAt: paid},  // old
		{RecurringID: "r-1", DueDate: date(2025, time.June, 1), PaidAt: paid},     // old
		{RecurringID: "r-1", DueDate: date(2026, time.January, 1), PaidAt: paid},  // on cutoff → kept
		{RecurringID: "r-1", DueDate: date(2026, time.March, 1), PaidAt: nil},     // recent
		{RecurringID: "r-2", DueDate: date(2025, time.December, 31), PaidAt: paid}, // old (one day before cutoff)
	}

	cutoff := date(2026, time.January, 1)
	got := PruneOccurrences(occs, cutoff)

	// Expect: 2026-01-01 (on cutoff, kept) and 2026-03-01 (recent, kept).
	if len(got) != 2 {
		t.Fatalf("want 2 occurrences after pruning, got %d", len(got))
	}
	for _, o := range got {
		if o.DueDate.Before(cutoff) {
			t.Errorf("pruned slice contains entry with DueDate %v which is before cutoff %v", o.DueDate, cutoff)
		}
	}
}

func TestPruneOccurrences_EmptyInput(t *testing.T) {
	got := PruneOccurrences(nil, date(2026, time.January, 1))
	if len(got) != 0 {
		t.Errorf("PruneOccurrences(nil, ...) = %v; want empty", got)
	}
}

func TestPruneOccurrences_AllKept(t *testing.T) {
	occs := []RecurringOccurrence{
		{RecurringID: "r-1", DueDate: date(2026, time.March, 1)},
		{RecurringID: "r-1", DueDate: date(2026, time.April, 1)},
	}
	before := date(2025, time.January, 1) // cutoff in the past — nothing dropped
	got := PruneOccurrences(occs, before)
	if len(got) != 2 {
		t.Errorf("want 2 kept, got %d", len(got))
	}
}

func TestPruneOccurrences_AllDropped(t *testing.T) {
	occs := []RecurringOccurrence{
		{RecurringID: "r-1", DueDate: date(2024, time.March, 1)},
		{RecurringID: "r-1", DueDate: date(2024, time.April, 1)},
	}
	before := date(2026, time.January, 1) // cutoff well in the future
	got := PruneOccurrences(occs, before)
	if len(got) != 0 {
		t.Errorf("want 0 kept, got %d", len(got))
	}
}

// TestPruneOccurrences_DoesNotMutateInput verifies that pruning returns a fresh
// slice and does not corrupt the original backing array.
func TestPruneOccurrences_DoesNotMutateInput(t *testing.T) {
	occs := []RecurringOccurrence{
		{RecurringID: "r-1", DueDate: date(2025, time.January, 1)},
		{RecurringID: "r-1", DueDate: date(2026, time.June, 1)},
	}
	original := make([]RecurringOccurrence, len(occs))
	copy(original, occs)

	PruneOccurrences(occs, date(2026, time.January, 1))

	for i, o := range occs {
		if o != original[i] {
			t.Errorf("input slice[%d] was mutated: got %+v, want %+v", i, o, original[i])
		}
	}
}
