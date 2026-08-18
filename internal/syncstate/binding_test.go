// SPDX-License-Identifier: MIT

package syncstate

import (
	"reflect"
	"testing"
)

func TestBindingOwnership(t *testing.T) {
	tests := []struct {
		name    string
		binding Binding
		user    string
		want    bool
	}{
		{"same user owns it", Binding{UserID: "u1", WorkspaceID: "w"}, "u1", true},
		{"another user does not", Binding{UserID: "u1", WorkspaceID: "w"}, "u2", false},
		{"unbound state is adoptable", Binding{WorkspaceID: "w"}, "u2", true},
		{"nobody signed in owns nothing", Binding{WorkspaceID: "w"}, "", false},
		{"nobody signed in owns nothing, bound", Binding{UserID: "u1", WorkspaceID: "w"}, "", false},
		{"whitespace is not an identity", Binding{UserID: "u1", WorkspaceID: "w"}, "  ", false},
	}
	for _, tt := range tests {
		if got := tt.binding.OwnedBy(tt.user); got != tt.want {
			t.Errorf("%s: OwnedBy(%q) = %v, want %v", tt.name, tt.user, got, tt.want)
		}
	}
}

func TestIsWorkspaceNotFound(t *testing.T) {
	for _, reason := range []string{
		"workspace not found",
		"rpc error: code = NotFound desc = workspace not found",
		"WORKSPACE NOT FOUND",
	} {
		if !IsWorkspaceNotFound(reason) {
			t.Errorf("IsWorkspaceNotFound(%q) = false", reason)
		}
	}
	for _, reason := range []string{"", "unavailable", "context deadline exceeded", "not found"} {
		if IsWorkspaceNotFound(reason) {
			t.Errorf("IsWorkspaceNotFound(%q) = true", reason)
		}
	}
}

func TestDecideUpload(t *testing.T) {
	mine := PendingMutation{UserID: "u1", WorkspaceID: "w1", Hash: "h"}
	theirs := PendingMutation{UserID: "u2", WorkspaceID: "w1", Hash: "h"}
	legacy := PendingMutation{WorkspaceID: "w1", Hash: "h"}

	tests := []struct {
		name   string
		user   string
		next   PendingMutation
		reason string
		want   UploadDecision
	}{
		{"nothing queued", "u1", PendingMutation{}, "", UploadNothing},
		// An unknown identity must NOT stop a working client. Blocking here
		// meant every deployment whose server does not answer an identity
		// lookup — token auth, older servers, a first run before the lookup
		// lands — stopped syncing entirely. The server is the authority.
		{"unknown identity still uploads", "", mine, "", UploadProceed},
		{"unknown identity stops once the server refuses", "", mine, "workspace not found", UploadSignIn},
		{"own workspace uploads", "u1", mine, "", UploadProceed},
		{"legacy unbound work is adopted", "u1", legacy, "", UploadProceed},
		{"another identity's work needs a decision", "u1", theirs, "", UploadRebind},
		{"workspace not found is terminal, not transient", "u1", mine, "rpc error: workspace not found", UploadRebind},
		{"an unrelated failure still retries", "u1", mine, "unavailable", UploadProceed},
	}
	for _, tt := range tests {
		if got := DecideUpload(tt.user, tt.next, tt.reason); got != tt.want {
			t.Errorf("%s: DecideUpload = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestPendingForSplitsWithoutDiscarding(t *testing.T) {
	queue := []PendingMutation{
		{UserID: "u1", WorkspaceID: "w1", Hash: "a"},
		{UserID: "u2", WorkspaceID: "w2", Hash: "b"},
		{WorkspaceID: "w3", Hash: "c"},
	}
	mine, foreign := PendingFor(queue, "u1")
	if len(mine) != 2 || mine[0].Hash != "a" || mine[1].Hash != "c" {
		t.Fatalf("mine = %+v", mine)
	}
	if len(foreign) != 1 || foreign[0].Hash != "b" {
		t.Fatalf("foreign = %+v", foreign)
	}
	// Nothing is lost: foreign work is a decision the user has not made yet.
	if len(mine)+len(foreign) != len(queue) {
		t.Fatal("PendingFor dropped an entry")
	}
}

func TestAdoptClaimsOnlyUnboundWork(t *testing.T) {
	queue := []PendingMutation{
		{WorkspaceID: "w1", Hash: "a"},
		{UserID: "u2", WorkspaceID: "w2", Hash: "b"},
	}
	got := Adopt(queue, "u1")
	if got[0].UserID != "u1" {
		t.Errorf("unbound entry was not adopted: %+v", got[0])
	}
	if got[1].UserID != "u2" {
		t.Errorf("another user's work was silently transferred: %+v", got[1])
	}
	// Signed out, nothing is claimed.
	if out := Adopt(queue, ""); !reflect.DeepEqual(out, queue) {
		t.Errorf("Adopt with no identity changed the queue: %+v", out)
	}
}

func TestUpsertPendingKeysOnThePairNotTheWorkspace(t *testing.T) {
	// Two identities holding queued work for the same workspace id is exactly
	// the state a device-account change produces. Collapsing on workspace alone
	// would destroy one of them.
	queue := []PendingMutation{{UserID: "u1", WorkspaceID: "w1", Hash: "a"}}
	queue = UpsertPending(queue, PendingMutation{UserID: "u2", WorkspaceID: "w1", Hash: "b"})
	if len(queue) != 2 {
		t.Fatalf("a second identity's work replaced the first: %+v", queue)
	}
	queue = UpsertPending(queue, PendingMutation{UserID: "u1", WorkspaceID: "w1", Hash: "c"})
	if len(queue) != 2 || queue[0].Hash != "c" || queue[1].Hash != "b" {
		t.Fatalf("same-binding upsert did not replace in place: %+v", queue)
	}
}

func TestRemovePendingDoesNotClearAnotherIdentitysWork(t *testing.T) {
	queue := []PendingMutation{
		{UserID: "u1", WorkspaceID: "w1", Hash: "a"},
		{UserID: "u2", WorkspaceID: "w1", Hash: "b"},
	}
	got := RemovePending(queue, Binding{UserID: "u1", WorkspaceID: "w1"}, "a")
	if len(got) != 1 || got[0].UserID != "u2" {
		t.Fatalf("remove = %+v", got)
	}
	// A mismatched hash is a no-op, as before.
	if out := RemovePending(got, Binding{UserID: "u2", WorkspaceID: "w1"}, "wrong"); len(out) != 1 {
		t.Fatalf("mismatched hash removed an entry: %+v", out)
	}
	// An empty hash clears whatever that binding has queued.
	if out := RemovePending(got, Binding{UserID: "u2", WorkspaceID: "w1"}, ""); len(out) != 0 {
		t.Fatalf("empty-hash remove = %+v", out)
	}
}

func TestRebindMovesOnlyTheChosenBinding(t *testing.T) {
	queue := []PendingMutation{
		{UserID: "u1", WorkspaceID: "w1", Hash: "a"},
		{UserID: "u1", WorkspaceID: "w2", Hash: "b"},
	}
	got := Rebind(queue, Binding{UserID: "u1", WorkspaceID: "w1"}, Binding{UserID: "u2", WorkspaceID: "w9"})
	if len(got) != 2 {
		t.Fatalf("rebind changed the queue length: %+v", got)
	}
	var moved, untouched PendingMutation
	for _, m := range got {
		switch m.Hash {
		case "a":
			moved = m
		case "b":
			untouched = m
		}
	}
	if moved.UserID != "u2" || moved.WorkspaceID != "w9" {
		t.Errorf("rebound entry = %+v", moved)
	}
	if untouched.UserID != "u1" || untouched.WorkspaceID != "w2" {
		t.Errorf("an unrelated binding was swept up: %+v", untouched)
	}
}

func TestRebindOntoAnOccupiedBindingCollapsesToOne(t *testing.T) {
	queue := []PendingMutation{
		{UserID: "u1", WorkspaceID: "w1", Hash: "incoming"},
		{UserID: "u2", WorkspaceID: "w9", Hash: "already-there"},
	}
	got := Rebind(queue, Binding{UserID: "u1", WorkspaceID: "w1"}, Binding{UserID: "u2", WorkspaceID: "w9"})
	if len(got) != 1 {
		t.Fatalf("expected one entry per binding, got %+v", got)
	}
	if got[0].Hash != "incoming" {
		t.Errorf("kept %q, want the snapshot the user just chose", got[0].Hash)
	}
}

func TestRebindIgnoresIncompleteBindings(t *testing.T) {
	queue := []PendingMutation{{UserID: "u1", WorkspaceID: "w1", Hash: "a"}}
	for _, tc := range []struct{ from, to Binding }{
		{Binding{UserID: "u1"}, Binding{UserID: "u2", WorkspaceID: "w9"}},
		{Binding{UserID: "u1", WorkspaceID: "w1"}, Binding{UserID: "u2"}},
	} {
		if got := Rebind(queue, tc.from, tc.to); !reflect.DeepEqual(got, queue) {
			t.Errorf("Rebind(%+v → %+v) changed the queue: %+v", tc.from, tc.to, got)
		}
	}
}
