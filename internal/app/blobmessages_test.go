// SPDX-License-Identifier: MIT

package app

import "testing"

// TestBuildUploadBlobHeaderIncludesWorkspaceID guards against the exact
// regression a post-commit security scan caught: uploadBackendArtifactBlob
// received a workspaceID parameter but never threaded it into the header it
// actually sent over the wire, so the server's authorization check (which
// fails closed on an empty workspace id) rejected every real upload. This
// test would have failed against that code.
func TestBuildUploadBlobHeaderIncludesWorkspaceID(t *testing.T) {
	h := buildUploadBlobHeader("ws-123", "deadbeef", 4096, "application/octet-stream")
	if h.WorkspaceID != "ws-123" {
		t.Fatalf("WorkspaceID = %q, want %q — a blob upload with a missing workspace id is hard-rejected by the server (fail-closed), silently breaking every artifact upload", h.WorkspaceID, "ws-123")
	}
	if h.Hash != "deadbeef" || h.DeclaredSizeBytes != 4096 || h.Mime != "application/octet-stream" {
		t.Fatalf("unexpected header: %+v", h)
	}
}

// TestBuildUploadBlobHeaderEmptyWorkspaceIDIsVisible documents that an empty
// workspaceID is passed through as-is (not silently substituted) — the
// server is the single source of truth for rejecting it, so this function
// must never paper over a genuinely missing id with a fallback value.
func TestBuildUploadBlobHeaderEmptyWorkspaceIDIsVisible(t *testing.T) {
	h := buildUploadBlobHeader("", "deadbeef", 1, "text/plain")
	if h.WorkspaceID != "" {
		t.Fatalf("WorkspaceID = %q, want empty string passed through unchanged", h.WorkspaceID)
	}
}

// TestBuildDownloadBlobRequestIncludesWorkspaceID guards against the sibling
// regression: downloadBackendArtifactBlob explicitly discarded workspaceID
// (`_ = workspaceID`) instead of sending it, which the server also requires
// to scope tenant access to a download.
func TestBuildDownloadBlobRequestIncludesWorkspaceID(t *testing.T) {
	r := buildDownloadBlobRequest("ws-456", "cafebabe")
	if r.WorkspaceID != "ws-456" {
		t.Fatalf("WorkspaceID = %q, want %q — a blob download with a missing workspace id is hard-rejected by the server (fail-closed), and was previously silently discarded client-side", r.WorkspaceID, "ws-456")
	}
	if r.Hash != "cafebabe" {
		t.Fatalf("Hash = %q, want %q", r.Hash, "cafebabe")
	}
}
