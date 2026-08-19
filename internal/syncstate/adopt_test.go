// SPDX-License-Identifier: MIT

package syncstate

import "testing"

func TestAdoptTarget(t *testing.T) {
	two := []RemoteWorkspace{
		{ID: "ws_old", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: "ws_new", UpdatedAt: "2026-08-01T00:00:00Z"},
	}
	for _, tc := range []struct {
		name       string
		activeID   string
		hasSynced  bool
		hasData    bool
		remote     []RemoteWorkspace
		wantTarget string
	}{
		{"fresh browser opens the account's workspace", "default", false, false, two, "ws_new"},
		{"a device that has synced keeps what it has open", "default", true, false, two, ""},
		{"local records are never replaced", "default", false, true, two, ""},
		{"the account already knows this workspace", "ws_old", false, false, two, ""},
		{"an account that owns nothing has nothing to adopt", "default", false, false, nil, ""},
		{"deleted workspaces are not adoptable", "default", false, false,
			[]RemoteWorkspace{{ID: "ws_gone", UpdatedAt: "2026-08-01T00:00:00Z", Deleted: true}}, ""},
		{"no active workspace, no decision", "", false, false, two, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AdoptTarget(tc.activeID, tc.hasSynced, tc.hasData, tc.remote)
			if got != tc.wantTarget {
				t.Fatalf("AdoptTarget = %q, want %q", got, tc.wantTarget)
			}
		})
	}
}

// A deleted workspace must not shadow a live one just by sorting later.
func TestAdoptTargetSkipsDeletedNewer(t *testing.T) {
	got := AdoptTarget("default", false, false, []RemoteWorkspace{
		{ID: "ws_live", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: "ws_dead", UpdatedAt: "2026-09-09T00:00:00Z", Deleted: true},
	})
	if got != "ws_live" {
		t.Fatalf("AdoptTarget = %q, want ws_live", got)
	}
}
