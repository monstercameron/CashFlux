// SPDX-License-Identifier: MIT

package domain

import (
	"testing"
	"time"
)

var day1 = time.Date(2026, time.August, 1, 9, 0, 0, 0, time.UTC)

func TestMarkClearedRecordsTheMoment(t *testing.T) {
	tx := Transaction{Date: day1}.MarkCleared(true, day1.AddDate(0, 0, 3))
	if !tx.Cleared {
		t.Fatal("the flag was not set")
	}
	d, ok := tx.DaysToClear()
	if !ok || d != 3 {
		t.Errorf("days = %d, %v; want 3", d, ok)
	}
}

// A bulk action that touches a row again did not clear it a second time, and
// restamping would quietly reset every learned clearing window to zero days.
func TestReMarkingKeepsTheOriginalMoment(t *testing.T) {
	tx := Transaction{Date: day1}.MarkCleared(true, day1.AddDate(0, 0, 3))
	again := tx.MarkCleared(true, day1.AddDate(0, 0, 30))
	if !again.ClearedAt.Equal(day1.AddDate(0, 0, 3)) {
		t.Errorf("stamp moved to %v — a second mark is not a second clearing", again.ClearedAt)
	}
	if d, _ := again.DaysToClear(); d != 3 {
		t.Errorf("days = %d, want the original 3", d)
	}
}

// A transaction that has not cleared has no moment at which it did.
func TestUnclearingDropsTheMoment(t *testing.T) {
	tx := Transaction{Date: day1}.MarkCleared(true, day1.AddDate(0, 0, 3)).MarkCleared(false, day1)
	if tx.Cleared {
		t.Error("the flag survived un-clearing")
	}
	if !tx.ClearedAt.IsZero() {
		t.Errorf("stamp survived un-clearing: %v", tx.ClearedAt)
	}
	if _, ok := tx.DaysToClear(); ok {
		t.Error("an uncleared transaction reported a clearing time")
	}
	// And re-clearing it later takes the NEW moment, because this time it really
	// did clear then.
	back := tx.MarkCleared(true, day1.AddDate(0, 0, 10))
	if d, ok := back.DaysToClear(); !ok || d != 10 {
		t.Errorf("days = %d, %v after re-clearing; want 10", d, ok)
	}
}

// Old data has the flag and no stamp. Treating that as "cleared same day" would
// teach every account a window of nothing.
func TestLegacyClearedRowsReportUnknown(t *testing.T) {
	legacy := Transaction{Date: day1, Cleared: true}
	if _, ok := legacy.DaysToClear(); ok {
		t.Error("a cleared row with no stamp reported a clearing time")
	}
	if _, ok := (Transaction{Date: day1}).DaysToClear(); ok {
		t.Error("an uncleared row reported a clearing time")
	}
}

// Data that disagrees with itself is not evidence.
func TestClearingBeforeItHappenedIsUnknown(t *testing.T) {
	tx := Transaction{Date: day1}.MarkCleared(true, day1.AddDate(0, 0, -2))
	if _, ok := tx.DaysToClear(); ok {
		t.Error("a transaction that cleared before it happened reported a clearing time")
	}
}
