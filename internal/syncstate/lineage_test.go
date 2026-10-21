// SPDX-License-Identifier: MIT

package syncstate

import "testing"

func TestCanRebaseQueuedSuccessor(t *testing.T) {
	accepted := UploadLineage{UserID: "alice", WorkspaceID: "home", PageID: "tab-a", Hash: "first", BaseVersion: 4}
	pending := UploadLineage{UserID: "alice", WorkspaceID: "home", PageID: "tab-a", Hash: "latest", AncestorHash: "first", BaseVersion: 4}
	tests := []struct {
		name    string
		change  func(*UploadLineage)
		version int64
		want    bool
	}{
		{"same page descendant", nil, 5, true},
		{"another account", func(p *UploadLineage) { p.UserID = "bob" }, 5, false},
		{"another workspace", func(p *UploadLineage) { p.WorkspaceID = "work" }, 5, false},
		{"another tab", func(p *UploadLineage) { p.PageID = "tab-b" }, 5, false},
		{"unrelated edit", func(p *UploadLineage) { p.AncestorHash = "other" }, 5, false},
		{"already rebased", func(p *UploadLineage) { p.BaseVersion = 5 }, 5, false},
		{"unchanged snapshot", func(p *UploadLineage) { p.Hash = "first" }, 5, false},
		{"server did not advance", nil, 4, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := pending
			if tc.change != nil {
				tc.change(&candidate)
			}
			if got := CanRebaseQueuedSuccessor(accepted, candidate, tc.version); got != tc.want {
				t.Fatalf("CanRebaseQueuedSuccessor = %v, want %v", got, tc.want)
			}
		})
	}
}
