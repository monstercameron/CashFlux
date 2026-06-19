package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepositoryUserAndWorkspaceFlow(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.June, 18, 16, 12, 0, 0, time.UTC)
	for _, u := range []User{
		{ID: "u1", Provider: "github", Subject: "alice", Email: "alice@example.com", CreatedAt: now},
		{ID: "u2", Provider: "github", Subject: "bob", Email: "bob@example.com", CreatedAt: now},
	} {
		if err := s.UpsertUser(u); err != nil {
			t.Fatalf("UpsertUser %s: %v", u.ID, err)
		}
	}
	for _, w := range []Workspace{
		{ID: "w2", UserID: "u1", Name: "Budget", Sort: 2, Version: 1, UpdatedAt: now, DeviceID: "laptop"},
		{ID: "w1", UserID: "u1", Name: "Home", Sort: 1, Version: 3, UpdatedAt: now.Add(time.Minute), DeviceID: "phone"},
		{ID: "w3", UserID: "u2", Name: "Other user", Sort: 1, Version: 1, UpdatedAt: now, DeviceID: "tablet"},
	} {
		if err := s.PutWorkspace(w); err != nil {
			t.Fatalf("PutWorkspace %s: %v", w.ID, err)
		}
	}

	got, err := s.ListWorkspaces("u1", false)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(got) != 2 || got[0].ID != "w1" || got[1].ID != "w2" {
		t.Fatalf("u1 workspaces ordered/scoped = %+v", got)
	}
	w, ok, err := s.GetWorkspace("u1", "w1")
	if err != nil || !ok {
		t.Fatalf("GetWorkspace = %+v/%v/%v", w, ok, err)
	}
	if w.Version != 3 || !w.UpdatedAt.Equal(now.Add(time.Minute)) || w.DeviceID != "phone" {
		t.Fatalf("workspace metadata = %+v", w)
	}
	if _, ok, err := s.GetWorkspace("u2", "w1"); err != nil || ok {
		t.Fatalf("cross-user get = ok %v err %v, want not found", ok, err)
	}
}

func TestStoreAuditEventsAppendHashChain(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.June, 19, 2, 10, 0, 0, time.UTC)
	first, err := s.AppendAuditEvent(AuditEvent{
		Timestamp:  now,
		ActorID:    "u1",
		Action:     "auth.login",
		TargetType: "user",
		TargetID:   "u1",
		IP:         "198.51.100.10",
		RequestID:  "req-1",
	})
	if err != nil {
		t.Fatalf("AppendAuditEvent first: %v", err)
	}
	second, err := s.AppendAuditEvent(AuditEvent{
		Timestamp:  now.Add(time.Second),
		ActorID:    "u1",
		Action:     "workspace.delete",
		TargetType: "workspace",
		TargetID:   "w1",
		RequestID:  "req-2",
	})
	if err != nil {
		t.Fatalf("AppendAuditEvent second: %v", err)
	}
	if first.ID == 0 || second.ID <= first.ID {
		t.Fatalf("audit ids not increasing: first=%d second=%d", first.ID, second.ID)
	}
	if first.Hash == "" || second.Hash == "" || second.PreviousHash != first.Hash {
		t.Fatalf("audit hash chain first=%+v second=%+v", first, second)
	}
	events, err := s.ListAuditEvents(first.ID, 10)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(events) != 1 || events[0].ID != second.ID || events[0].Action != "workspace.delete" {
		t.Fatalf("events after first = %+v", events)
	}
}

func TestStoreRecordsDBMetrics(t *testing.T) {
	s := openTestStore(t)
	metrics := NewMetrics()
	s.SetMetrics(metrics)
	now := time.Date(2026, time.June, 19, 0, 40, 0, 0, time.UTC)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	if _, ok, err := s.GetWorkspace("u1", "w1"); err != nil || !ok {
		t.Fatalf("GetWorkspace = %v/%v", ok, err)
	}
	if _, err := s.ListWorkspaces("u1", false); err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}

	var out strings.Builder
	metrics.WritePrometheus(&out)
	for _, want := range []string{
		`cashflux_db_queries_total{operation="GetWorkspace"} 1`,
		`cashflux_db_queries_total{operation="ListWorkspaces"} 1`,
		`cashflux_db_queries_total{operation="PutWorkspace"} 1`,
		`cashflux_db_queries_total{operation="UpsertUser"} 1`,
		`cashflux_db_query_duration_seconds_sum{operation="GetWorkspace"}`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing db metric %q in:\n%s", want, out.String())
		}
	}
}

func TestRepositoryValidationAndSoftDelete(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.June, 18, 16, 20, 0, 0, time.UTC)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", Email: "alice@example.com", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	cases := []struct {
		name string
		run  func() error
	}{
		{name: "missing user id", run: func() error { return s.UpsertUser(User{Provider: "github", Subject: "x"}) }},
		{name: "missing workspace name", run: func() error { return s.PutWorkspace(Workspace{ID: "w1", UserID: "u1"}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	if err := s.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	deleted, err := s.SoftDeleteWorkspace("u1", "w1", now.Add(time.Hour), "phone")
	if err != nil || !deleted {
		t.Fatalf("SoftDeleteWorkspace = %v/%v, want deleted", deleted, err)
	}
	active, err := s.ListWorkspaces("u1", false)
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active workspaces after delete = %+v", active)
	}
	all, err := s.ListWorkspaces("u1", true)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 1 || !all[0].Deleted || all[0].DeviceID != "phone" {
		t.Fatalf("deleted workspace = %+v", all)
	}
}

func TestSnapshotStoreCurrentHistoryAndSizeLimit(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.June, 18, 16, 30, 0, 0, time.UTC)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	for i, payload := range [][]byte{[]byte("v1"), []byte("v2"), []byte("v3")} {
		if err := s.PutSnapshot(Snapshot{
			WorkspaceID: "w1",
			Dataset:     payload,
			Version:     int64(i + 1),
			UpdatedAt:   now.Add(time.Duration(i) * time.Minute),
		}, 16, 2); err != nil {
			t.Fatalf("PutSnapshot %d: %v", i+1, err)
		}
	}
	current, ok, err := s.GetSnapshot("w1")
	if err != nil || !ok {
		t.Fatalf("GetSnapshot = %+v/%v/%v", current, ok, err)
	}
	if current.Version != 3 || string(current.Dataset) != "v3" {
		t.Fatalf("current snapshot = %+v/%q, want v3", current, current.Dataset)
	}
	history, err := s.SnapshotHistory("w1", 0)
	if err != nil {
		t.Fatalf("SnapshotHistory: %v", err)
	}
	if len(history) != 2 || history[0].Version != 2 || string(history[0].Dataset) != "v2" || history[1].Version != 1 {
		t.Fatalf("history = %+v, want versions 2,1", history)
	}
	if err := s.PutSnapshot(Snapshot{WorkspaceID: "w1", Dataset: []byte("too-large"), Version: 4, UpdatedAt: now}, 4, 2); err == nil {
		t.Fatal("oversized snapshot accepted")
	}
}

func TestSnapshotStoreCanDropHistory(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.June, 18, 16, 40, 0, 0, time.UTC)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	if err := s.PutSnapshot(Snapshot{WorkspaceID: "w1", Dataset: []byte("v1"), Version: 1, UpdatedAt: now}, 16, 1); err != nil {
		t.Fatalf("PutSnapshot v1: %v", err)
	}
	if err := s.PutSnapshot(Snapshot{WorkspaceID: "w1", Dataset: []byte("v2"), Version: 2, UpdatedAt: now.Add(time.Minute)}, 16, 0); err != nil {
		t.Fatalf("PutSnapshot v2: %v", err)
	}
	history, err := s.SnapshotHistory("w1", 0)
	if err != nil {
		t.Fatalf("SnapshotHistory: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("history with limit 0 = %+v, want empty", history)
	}
}

func TestBlobStoreContentAddressingLinksAndGC(t *testing.T) {
	s := openTestStore(t)
	root := filepath.Join(t.TempDir(), "blobs")
	now := time.Date(2026, time.June, 18, 16, 50, 0, 0, time.UTC)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}

	data := []byte("receipt bytes")
	blob, err := s.PutBlob(root, data, "image/png", "receipt.png", 1024)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	wantHash := sha256.Sum256(data)
	if blob.Hash != hex.EncodeToString(wantHash[:]) || blob.Size != int64(len(data)) || blob.Mime != "image/png" {
		t.Fatalf("blob metadata = %+v", blob)
	}
	read, err := s.ReadBlob(root, blob.Hash)
	if err != nil {
		t.Fatalf("ReadBlob: %v", err)
	}
	if string(read) != string(data) {
		t.Fatalf("read blob = %q, want %q", read, data)
	}
	if err := s.LinkWorkspaceBlob("w1", blob.Hash); err != nil {
		t.Fatalf("LinkWorkspaceBlob: %v", err)
	}
	linkedToUser, err := s.UserWorkspaceBlob("u1", "w1", blob.Hash)
	if err != nil {
		t.Fatalf("UserWorkspaceBlob own: %v", err)
	}
	if !linkedToUser {
		t.Fatal("UserWorkspaceBlob own = false, want true")
	}
	linkedToOther, err := s.UserWorkspaceBlob("u2", "w1", blob.Hash)
	if err != nil {
		t.Fatalf("UserWorkspaceBlob other user: %v", err)
	}
	if linkedToOther {
		t.Fatal("UserWorkspaceBlob other user = true, want false")
	}
	linked, err := s.WorkspaceBlobs("w1")
	if err != nil {
		t.Fatalf("WorkspaceBlobs: %v", err)
	}
	if len(linked) != 1 || linked[0].Hash != blob.Hash {
		t.Fatalf("linked blobs = %+v", linked)
	}
	deleted, err := s.SweepUnreferencedBlobs(root)
	if err != nil {
		t.Fatalf("Sweep linked: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted linked blobs = %d, want 0", deleted)
	}
	if _, err := s.db.Exec(`DELETE FROM workspace_blobs WHERE workspace_id = ? AND hash = ?`, "w1", blob.Hash); err != nil {
		t.Fatalf("unlink blob: %v", err)
	}
	deleted, err = s.SweepUnreferencedBlobs(root)
	if err != nil {
		t.Fatalf("Sweep unlinked: %v", err)
	}
	metrics := NewMetrics()
	metrics.ObserveBlobGC(deleted)
	if deleted != 1 {
		t.Fatalf("deleted unlinked blobs = %d, want 1", deleted)
	}
	var metricsOut strings.Builder
	metrics.WritePrometheus(&metricsOut)
	for _, want := range []string{
		"cashflux_blob_gc_sweeps_total 1",
		"cashflux_blob_gc_deleted_total 1",
	} {
		if !strings.Contains(metricsOut.String(), want) {
			t.Fatalf("blob gc metric missing %q in %q", want, metricsOut.String())
		}
	}
	if _, ok, err := s.GetBlob(blob.Hash); err != nil || ok {
		t.Fatalf("blob metadata after sweep = ok %v err %v, want missing", ok, err)
	}
}

func TestBlobStoreRejectsOversizedAndHashMismatch(t *testing.T) {
	s := openTestStore(t)
	root := filepath.Join(t.TempDir(), "blobs")
	blob, err := s.PutBlob(root, []byte("abc"), "text/plain", "a.txt", 16)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if _, err := s.PutBlob(root, []byte("too big"), "text/plain", "big.txt", 2); err == nil {
		t.Fatal("oversized blob accepted")
	}
	path, err := blobPath(root, blob.Hash)
	if err != nil {
		t.Fatalf("blobPath: %v", err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper blob: %v", err)
	}
	if _, err := s.ReadBlob(root, blob.Hash); err == nil {
		t.Fatal("hash-mismatched blob read accepted")
	}
}

func TestBlobStoreHonorsCanceledContext(t *testing.T) {
	s := openTestStore(t)
	root := filepath.Join(t.TempDir(), "blobs")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.PutBlobContext(ctx, root, []byte("abc"), "text/plain", "a.txt", 16); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("PutBlobContext canceled err = %v", err)
	}

	blob, err := s.PutBlob(root, []byte("abc"), "text/plain", "a.txt", 16)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if _, err := s.ReadBlobContext(ctx, root, blob.Hash); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("ReadBlobContext canceled err = %v", err)
	}
}

func TestBlobPathRejectsTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "blobs")
	for _, hash := range []string{
		"",
		"abc",
		"../outside",
		strings.Repeat("g", sha256.Size*2),
		strings.Repeat("a", sha256.Size*2) + "../outside",
	} {
		if _, err := blobPath(root, hash); err == nil {
			t.Fatalf("blobPath accepted %q", hash)
		}
	}
	hash := strings.Repeat("a", sha256.Size*2)
	path, err := blobPath(root, hash)
	if err != nil {
		t.Fatalf("valid blobPath: %v", err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("blob path rel: %v", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("blob path escaped root: %s", path)
	}
}

func TestAIKeyEncryptDecryptAndRotate(t *testing.T) {
	s := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	now := time.Date(2026, time.June, 18, 17, 0, 0, 0, time.UTC)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.PutAIKey("u1", "openai", "sk-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	var raw string
	if err := s.db.QueryRow(`SELECT CAST(ciphertext AS TEXT) FROM ai_keys WHERE user_id = ? AND provider = ?`, "u1", "openai").Scan(&raw); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if raw == "sk-secret" {
		t.Fatal("ai key stored in plaintext")
	}
	got, ok, err := s.GetAIKey("u1", "openai", master)
	if err != nil || !ok || got != "sk-secret" {
		t.Fatalf("GetAIKey = %q/%v/%v", got, ok, err)
	}
	if err := s.PutAIKey("u1", "openai", "sk-rotated", master); err != nil {
		t.Fatalf("rotate PutAIKey: %v", err)
	}
	got, ok, err = s.GetAIKey("u1", "openai", master)
	if err != nil || !ok || got != "sk-rotated" {
		t.Fatalf("rotated GetAIKey = %q/%v/%v", got, ok, err)
	}
}

func TestAIKeyRejectsBadMasterAndWrongAAD(t *testing.T) {
	s := openTestStore(t)
	master := []byte("0123456789abcdef0123456789abcdef")
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.PutAIKey("u1", "openai", "sk-secret", []byte("short")); err == nil {
		t.Fatal("short master key accepted")
	}
	if err := s.PutAIKey("u1", "openai", "sk-secret", master); err != nil {
		t.Fatalf("PutAIKey: %v", err)
	}
	if _, ok, err := s.GetAIKey("u2", "openai", master); err != nil || ok {
		t.Fatalf("cross-user GetAIKey = ok %v err %v, want missing", ok, err)
	}
	if _, ok, err := s.GetAIKey("u1", "openai", []byte("abcdef0123456789abcdef0123456789")); err == nil || ok {
		t.Fatalf("wrong master GetAIKey = ok %v err %v, want decrypt error", ok, err)
	}
}

func TestUsageCountersIncrementAndLimit(t *testing.T) {
	s := openTestStore(t)
	day := time.Date(2026, time.June, 18, 23, 30, 0, 0, time.FixedZone("offset", -4*60*60))
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: day}); err != nil {
		t.Fatalf("UpsertUser u1: %v", err)
	}
	if err := s.UpsertUser(User{ID: "u2", Provider: "github", Subject: "bob", CreatedAt: day}); err != nil {
		t.Fatalf("UpsertUser u2: %v", err)
	}

	usage, err := s.AddUsage("u1", day, 1, 50)
	if err != nil {
		t.Fatalf("AddUsage first: %v", err)
	}
	if usage.Day != "2026-06-19" || usage.Requests != 1 || usage.Tokens != 50 {
		t.Fatalf("first usage = %+v", usage)
	}
	usage, err = s.AddUsage("u1", day, 2, 75)
	if err != nil {
		t.Fatalf("AddUsage second: %v", err)
	}
	if usage.Requests != 3 || usage.Tokens != 125 {
		t.Fatalf("incremented usage = %+v", usage)
	}
	ok, err := s.UsageWithinLimit("u1", day, 3, 125)
	if err != nil || !ok {
		t.Fatalf("UsageWithinLimit exact = %v/%v, want true", ok, err)
	}
	ok, err = s.UsageWithinLimit("u1", day, 2, 125)
	if err != nil || ok {
		t.Fatalf("UsageWithinLimit request cap = %v/%v, want false", ok, err)
	}
	ok, err = s.UsageWithinLimit("u2", day, 0, 0)
	if err != nil || !ok {
		t.Fatalf("empty user UsageWithinLimit = %v/%v, want true", ok, err)
	}
}

func TestUsageCountersValidateAndIsolateDays(t *testing.T) {
	s := openTestStore(t)
	day := time.Date(2026, time.June, 18, 10, 0, 0, 0, time.UTC)
	nextDay := day.Add(24 * time.Hour)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: day}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if _, err := s.AddUsage("u1", day, -1, 0); err == nil {
		t.Fatal("negative request increment accepted")
	}
	if _, err := s.AddUsage("u1", day, 0, -1); err == nil {
		t.Fatal("negative token increment accepted")
	}
	if _, err := s.AddUsage("", day, 1, 1); err == nil {
		t.Fatal("blank user accepted")
	}
	if _, err := s.AddUsage("u1", day, 1, 10); err != nil {
		t.Fatalf("AddUsage day: %v", err)
	}
	if _, err := s.AddUsage("u1", nextDay, 4, 40); err != nil {
		t.Fatalf("AddUsage next day: %v", err)
	}
	usage, ok, err := s.GetUsage("u1", day)
	if err != nil || !ok || usage.Requests != 1 || usage.Tokens != 10 {
		t.Fatalf("day usage = %+v/%v/%v", usage, ok, err)
	}
	usage, ok, err = s.GetUsage("u1", nextDay)
	if err != nil || !ok || usage.Requests != 4 || usage.Tokens != 40 {
		t.Fatalf("next day usage = %+v/%v/%v", usage, ok, err)
	}
	if _, err := s.UsageWithinLimit("u1", day, -1, 0); err == nil {
		t.Fatal("negative request limit accepted")
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
