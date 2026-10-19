// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "testing"

func TestRemoveQueuedSyncMutationForMatchesFullScope(t *testing.T) {
	queue := []queuedSyncMutation{
		{UserID: "account-a", WorkspaceID: "shared", Hash: "same"},
		{UserID: "account-b", WorkspaceID: "shared", Hash: "same"},
		{UserID: "account-a", WorkspaceID: "other", Hash: "same"},
		{UserID: "account-a", WorkspaceID: "shared", Hash: "other"},
	}

	got := withoutQueuedSyncMutation(queue, "account-a", "shared", "same")
	if len(got) != 3 {
		t.Fatalf("remaining queue entries = %d, want 3", len(got))
	}
	for _, item := range got {
		if item.UserID == "account-a" && item.WorkspaceID == "shared" && item.Hash == "same" {
			t.Fatal("matching owner, workspace, and hash remained queued")
		}
	}
	if got[0].UserID != "account-b" || got[1].WorkspaceID != "other" || got[2].Hash != "other" {
		t.Fatalf("unrelated queue entries changed order or scope: %#v", got)
	}
}

func TestSyncChipFaceIncludesSVIdentityStates(t *testing.T) {
	cases := []struct {
		state string
		key   string
	}{
		{syncStateLockedDifferentPasscode, "sync.lockedDifferentPasscode"},
		{syncStateIdentityUnknown, "sync.identityUnknownLabel"},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			key, class, ok := syncChipFace(tc.state)
			if !ok || key != tc.key || class != "sync-chip sync-warn" {
				t.Fatalf("syncChipFace(%q) = (%q, %q, %v)", tc.state, key, class, ok)
			}
		})
	}
}
