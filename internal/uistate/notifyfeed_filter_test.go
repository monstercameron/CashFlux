// SPDX-License-Identifier: MIT

package uistate_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestVisibleFeed(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC).Unix()

	past := now - 3600   // 1 hour ago — snooze expired
	future := now + 3600 // 1 hour from now — still snoozed

	items := []uistate.FeedItem{
		{ID: "a", Title: "Normal item", SnoozedUntil: 0},
		{ID: "b", Title: "Snooze expired", SnoozedUntil: past},
		{ID: "c", Title: "Still snoozed", SnoozedUntil: future},
		{ID: "d", Title: "Snoozed until exactly now", SnoozedUntil: now},
		{ID: "e", Title: "Critical urgent", Severity: "critical"},
	}

	tests := []struct {
		name    string
		now     int64
		wantIDs []string
	}{
		{
			name:    "normal: snoozed-future hidden, rest visible",
			now:     now,
			wantIDs: []string{"a", "b", "d", "e"},
		},
		{
			name:    "all visible when now is very large",
			now:     now + 7200,
			wantIDs: []string{"a", "b", "c", "d", "e"},
		},
		{
			// now=0 means every item whose SnoozedUntil > 0 is "still snoozed":
			// b(past>0), c(future>0), d(now_unix>0) are all hidden; only a and e
			// (SnoozedUntil==0) survive.
			name:    "now=0 hides any item with a positive SnoozedUntil",
			now:     0,
			wantIDs: []string{"a", "e"},
		},
		{
			name:    "empty input → empty output",
			now:     now,
			wantIDs: []string{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			input := items
			if tc.name == "empty input → empty output" {
				input = nil
			}
			got := uistate.VisibleFeed(input, tc.now)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("VisibleFeed returned %d items, want %d: got IDs %v",
					len(got), len(tc.wantIDs), ids(got))
			}
			for i, want := range tc.wantIDs {
				if got[i].ID != want {
					t.Errorf("item[%d]: got ID %q, want %q", i, got[i].ID, want)
				}
			}
		})
	}
}

func TestNewSinceLastSeen(t *testing.T) {
	base := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC).Unix()

	items := []uistate.FeedItem{
		{ID: "old1", Title: "Old item 1", At: base - 7200}, // 2 h before base
		{ID: "old2", Title: "Old item 2", At: base - 3600}, // 1 h before base
		{ID: "exact", Title: "Exactly at lastSeen", At: base},
		{ID: "new1", Title: "New item 1", At: base + 60},   // 1 min after
		{ID: "new2", Title: "New item 2", At: base + 3600}, // 1 h after
	}

	tests := []struct {
		name     string
		lastSeen int64
		input    []uistate.FeedItem
		wantIDs  []string
	}{
		{
			name:     "newer items included, older and equal excluded",
			lastSeen: base,
			input:    items,
			wantIDs:  []string{"new1", "new2"},
		},
		{
			name:     "at==lastSeen boundary excluded (strictly greater required)",
			lastSeen: base,
			input:    []uistate.FeedItem{{ID: "exact", At: base}},
			wantIDs:  nil,
		},
		{
			name:     "all items older than lastSeen — none returned",
			lastSeen: base + 7200,
			input:    items,
			wantIDs:  nil,
		},
		{
			name:     "lastSeen=0 — everything after epoch is new",
			lastSeen: 0,
			input:    items,
			wantIDs:  []string{"old1", "old2", "exact", "new1", "new2"},
		},
		{
			name:     "empty input slice returns nil",
			lastSeen: base,
			input:    nil,
			wantIDs:  nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := uistate.NewSinceLastSeen(tc.input, tc.lastSeen)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("NewSinceLastSeen returned %d items, want %d: got IDs %v",
					len(got), len(tc.wantIDs), ids(got))
			}
			for i, want := range tc.wantIDs {
				if got[i].ID != want {
					t.Errorf("item[%d]: got ID %q, want %q", i, got[i].ID, want)
				}
			}
		})
	}
}

func TestPartitionTriage(t *testing.T) {
	items := []uistate.FeedItem{
		{ID: "crit", Severity: "critical"},
		{ID: "info1", Severity: "info"},
		{ID: "warn", Severity: "warning"},
		{ID: "legacy", Severity: ""}, // empty severity == info
		{ID: "info2", Severity: "info"},
	}
	needs, watching := uistate.PartitionTriage(items)
	if got := ids(needs); len(got) != 2 || got[0] != "crit" || got[1] != "warn" {
		t.Errorf("needs bucket = %v, want [crit warn]", got)
	}
	if got := ids(watching); len(got) != 3 || got[0] != "info1" || got[1] != "legacy" || got[2] != "info2" {
		t.Errorf("watching bucket = %v, want [info1 legacy info2]", got)
	}

	// Empty input yields two empty (non-nil) slices.
	n, w := uistate.PartitionTriage(nil)
	if len(n) != 0 || len(w) != 0 {
		t.Errorf("empty input: got needs=%v watching=%v, want both empty", ids(n), ids(w))
	}
}

func TestDedupeFeed(t *testing.T) {
	items := []uistate.FeedItem{
		{ID: "a1", Title: "Backup ran", Body: "Your data was backed up."},
		{ID: "a2", Title: "Backup ran", Body: "Your data was backed up."}, // exact dup of a1
		{ID: "b", Title: "Bill due", Body: "Rent is due tomorrow."},
		{ID: "c", Title: "Bill due", Body: "Gym is due tomorrow."},        // same title, different body — kept
		{ID: "a3", Title: "Backup ran", Body: "Your data was backed up."}, // another dup
	}
	got := uistate.DedupeFeed(items)
	want := []string{"a1", "b", "c"}
	if g := ids(got); len(g) != len(want) {
		t.Fatalf("DedupeFeed returned %v, want IDs %v", g, want)
	}
	for i, w := range want {
		if got[i].ID != w {
			t.Errorf("item[%d]: got %q, want %q", i, got[i].ID, w)
		}
	}
	// Input slice must be untouched.
	if len(items) != 5 {
		t.Errorf("DedupeFeed mutated the input slice length: %d", len(items))
	}
}

func ids(items []uistate.FeedItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func TestOverdueDays(t *testing.T) {
	const day = int64(86400)
	due := int64(1_700_000_000) - (int64(1_700_000_000) % day) // a UTC midnight
	tests := []struct {
		name  string
		dueAt int64
		now   int64
		want  int
	}{
		{"no due date", 0, due, 0},
		{"due today", due, due + day/2, 0},
		{"due tomorrow", due + day, due, 0},
		{"one day overdue", due, due + day + 3600, 1},
		{"ten days overdue", due, due + 10*day, 10},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := uistate.OverdueDays(tc.dueAt, tc.now); got != tc.want {
				t.Errorf("OverdueDays(%d, %d) = %d, want %d", tc.dueAt, tc.now, got, tc.want)
			}
		})
	}
}

func TestDueToday(t *testing.T) {
	const day = int64(86400)
	due := int64(1_700_000_000) - (int64(1_700_000_000) % day) // a UTC midnight
	tests := []struct {
		name  string
		dueAt int64
		now   int64
		want  bool
	}{
		{"no due date", 0, due, false},
		{"due today at midnight", due, due, true},
		{"due today midday", due, due + day/2, true},
		{"due today last second", due, due + day - 1, true},
		{"due tomorrow", due + day, due + day/2, false},
		{"due yesterday (overdue)", due, due + day, false},
		{"due next week", due + 7*day, due, false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := uistate.DueToday(tc.dueAt, tc.now); got != tc.want {
				t.Errorf("DueToday(%d, %d) = %v, want %v", tc.dueAt, tc.now, got, tc.want)
			}
			// Invariant: DueToday and OverdueDays>0 are mutually exclusive.
			if got := uistate.DueToday(tc.dueAt, tc.now); got && uistate.OverdueDays(tc.dueAt, tc.now) > 0 {
				t.Errorf("DueToday and OverdueDays>0 both true for (%d, %d)", tc.dueAt, tc.now)
			}
		})
	}
}
