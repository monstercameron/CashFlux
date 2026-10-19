// SPDX-License-Identifier: MIT

package syncstate

import "testing"

// SV0.1: a queued push must carry the scope it was created under — workspace,
// owning account and the server version it was based on — and that scope, not
// whatever happens to be active when the flush runs, decides whether and how
// it may be sent.

func TestCaptureScopeStampsWorkspaceOwnerAndBaseVersion(t *testing.T) {
	scope := CaptureScope(" ws-1 ", " user-a ", 42)
	if scope.WorkspaceID != "ws-1" || scope.OwnerUserID != "user-a" || scope.BaseVersion != 42 {
		t.Fatalf("CaptureScope = %+v, want {ws-1 user-a 42}", scope)
	}
	entry := scope.Entry("hash-1")
	if entry.WorkspaceID != "ws-1" || entry.OwnerUserID != "user-a" || entry.BaseVersion != 42 || entry.Hash != "hash-1" {
		t.Fatalf("Entry = %+v, want scope fields plus hash-1", entry)
	}
}

func TestPushableEntryScopeRules(t *testing.T) {
	tests := []struct {
		name     string
		entry    ScopedEntry
		signedIn string
		want     bool
	}{
		{"own entry", ScopedEntry{WorkspaceID: "ws-1", OwnerUserID: "user-a", BaseVersion: 3}, "user-a", true},
		{"another owner's entry is never pushable", ScopedEntry{WorkspaceID: "ws-1", OwnerUserID: "user-a", BaseVersion: 3}, "user-b", false},
		{"empty signed-in user pushes nothing (fail closed)", ScopedEntry{WorkspaceID: "ws-1", OwnerUserID: "user-a", BaseVersion: 3}, "", false},
		{"empty signed-in pushes not even unowned legacy work", ScopedEntry{WorkspaceID: "ws-1"}, "", false},
		{"unowned legacy entry is adoptable once identity is known", ScopedEntry{WorkspaceID: "ws-1"}, "user-a", true},
		{"no workspace cannot be pushed", ScopedEntry{OwnerUserID: "user-a"}, "user-a", false},
		{"whitespace identity is empty identity", ScopedEntry{WorkspaceID: "ws-1", OwnerUserID: "user-a"}, "  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PushableEntry(tt.entry, tt.signedIn); got != tt.want {
				t.Fatalf("PushableEntry(%+v, %q) = %v, want %v", tt.entry, tt.signedIn, got, tt.want)
			}
		})
	}
}

func TestSplitPushableParksForeignWork(t *testing.T) {
	queue := []ScopedEntry{
		{WorkspaceID: "ws-1", OwnerUserID: "user-a", Hash: "a1"},
		{WorkspaceID: "ws-1", OwnerUserID: "user-b", Hash: "b1"},
		{WorkspaceID: "ws-2", OwnerUserID: "user-b", Hash: "b2"},
		{WorkspaceID: "ws-3", OwnerUserID: "user-a", Hash: "a2"},
	}
	pushable, parked := SplitPushable(queue, "user-a")
	if len(pushable) != 2 || pushable[0].Hash != "a1" || pushable[1].Hash != "a2" {
		t.Fatalf("pushable = %+v, want a1,a2 in order", pushable)
	}
	if len(parked) != 2 || parked[0].Hash != "b1" || parked[1].Hash != "b2" {
		t.Fatalf("parked = %+v, want b1,b2 in order (kept, not dropped)", parked)
	}

	// Nobody signed in: everything parks, nothing is sent, nothing is lost.
	pushable, parked = SplitPushable(queue, "")
	if len(pushable) != 0 || len(parked) != len(queue) {
		t.Fatalf("empty identity: pushable=%+v parked=%+v, want none pushable and all parked", pushable, parked)
	}
}

func TestParkedOwnersNamesDistinctForeignAccounts(t *testing.T) {
	parked := []ScopedEntry{
		{WorkspaceID: "ws-1", OwnerUserID: "user-b"},
		{WorkspaceID: "ws-2", OwnerUserID: "user-b"},
		{WorkspaceID: "ws-3", OwnerUserID: ""},
		{WorkspaceID: "ws-4", OwnerUserID: "user-c"},
	}
	got := ParkedOwners(parked)
	want := []string{"user-b", "user-c"}
	if len(got) != len(want) {
		t.Fatalf("ParkedOwners = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParkedOwners = %v, want %v", got, want)
		}
	}
	if owners := ParkedOwners(nil); len(owners) != 0 {
		t.Fatalf("ParkedOwners(nil) = %v, want empty", owners)
	}
}

func TestCheckPullScope(t *testing.T) {
	tests := []struct {
		name        string
		pulledWS    string
		activeWS    string
		requestedBy string
		signedIn    string
		wantOK      bool
		wantReason  string
	}{
		{"same workspace same account applies", "ws-1", "ws-1", "user-a", "user-a", true, ""},
		{"pull for a no-longer-active workspace is rejected", "ws-1", "ws-2", "user-a", "user-a", false, PullRejectWorkspaceChanged},
		{"account changed while pull was in flight", "ws-1", "ws-1", "user-a", "user-b", false, PullRejectAccountChanged},
		{"signed out while pull was in flight", "ws-1", "ws-1", "user-a", "", false, PullRejectAccountChanged},
		{"identity resolved while pull was in flight", "ws-1", "ws-1", "", "user-a", false, PullRejectAccountChanged},
		{"unknown current identity cannot establish owner", "ws-1", "ws-1", "", "", false, PullRejectUnknownAccount},
		{"pulled snapshot with no workspace id is rejected", "", "ws-1", "user-a", "user-a", false, PullRejectWorkspaceChanged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := CheckPullScope(tt.pulledWS, tt.activeWS, tt.requestedBy, tt.signedIn)
			if ok != tt.wantOK || reason != tt.wantReason {
				t.Fatalf("CheckPullScope = (%v, %q), want (%v, %q)", ok, reason, tt.wantOK, tt.wantReason)
			}
		})
	}
}
