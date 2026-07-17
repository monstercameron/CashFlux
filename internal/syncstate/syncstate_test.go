// SPDX-License-Identifier: MIT

package syncstate

import (
	"testing"
	"time"
)

func TestShouldApplyRemote(t *testing.T) {
	base := time.Date(2026, time.June, 18, 20, 30, 0, 0, time.UTC)
	tests := []struct {
		name          string
		local         time.Time
		hasLocalMeta  bool
		hasLocalData  bool
		remote        time.Time
		hasRemoteData bool
		want          bool
	}{
		{"newer remote", base, true, true, base.Add(time.Minute), true, true},
		{"older remote", base, true, true, base.Add(-time.Minute), true, false},
		{"equal remote", base, true, true, base, true, false},
		{"missing local metadata with local data", base, false, true, base.Add(time.Minute), true, false},
		{"fresh browser accepts remote", base, false, false, base.Add(time.Minute), true, true},
		{"missing remote data", base, true, true, base.Add(time.Minute), false, false},
		{"zero remote time", base, true, true, time.Time{}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldApplyRemote(tt.local, tt.hasLocalMeta, tt.hasLocalData, tt.remote, tt.hasRemoteData); got != tt.want {
				t.Fatalf("ShouldApplyRemote = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPendingQueueUpsertAndRemove(t *testing.T) {
	queue := []PendingMutation{
		{WorkspaceID: "w1", Hash: "old", UpdatedAt: "2026-06-19T10:00:00Z"},
		{WorkspaceID: "w2", Hash: "keep", UpdatedAt: "2026-06-19T10:01:00Z"},
	}

	queue = UpsertPending(queue, PendingMutation{WorkspaceID: "w1", Hash: "new", UpdatedAt: "2026-06-19T10:02:00Z"})
	if len(queue) != 2 || queue[0].WorkspaceID != "w1" || queue[0].Hash != "new" || queue[1].WorkspaceID != "w2" {
		t.Fatalf("upsert replacement queue = %+v", queue)
	}

	queue = UpsertPending(queue, PendingMutation{WorkspaceID: "w3", Hash: "third", UpdatedAt: "2026-06-19T10:03:00Z"})
	if len(queue) != 3 || queue[2].WorkspaceID != "w3" {
		t.Fatalf("upsert append queue = %+v", queue)
	}

	queue = RemovePending(queue, "w1", "wrong")
	if len(queue) != 3 {
		t.Fatalf("remove with mismatched hash changed queue = %+v", queue)
	}
	queue = RemovePending(queue, "w1", "new")
	if len(queue) != 2 || queue[0].WorkspaceID != "w2" || queue[1].WorkspaceID != "w3" {
		t.Fatalf("remove accepted queue = %+v", queue)
	}
}

func TestShouldResetBackoff(t *testing.T) {
	const healthy = 30 * time.Second
	tests := []struct {
		name         string
		received     bool
		connectedFor time.Duration
		want         bool
	}{
		{"delivered a message resets", true, time.Second, true},
		{"survived long enough resets", false, healthy, true},
		{"survived well past threshold resets", false, 2 * healthy, true},
		{"immediate error does NOT reset (thrash guard)", false, 50 * time.Millisecond, false},
		{"short-lived silent stream does NOT reset", false, 5 * time.Second, false},
		{"message beats a short lifetime", true, time.Millisecond, true},
	}
	for _, tc := range tests {
		if got := ShouldResetBackoff(tc.received, tc.connectedFor, healthy); got != tc.want {
			t.Errorf("%s: ShouldResetBackoff(%v, %v) = %v, want %v", tc.name, tc.received, tc.connectedFor, got, tc.want)
		}
	}
}
