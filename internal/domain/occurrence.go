// SPDX-License-Identifier: MIT

package domain

import "time"

// RecurringOccurrence records the paid status for one specific due-date of a
// Recurring cash flow. A Recurring rule repeats indefinitely; each occurrence
// tracks whether that particular due-date has been settled.
//
// RecurringID links back to Recurring.ID. DueDate is the calendar date the
// payment was due (time-of-day component is ignored for matching purposes).
// PaidAt is nil until the occurrence is explicitly marked paid; a non-nil
// PaidAt means the occurrence was settled at that instant.
type RecurringOccurrence struct {
	RecurringID string
	DueDate     time.Time
	PaidAt      *time.Time
}

// OccurrenceKey returns a stable, human-readable string key for an occurrence.
// The key encodes the recurring-rule identifier and the due date in ISO-8601
// (YYYY-MM-DD) form, separated by a pipe character. It is safe to use as a
// map key or a storage row key.
//
// Example: "sub-42|2026-03-01"
func OccurrenceKey(recurringID string, due time.Time) string {
	return recurringID + "|" + due.Format("2006-01-02")
}

// IsPaid reports whether the given due-date occurrence of recurringID has been
// marked paid. It matches by recurringID and by the date portion of due
// (year-month-day only; the time-of-day component is ignored).
// Returns false when no matching occurrence exists.
func IsPaid(occs []RecurringOccurrence, recurringID string, due time.Time) bool {
	key := OccurrenceKey(recurringID, due)
	for _, o := range occs {
		if OccurrenceKey(o.RecurringID, o.DueDate) == key {
			return o.PaidAt != nil
		}
	}
	return false
}

// MarkPaid records recurringID/due as paid at now. It is idempotent: if a
// matching occurrence already exists its PaidAt is updated; otherwise a new
// occurrence is appended. The updated slice is returned; the caller must
// replace its own slice with the return value (the input is not modified in
// place, though its backing array may be reused).
func MarkPaid(occs []RecurringOccurrence, recurringID string, due time.Time, now time.Time) []RecurringOccurrence {
	key := OccurrenceKey(recurringID, due)
	paidAt := now // take a copy so the pointer is stable
	for i, o := range occs {
		if OccurrenceKey(o.RecurringID, o.DueDate) == key {
			occs[i].PaidAt = &paidAt
			return occs
		}
	}
	return append(occs, RecurringOccurrence{
		RecurringID: recurringID,
		DueDate:     due,
		PaidAt:      &paidAt,
	})
}

// PruneOccurrences returns a new slice containing only occurrences whose
// DueDate is on or after before (i.e. occurrences strictly before before are
// dropped). Call with before = now minus your retention window (e.g. 12 months)
// to bound storage growth without losing current or future entries.
func PruneOccurrences(occs []RecurringOccurrence, before time.Time) []RecurringOccurrence {
	kept := occs[:0:0] // nil-safe empty slice with zero capacity, avoids aliasing
	cutoff := before.Truncate(24 * time.Hour)
	for _, o := range occs {
		if !o.DueDate.Truncate(24 * time.Hour).Before(cutoff) {
			kept = append(kept, o)
		}
	}
	return kept
}
