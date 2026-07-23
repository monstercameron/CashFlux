// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
)

// fakeUploadStream is an in-memory grpc.ServerStream for driving
// blobServer.UploadBlob directly, without a real network transport: RecvMsg
// pops from a queue of pre-built client messages, SendMsg captures the
// server's single response.
type fakeUploadStream struct {
	grpc.ServerStream
	ctx  context.Context
	in   []backendrpc.UploadBlobChunk
	pos  int
	resp backendrpc.UploadBlobResponse
}

func (s *fakeUploadStream) Context() context.Context { return s.ctx }

func (s *fakeUploadStream) RecvMsg(m any) error {
	if s.pos >= len(s.in) {
		return io.EOF
	}
	chunk := s.in[s.pos]
	s.pos++
	*(m.(*backendrpc.UploadBlobChunk)) = chunk
	return nil
}

func (s *fakeUploadStream) SendMsg(m any) error {
	s.resp = *(m.(*backendrpc.UploadBlobResponse))
	return nil
}

// fakeDownloadStream is the download-side twin: SendMsg accumulates every
// streamed chunk's bytes in order.
type fakeDownloadStream struct {
	grpc.ServerStream
	ctx  context.Context
	data []byte
}

func (s *fakeDownloadStream) Context() context.Context { return s.ctx }

func (s *fakeDownloadStream) SendMsg(m any) error {
	s.data = append(s.data, m.(*backendrpc.DownloadBlobChunk).Data...)
	return nil
}

func testBlobServer(t *testing.T, cfg Config) (*blobServer, *Store) {
	t.Helper()
	store := openTestStore(t)
	if cfg.DataDir == "" {
		cfg.DataDir = t.TempDir()
	}
	return newBlobService(store, cfg), store
}

func hashOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func uploadCtx(userID string) context.Context {
	return ContextWithAuthUser(context.Background(), AuthUser{ID: userID})
}

// seedTestWorkspace creates userID (if not already present) and a workspace
// it owns, so blobServer's per-workspace ownership/link checks (WorkspaceID
// on UploadBlobHeader/DownloadBlobRequest) have something real to verify
// against — mirroring http_test.go's TestBlobEndpointsRequireOwnedWorkspaceLink
// seeding for the REST transport.
func seedTestWorkspace(t *testing.T, store *Store, userID, workspaceID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := store.UpsertUser(User{ID: userID, Provider: "token", Subject: userID, CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser(%s): %v", userID, err)
	}
	if err := store.PutWorkspace(Workspace{ID: workspaceID, UserID: userID, Name: "Test Workspace", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace(%s, %s): %v", workspaceID, userID, err)
	}
}

func chunkedUpload(hash, workspaceID string, declaredSize int64, mime string, data []byte, chunkSize int) []backendrpc.UploadBlobChunk {
	msgs := []backendrpc.UploadBlobChunk{{Header: &backendrpc.UploadBlobHeader{
		Hash: hash, DeclaredSizeBytes: declaredSize, Mime: mime, WorkspaceID: workspaceID,
	}}}
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		msgs = append(msgs, backendrpc.UploadBlobChunk{Data: data[i:end]})
	}
	return msgs
}

func TestBlobServiceUploadDownloadRoundTrip(t *testing.T) {
	data := []byte("hello streaming blob world, round tripped end to end")
	hash := hashOf(data)
	s, store := testBlobServer(t, Config{})
	seedTestWorkspace(t, store, "user-1", "w1")

	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "w1", int64(len(data)), "text/plain", data, 7)}
	if err := s.UploadBlob(up); err != nil {
		t.Fatalf("UploadBlob: %v", err)
	}
	if up.resp.Hash != hash || up.resp.Size != int64(len(data)) {
		t.Fatalf("upload response = %+v, want hash %s size %d", up.resp, hash, len(data))
	}

	down := &fakeDownloadStream{ctx: uploadCtx("user-1")}
	if err := s.DownloadBlob(backendrpc.DownloadBlobRequest{Hash: hash, WorkspaceID: "w1"}, down); err != nil {
		t.Fatalf("DownloadBlob: %v", err)
	}
	if string(down.data) != string(data) {
		t.Fatalf("downloaded bytes = %q, want %q", down.data, data)
	}

	// The staged partial file must not remain after a successful commit.
	entries, err := os.ReadDir(filepath.Join(blobRoot(s.cfg), "partials"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read partials dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("partials dir not empty after successful upload: %v", entries)
	}
}

// TestBlobServiceDownloadCrossTenantIsolation proves that BlobService.DownloadBlob
// enforces the same per-user tenant isolation as the REST GET /v1/blobs/{hash}
// route (see TestBlobEndpointsRequireOwnedWorkspaceLink in http_test.go and the
// "strict tenant isolation ... cross-user tests covering workspace and blob
// access" invariant documented in docs/BACKEND_SECURITY.md). The REST route
// requires the caller's workspace to be linked to the hash
// (authorizedBlobWorkspace with requireLink=true / Store.UserWorkspaceBlob)
// before serving bytes; this test demonstrates whether the gRPC transport does
// the same for a blob user-1 uploaded that user-2 never linked to anything.
func TestBlobServiceDownloadCrossTenantIsolation(t *testing.T) {
	data := []byte("user-1's private financial receipt")
	hash := hashOf(data)
	s, store := testBlobServer(t, Config{})
	seedTestWorkspace(t, store, "user-1", "w1")
	seedTestWorkspace(t, store, "user-2", "w2")

	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "w1", int64(len(data)), "text/plain", data, 9)}
	if err := s.UploadBlob(up); err != nil {
		t.Fatalf("UploadBlob: %v", err)
	}

	// user-2 tries their OWN workspace, which is simply never linked to this hash.
	down := &fakeDownloadStream{ctx: uploadCtx("user-2")}
	if err := s.DownloadBlob(backendrpc.DownloadBlobRequest{Hash: hash, WorkspaceID: "w2"}, down); err == nil {
		t.Fatalf("DownloadBlob: user-2 downloaded user-1's blob via their own, unlinked workspace — tenant isolation bypass; got bytes %q", down.data)
	}

	// user-2 tries user-1's workspace id directly — must fail at the ownership
	// check, not merely the link check.
	down2 := &fakeDownloadStream{ctx: uploadCtx("user-2")}
	if err := s.DownloadBlob(backendrpc.DownloadBlobRequest{Hash: hash, WorkspaceID: "w1"}, down2); err == nil {
		t.Fatalf("DownloadBlob: user-2 downloaded user-1's blob by naming user-1's workspace id — ownership bypass; got bytes %q", down2.data)
	}
}

func TestBlobServiceDownloadNonexistentHash(t *testing.T) {
	s, store := testBlobServer(t, Config{})
	seedTestWorkspace(t, store, "user-1", "w1")
	down := &fakeDownloadStream{ctx: uploadCtx("user-1")}
	err := s.DownloadBlob(backendrpc.DownloadBlobRequest{Hash: hashOf([]byte("never uploaded")), WorkspaceID: "w1"}, down)
	if err == nil {
		t.Fatal("DownloadBlob: want error for nonexistent hash, got nil")
	}
}

func TestBlobServiceDownloadRequiresWorkspaceID(t *testing.T) {
	data := []byte("payload")
	hash := hashOf(data)
	s, store := testBlobServer(t, Config{})
	seedTestWorkspace(t, store, "user-1", "w1")
	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "w1", int64(len(data)), "", data, 4)}
	if err := s.UploadBlob(up); err != nil {
		t.Fatalf("UploadBlob: %v", err)
	}
	down := &fakeDownloadStream{ctx: uploadCtx("user-1")}
	if err := s.DownloadBlob(backendrpc.DownloadBlobRequest{Hash: hash}, down); err == nil {
		t.Fatal("DownloadBlob: want error when WorkspaceID is missing, got nil")
	}
}

func TestBlobServiceUploadRequiresWorkspaceID(t *testing.T) {
	data := []byte("payload")
	hash := hashOf(data)
	s, store := testBlobServer(t, Config{})
	seedTestWorkspace(t, store, "user-1", "w1")
	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "", int64(len(data)), "", data, 4)}
	if err := s.UploadBlob(up); err == nil {
		t.Fatal("UploadBlob: want error when WorkspaceID is missing, got nil")
	}
}

func TestBlobServiceUploadRejectsUnownedWorkspace(t *testing.T) {
	data := []byte("payload")
	hash := hashOf(data)
	s, store := testBlobServer(t, Config{})
	seedTestWorkspace(t, store, "user-2", "w2")
	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "w2", int64(len(data)), "", data, 4)}
	if err := s.UploadBlob(up); err == nil {
		t.Fatal("UploadBlob: want error uploading against a workspace user-1 does not own, got nil")
	}
}

// TestBlobServiceUploadEnforcesPerPlanStorageLimit proves that
// blobWithinStorageQuota (the only enforcement point on this transport, and
// blob_http.go's withinStorageQuota mirrors it exactly for REST) actually
// consults Config.StorageLimitForPlan/StoragePlanBytesOverride via
// storageLimitForUser, not just the flat, global Config.StorageMaxBytes.
// Config.StorageLimitForPlan exists specifically so an operator can give a
// cheaper plan a SMALLER quota than the global default
// (accountservice.go's GetEntitlement and admin_manage.go's admin view both
// already report the per-plan number to the client/operator as the real
// limit) — before storageLimitForUser existed, neither actual upload gate
// looked up the caller's subscription plan at all, so a user on a
// deliberately storage-limited cheap plan could consume storage all the way
// up to the operator's generous GLOBAL cap, a materially larger quota than
// the plan they were paying for (or not paying for) granted.
func TestBlobServiceUploadEnforcesPerPlanStorageLimit(t *testing.T) {
	s, store := testBlobServer(t, Config{
		Billing:         true,
		StorageMaxBytes: 1_000_000, // generous global fallback for well-paying plans
		StoragePlanBytesOverride: map[string]int64{
			"free": 100, // the free tier is supposed to be capped far below the global default
		},
	})
	seedTestWorkspace(t, store, "user-1", "w1")
	if err := store.PutSubscription(Subscription{
		UserID: "user-1", ProviderCustomer: "cus-1", ProviderSubscription: "sub-1",
		Status: "active", Plan: "free", UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("PutSubscription: %v", err)
	}

	// Sanity: the plan-aware limit function (already used for reporting) does
	// say this user's real cap is 100 bytes.
	if got := s.cfg.StorageLimitForPlan("free"); got != 100 {
		t.Fatalf("StorageLimitForPlan(free) = %d, want 100", got)
	}

	// A payload far over the free plan's 100-byte allowance, but comfortably
	// under the global 1,000,000-byte fallback.
	data := make([]byte, 5000)
	hash := hashOf(data)
	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "w1", int64(len(data)), "", data, 512)}
	if err := s.UploadBlob(up); err == nil {
		t.Fatal("UploadBlob: a free-plan user's 5000-byte upload should have been rejected against their real 100-byte plan limit, but the actual quota gate only checks the global cap and let it through")
	}
}

// TestBlobServiceUploadCountsTowardStorageQuota proves that repeated uploads
// through BlobService.UploadBlob actually accrue against the uploading user's
// storage quota (Store.UserBlobBytes), the way the REST PUT /v1/blobs/{hash}
// route does via Store.LinkWorkspaceBlob (blob_http.go's handlePutBlob). If
// UploadBlob never attributes the blob to the user, UserBlobBytes stays flat
// no matter how much distinct content is uploaded, and blobWithinStorageQuota
// (the only quota gate on this transport) can never trip — an unlimited
// storage bypass, since C434's soft/hard checks both key off UserBlobBytes.
func TestBlobServiceUploadCountsTowardStorageQuota(t *testing.T) {
	s, store := testBlobServer(t, Config{})
	seedTestWorkspace(t, store, "user-1", "w1")

	for i := 0; i < 3; i++ {
		data := []byte(fmt.Sprintf("distinct payload number %d, unique per iteration", i))
		hash := hashOf(data)
		up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "w1", int64(len(data)), "", data, 6)}
		if err := s.UploadBlob(up); err != nil {
			t.Fatalf("UploadBlob #%d: %v", i, err)
		}
	}

	used, err := store.UserBlobBytes("user-1")
	if err != nil {
		t.Fatalf("UserBlobBytes: %v", err)
	}
	if used == 0 {
		t.Fatalf("UserBlobBytes after 3 uploads = 0 — BlobService.UploadBlob never attributes uploaded bytes to the user's quota (missing LinkWorkspaceBlob), so the storage cap can never be enforced on this transport")
	}
}

func TestBlobServiceUploadSoftRejectsDeclaredSizeOverQuota(t *testing.T) {
	data := []byte("some content that would exceed a tiny declared quota")
	hash := hashOf(data)
	s, store := testBlobServer(t, Config{StorageMaxBytes: 10})
	seedTestWorkspace(t, store, "user-1", "w1")

	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "w1", int64(len(data)), "", data, 8)}
	err := s.UploadBlob(up)
	if err == nil {
		t.Fatal("UploadBlob: want soft quota rejection, got nil")
	}
	if _, ok, gerr := s.store.GetBlob(hash); gerr != nil || ok {
		t.Fatalf("blob must not be persisted after a soft-rejected upload: ok=%v err=%v", ok, gerr)
	}
}

func TestBlobServiceUploadHardRejectsActualSizeOverQuotaAfterDeclaredPassed(t *testing.T) {
	data := []byte("the declared size understates this payload substantially so it slips past the soft check")
	hash := hashOf(data)
	// Declare a tiny size (passes the soft pre-check) but actually send the
	// full payload, which alone exceeds the quota once real bytes land.
	s, store := testBlobServer(t, Config{StorageMaxBytes: int64(len(data)) - 1})
	seedTestWorkspace(t, store, "user-1", "w1")

	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(hash, "w1", 1, "", data, 11)}
	err := s.UploadBlob(up)
	if err == nil {
		t.Fatal("UploadBlob: want hard quota rejection after actual bytes exceed cap, got nil")
	}
	if _, ok, gerr := s.store.GetBlob(hash); gerr != nil || ok {
		t.Fatalf("blob must be rolled back (never persisted) after a hard-rejected upload: ok=%v err=%v", ok, gerr)
	}
	// The staged partial must also be cleaned up — nothing squats on disk.
	entries, err2 := os.ReadDir(filepath.Join(blobRoot(s.cfg), "partials"))
	if err2 != nil && !os.IsNotExist(err2) {
		t.Fatalf("read partials dir: %v", err2)
	}
	if len(entries) != 0 {
		t.Fatalf("partials dir not empty after a rolled-back upload: %v", entries)
	}
}

func TestBlobServiceUploadRejectsHashMismatch(t *testing.T) {
	data := []byte("this is the real payload")
	wrongHash := hashOf([]byte("not the real payload"))
	s, store := testBlobServer(t, Config{})
	seedTestWorkspace(t, store, "user-1", "w1")

	up := &fakeUploadStream{ctx: uploadCtx("user-1"), in: chunkedUpload(wrongHash, "w1", int64(len(data)), "", data, 5)}
	if err := s.UploadBlob(up); err == nil {
		t.Fatal("UploadBlob: want error on hash mismatch, got nil")
	}
}

func TestBlobServiceUploadRequiresAuthenticatedUser(t *testing.T) {
	data := []byte("payload")
	hash := hashOf(data)
	s, _ := testBlobServer(t, Config{})
	up := &fakeUploadStream{ctx: context.Background(), in: chunkedUpload(hash, "w1", int64(len(data)), "", data, 4)}
	if err := s.UploadBlob(up); err == nil {
		t.Fatal("UploadBlob: want error without an authenticated user, got nil")
	}
}

// TestRunBlobCleanupOnOpenSlowUploadDoesNotBlockOtherReaping proves
// RunBlobCleanup survives a still-active, legitimately slow upload whose
// partial looks stale by mtime (a throttled mobile connection, a large file
// over a congested link — not abandoned, just quiet between chunks longer
// than maxAge) without letting it block the rest of the sweep. On Windows,
// os.Remove on that still-open partial fails with a sharing violation ("The
// process cannot access the file because it is being used by another
// process"); RunBlobCleanup must treat that as a skip, not abort the whole
// sweep — otherwise one slow-but-alive upload whose temp filename happens to
// sort before another, genuinely abandoned partial in os.ReadDir's lexical
// order would permanently block that abandoned partial from ever being
// reaped, for as long as the slow upload's connection stays open (every
// periodic sweep would hit the same busy file and abort at the same point).
func TestRunBlobCleanupOnOpenSlowUploadDoesNotBlockOtherReaping(t *testing.T) {
	dataDir := t.TempDir()
	partials := filepath.Join(dataDir, "blobs", "partials")
	if err := os.MkdirAll(partials, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Named to sort before the genuinely-abandoned partial below, so it's
	// visited first in os.ReadDir's lexical order.
	busyPath := filepath.Join(partials, "aaa-slow-but-active.partial")
	f, err := os.OpenFile(busyPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open busy partial: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("first chunk of a real, still-in-progress upload"); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(busyPath, staleTime, staleTime); err != nil {
		t.Fatalf("chtimes (simulating a slow gap since the last chunk): %v", err)
	}

	// A completely unrelated, truly abandoned partial that sorts AFTER the
	// busy one and should be reaped on any correct sweep.
	abandonedPath := filepath.Join(partials, "zzz-actually-abandoned.partial")
	if err := os.WriteFile(abandonedPath, []byte("orphaned"), 0o600); err != nil {
		t.Fatalf("write abandoned partial: %v", err)
	}
	if err := os.Chtimes(abandonedPath, staleTime, staleTime); err != nil {
		t.Fatalf("chtimes abandoned: %v", err)
	}

	deleted, err := RunBlobCleanup(context.Background(), dataDir, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("RunBlobCleanup: want no error (the busy file should be skipped, not fatal), got %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (only the genuinely abandoned partial)", deleted)
	}

	if _, statErr := os.Stat(abandonedPath); statErr == nil {
		t.Fatal("the genuinely abandoned, unrelated partial that sorted after the busy file was not reaped — one slow-but-active upload blocked cleanup of everything after it")
	}

	// The still-open, still-in-progress upload's own staging file survived —
	// proving it's Windows' sharing violation, not a successful sweep, that
	// protected it; it was never meant to be reaped in the first place since
	// the upload is still active.
	if _, statErr := os.Stat(busyPath); statErr != nil {
		t.Fatalf("busy partial should still exist (open handle blocked its removal): %v", statErr)
	}
}

func TestRunBlobCleanupReapsOldPartialsOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir())
	partials := filepath.Join(dir, "blobs", "partials")
	if err := os.MkdirAll(partials, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oldPath := filepath.Join(partials, "old.partial")
	freshPath := filepath.Join(partials, "fresh.partial")
	otherPath := filepath.Join(partials, "not-a-partial.txt")
	for _, p := range []string{oldPath, freshPath, otherPath} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	deleted, err := RunBlobCleanup(context.Background(), dir, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("RunBlobCleanup: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old partial should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh partial should remain: %v", err)
	}
	if _, err := os.Stat(otherPath); err != nil {
		t.Fatalf("non-.partial file should remain untouched: %v", err)
	}
}

func TestStartBlobCleanupSweepsAndStops(t *testing.T) {
	dir := t.TempDir()
	partials := filepath.Join(dir, "blobs", "partials")
	if err := os.MkdirAll(partials, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oldPath := filepath.Join(partials, "old.partial")
	if err := os.WriteFile(oldPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartBlobCleanup(ctx, dir, time.Hour, time.Hour, nil)
	cancel()
	stop()

	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("StartBlobCleanup should have swept the old partial on its immediate first pass, stat err = %v", err)
	}
}
