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

func ids(items []uistate.FeedItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}
