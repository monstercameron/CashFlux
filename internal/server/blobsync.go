// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BlobSyncService provides tenant-scoped, ciphertext-only change storage.
type BlobSyncService struct {
	store   *Store
	cfg     Config
	mu      sync.Mutex
	buckets map[string]blobSyncBucket
}

type blobSyncBucket struct {
	tokens float64
	at     time.Time
}

const defaultBlobSyncPairingMaxBytes int64 = 64 << 10

const maxBlobSyncRateBuckets = 10_000

// NewBlobSyncService creates the v2 blind sync service.
func NewBlobSyncService(store *Store, cfg Config) *BlobSyncService {
	return &BlobSyncService{store: store, cfg: cfg, buckets: make(map[string]blobSyncBucket)}
}

func (s *BlobSyncService) caller(ctx context.Context) (AuthUser, error) {
	u, ok := AuthUserFromContext(ctx)
	if !ok || strings.TrimSpace(u.ID) == "" {
		return AuthUser{}, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	if s == nil || s.store == nil {
		return AuthUser{}, status.Error(codes.FailedPrecondition, "blob sync store is not configured")
	}
	if err := s.allow(u.ID); err != nil {
		return AuthUser{}, err
	}
	return u, nil
}

func (s *BlobSyncService) allow(userID string) error {
	rate, burst := s.cfg.BlobSyncRatePerSecond, s.cfg.BlobSyncRateBurst
	if rate <= 0 || burst <= 0 {
		return nil
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	idleWindow := blobSyncBucketIdleWindow(rate, burst)
	for id, bucket := range s.buckets {
		if now.Sub(bucket.at) > idleWindow {
			delete(s.buckets, id)
		}
	}
	b, ok := s.buckets[userID]
	if !ok {
		if len(s.buckets) >= maxBlobSyncRateBuckets {
			return status.Error(codes.ResourceExhausted, "v2 sync rate limit capacity is full")
		}
		b = blobSyncBucket{tokens: float64(burst), at: now}
	}
	b.tokens += now.Sub(b.at).Seconds() * float64(rate)
	if b.tokens > float64(burst) {
		b.tokens = float64(burst)
	}
	b.at = now
	if b.tokens < 1 {
		s.buckets[userID] = b
		return status.Error(codes.ResourceExhausted, "v2 sync call rate limit exceeded")
	}
	b.tokens--
	s.buckets[userID] = b
	return nil
}

// blobSyncBucketIdleWindow returns the time needed to refill a full bucket.
// Removing a bucket after this interval cannot increase the user's available tokens.
func blobSyncBucketIdleWindow(rate, burst int) time.Duration {
	seconds := float64(burst) / float64(rate)
	if seconds < 1 {
		seconds = 1
	}
	maxSeconds := float64(math.MaxInt64) / float64(time.Second)
	if seconds >= maxSeconds {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(seconds * float64(time.Second))
}

// AppendChanges durably appends encrypted changes and returns idempotent sequences.
func (s *BlobSyncService) AppendChanges(ctx context.Context, req backendrpc.AppendChangesRequest) (backendrpc.AppendChangesResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.AppendChangesResponse{}, err
	}
	ws := strings.TrimSpace(req.Workspace)
	if ws == "" || len(req.Changes) == 0 {
		return backendrpc.AppendChangesResponse{}, status.Error(codes.InvalidArgument, "workspace and at least one change are required")
	}
	max := s.cfg.BlobSyncChangeMaxBytes
	if max <= 0 {
		max = 1 << 20
	}
	batch := make([]storedChange, len(req.Changes))
	seen := map[string]bool{}
	for i, c := range req.Changes {
		c.ChangeID = strings.TrimSpace(c.ChangeID)
		c.DeviceID = strings.TrimSpace(c.DeviceID)
		if c.ChangeID == "" || c.DeviceID == "" || seen[c.ChangeID] {
			return backendrpc.AppendChangesResponse{}, status.Error(codes.InvalidArgument, "change ids and device ids must be present and unique within a batch")
		}
		seen[c.ChangeID] = true
		if int64(len(c.Blob)) > max {
			return backendrpc.AppendChangesResponse{}, status.Error(codes.ResourceExhausted, "change blob exceeds configured size limit")
		}
		batch[i] = storedChange{ChangeID: c.ChangeID, DeviceID: c.DeviceID, Blob: c.Blob}
	}
	seqs, err := s.store.appendBlobChanges(ctx, u.ID, ws, batch, time.Now().UTC())
	if err != nil {
		return backendrpc.AppendChangesResponse{}, mapBlobSyncError(err)
	}
	return backendrpc.AppendChangesResponse{Seqs: seqs}, nil
}

// ListChanges returns a bounded page of encrypted changes for the caller's workspace.
func (s *BlobSyncService) ListChanges(ctx context.Context, req backendrpc.ListChangesRequest) (backendrpc.ListChangesResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.ListChangesResponse{}, err
	}
	ws := strings.TrimSpace(req.Workspace)
	if ws == "" || req.Since < 0 {
		return backendrpc.ListChangesResponse{}, status.Error(codes.InvalidArgument, "workspace and non-negative cursor are required")
	}
	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	changes, head, err := s.store.listBlobChanges(ctx, u.ID, ws, req.Since, limit)
	if err != nil {
		return backendrpc.ListChangesResponse{}, mapBlobSyncError(err)
	}
	out := make([]backendrpc.BlobChange, 0, len(changes))
	for _, c := range changes {
		out = append(out, backendrpc.BlobChange{ChangeID: c.ChangeID, DeviceID: c.DeviceID, Seq: c.Seq, Blob: c.Blob})
	}
	return backendrpc.ListChangesResponse{Changes: out, Head: head}, nil
}

// PutSnapshot stores an advancing opaque snapshot and prunes eligible old log entries.
func (s *BlobSyncService) PutSnapshot(ctx context.Context, req backendrpc.PutSnapshotRequest) (backendrpc.PutSnapshotResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.PutSnapshotResponse{}, err
	}
	max := s.cfg.BlobSyncSnapshotMaxBytes
	if max <= 0 {
		max = 32 << 20
	}
	if strings.TrimSpace(req.Workspace) == "" || req.CoversSeq <= 0 {
		return backendrpc.PutSnapshotResponse{}, status.Error(codes.InvalidArgument, "workspace and positive snapshot cursor are required")
	}
	if int64(len(req.Blob)) > max {
		return backendrpc.PutSnapshotResponse{}, status.Error(codes.ResourceExhausted, "snapshot exceeds configured size limit")
	}
	days := s.cfg.BlobSyncRetentionDays
	if days <= 0 {
		days = 30
	}
	ok, err := s.store.putBlobSnapshot(ctx, u.ID, strings.TrimSpace(req.Workspace), req.CoversSeq, req.Blob, time.Duration(days)*24*time.Hour, time.Now().UTC())
	if err != nil {
		return backendrpc.PutSnapshotResponse{}, mapBlobSyncError(err)
	}
	return backendrpc.PutSnapshotResponse{Accepted: ok}, nil
}

// GetSnapshot returns the caller's newest opaque workspace snapshot.
func (s *BlobSyncService) GetSnapshot(ctx context.Context, req backendrpc.GetSnapshotRequest) (backendrpc.GetSnapshotResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.GetSnapshotResponse{}, err
	}
	if strings.TrimSpace(req.Workspace) == "" {
		return backendrpc.GetSnapshotResponse{}, status.Error(codes.InvalidArgument, "workspace is required")
	}
	seq, b, ok, err := s.store.getBlobSnapshot(ctx, u.ID, strings.TrimSpace(req.Workspace))
	if err != nil {
		return backendrpc.GetSnapshotResponse{}, status.Error(codes.Internal, err.Error())
	}
	return backendrpc.GetSnapshotResponse{Found: ok, CoversSeq: seq, Blob: b}, nil
}

// GetKeyring returns the caller's opaque wrapped workspace keys.
func (s *BlobSyncService) GetKeyring(ctx context.Context, req backendrpc.GetKeyringRequest) (backendrpc.GetKeyringResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.GetKeyringResponse{}, err
	}
	if strings.TrimSpace(req.Workspace) == "" {
		return backendrpc.GetKeyringResponse{}, status.Error(codes.InvalidArgument, "workspace is required")
	}
	if err := s.requireLiveWorkspace(ctx, u.ID, strings.TrimSpace(req.Workspace)); err != nil {
		return backendrpc.GetKeyringResponse{}, err
	}
	v, b, ok, err := s.store.getBlobKeyring(ctx, u.ID, strings.TrimSpace(req.Workspace))
	if err != nil {
		return backendrpc.GetKeyringResponse{}, status.Error(codes.Internal, err.Error())
	}
	return backendrpc.GetKeyringResponse{Found: ok, Version: v, Blob: b}, nil
}

// PutKeyring conditionally replaces the opaque keyring using its expected version.
func (s *BlobSyncService) PutKeyring(ctx context.Context, req backendrpc.PutKeyringRequest) (backendrpc.PutKeyringResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.PutKeyringResponse{}, err
	}
	if strings.TrimSpace(req.Workspace) == "" || req.ExpectedVersion < 0 {
		return backendrpc.PutKeyringResponse{}, status.Error(codes.InvalidArgument, "workspace and non-negative expected version are required")
	}
	if err := s.requireLiveWorkspace(ctx, u.ID, strings.TrimSpace(req.Workspace)); err != nil {
		return backendrpc.PutKeyringResponse{}, err
	}
	max := s.cfg.BlobSyncChangeMaxBytes
	if max <= 0 {
		max = 1 << 20
	}
	if int64(len(req.Blob)) > max {
		return backendrpc.PutKeyringResponse{}, status.Error(codes.ResourceExhausted, "keyring blob exceeds configured size limit")
	}
	v, ok, err := s.store.putBlobKeyring(ctx, u.ID, strings.TrimSpace(req.Workspace), req.ExpectedVersion, req.Blob)
	if err != nil {
		return backendrpc.PutKeyringResponse{}, status.Error(codes.Internal, err.Error())
	}
	return backendrpc.PutKeyringResponse{Version: v, Updated: ok}, nil
}

func (s *BlobSyncService) requireLiveWorkspace(ctx context.Context, userID, workspaceID string) error {
	var deleted string
	err := s.store.db.QueryRowContext(ctx, `SELECT deleted_at FROM ws_head WHERE user_id=? AND workspace_id=?`, userID, workspaceID).Scan(&deleted)
	if err == sql.ErrNoRows {
		return status.Error(codes.NotFound, "workspace not found")
	}
	if err != nil {
		return status.Error(codes.Internal, "workspace scope could not be checked")
	}
	if deleted != "" {
		return status.Error(codes.FailedPrecondition, "workspace has been deleted")
	}
	return nil
}

// DeleteWorkspace writes a tombstone that prevents subsequent appends.
func (s *BlobSyncService) DeleteWorkspace(ctx context.Context, req backendrpc.BlobSyncDeleteWorkspaceRequest) (backendrpc.BlobSyncDeleteWorkspaceResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.BlobSyncDeleteWorkspaceResponse{}, err
	}
	if strings.TrimSpace(req.Workspace) == "" {
		return backendrpc.BlobSyncDeleteWorkspaceResponse{}, status.Error(codes.InvalidArgument, "workspace is required")
	}
	ok, err := s.store.deleteBlobWorkspace(ctx, u.ID, strings.TrimSpace(req.Workspace), time.Now())
	if err != nil {
		return backendrpc.BlobSyncDeleteWorkspaceResponse{}, status.Error(codes.Internal, err.Error())
	}
	return backendrpc.BlobSyncDeleteWorkspaceResponse{Deleted: ok}, nil
}

// CreatePairing stores an opaque pairing envelope with a bounded lifetime.
func (s *BlobSyncService) CreatePairing(ctx context.Context, req backendrpc.CreatePairingRequest) (backendrpc.CreatePairingResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.CreatePairingResponse{}, err
	}
	if len(req.Blob) == 0 {
		return backendrpc.CreatePairingResponse{}, status.Error(codes.InvalidArgument, "pairing envelope is required")
	}
	max := s.cfg.BlobSyncPairingMaxBytes
	if max <= 0 {
		max = defaultBlobSyncPairingMaxBytes
	}
	if int64(len(req.Blob)) > max {
		return backendrpc.CreatePairingResponse{}, status.Error(codes.ResourceExhausted, "pairing envelope exceeds configured size limit")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		var b [24]byte
		if _, err := rand.Read(b[:]); err != nil {
			return backendrpc.CreatePairingResponse{}, status.Error(codes.Internal, "could not create pairing id")
		}
		id = hex.EncodeToString(b[:])
	}
	ttl := req.TTLSeconds
	if ttl <= 0 || ttl > 600 {
		ttl = 300
	}
	now := time.Now().UTC()
	expires := now.Add(time.Duration(ttl) * time.Second)
	if err := s.store.sweepPairingRelay(ctx, now); err != nil {
		return backendrpc.CreatePairingResponse{}, status.Error(codes.Internal, err.Error())
	}
	result, err := s.store.db.ExecContext(ctx, `INSERT OR IGNORE INTO pairing_relay(user_id,id,created_at,expires_at,blob) VALUES(?,?,?,?,?)`, u.ID, id, now.Format(time.RFC3339Nano), expires.Format(time.RFC3339Nano), req.Blob)
	if err != nil {
		return backendrpc.CreatePairingResponse{}, status.Error(codes.Internal, "pairing relay could not be stored")
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return backendrpc.CreatePairingResponse{}, status.Error(codes.Internal, "pairing relay result unavailable")
	}
	if inserted == 0 {
		return backendrpc.CreatePairingResponse{}, status.Error(codes.AlreadyExists, "pairing id is already in use")
	}
	return backendrpc.CreatePairingResponse{ID: id, ExpiresAt: expires.Format(time.RFC3339Nano)}, nil
}

// ClaimPairing consumes a relay envelope belonging to the authenticated user.
func (s *BlobSyncService) ClaimPairing(ctx context.Context, req backendrpc.ClaimPairingRequest) (backendrpc.ClaimPairingResponse, error) {
	u, err := s.caller(ctx)
	if err != nil {
		return backendrpc.ClaimPairingResponse{}, err
	}
	if err := s.store.sweepPairingRelay(ctx, time.Now()); err != nil {
		return backendrpc.ClaimPairingResponse{}, status.Error(codes.Internal, err.Error())
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return backendrpc.ClaimPairingResponse{}, status.Error(codes.Internal, "pairing relay transaction unavailable")
	}
	defer func() { _ = tx.Rollback() }()
	var blob []byte
	err = tx.QueryRowContext(ctx, `SELECT blob FROM pairing_relay WHERE id=? AND user_id=?`, strings.TrimSpace(req.ID), u.ID).Scan(&blob)
	if err == sql.ErrNoRows {
		return backendrpc.ClaimPairingResponse{Found: false}, nil
	}
	if err != nil {
		return backendrpc.ClaimPairingResponse{}, status.Error(codes.Internal, err.Error())
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM pairing_relay WHERE id=? AND user_id=?`, strings.TrimSpace(req.ID), u.ID); err != nil {
		return backendrpc.ClaimPairingResponse{}, status.Error(codes.Internal, "pairing relay could not be consumed")
	}
	if err := tx.Commit(); err != nil {
		return backendrpc.ClaimPairingResponse{}, status.Error(codes.Internal, "pairing relay consumption was not committed")
	}
	return backendrpc.ClaimPairingResponse{Found: true, Blob: blob}, nil
}

func mapBlobSyncError(err error) error {
	if err == errWorkspaceDeleted {
		return status.Error(codes.FailedPrecondition, "workspace has been deleted")
	}
	return status.Error(codes.Internal, fmt.Sprintf("blob sync: %v", err))
}

func (s *Store) ServerID(ctx context.Context) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM server_meta WHERE name='server_id'`).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("server store: read server id: %w", err)
	}
	return id, nil
}

// Keep a compile-time assertion that the configured SQLite driver retains context support.
var _ = sql.ErrNoRows
