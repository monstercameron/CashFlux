// SPDX-License-Identifier: MIT

package syncengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/conflictcheckpoint"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/syncstate"
)

// memBlobSyncServer simulates the v2 cashflux.v2.BlobSync server in memory
// with exact gap-free sequencing, ChangeID idempotency, and snapshot retention.
type memBlobSyncServer struct {
	mu          sync.Mutex
	changes     map[string][]backendrpc.BlobChange
	byChangeID  map[string]map[string]int64
	head        map[string]int64
	snapshots   map[string]backendrpc.GetSnapshotResponse
	failAppends int
	dropReply   bool // commits on server then returns transport error
}

func newMemBlobSyncServer() *memBlobSyncServer {
	return &memBlobSyncServer{
		changes:    map[string][]backendrpc.BlobChange{},
		byChangeID: map[string]map[string]int64{},
		head:       map[string]int64{},
		snapshots:  map[string]backendrpc.GetSnapshotResponse{},
	}
}

func (s *memBlobSyncServer) AppendChanges(_ context.Context, req backendrpc.AppendChangesRequest) (backendrpc.AppendChangesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failAppends > 0 {
		s.failAppends--
		return backendrpc.AppendChangesResponse{}, errors.New("transient network timeout")
	}
	if s.byChangeID[req.Workspace] == nil {
		s.byChangeID[req.Workspace] = map[string]int64{}
	}
	seqs := make([]int64, len(req.Changes))
	for i, ch := range req.Changes {
		if existing, ok := s.byChangeID[req.Workspace][ch.ChangeID]; ok {
			seqs[i] = existing
			continue
		}
		s.head[req.Workspace]++
		seq := s.head[req.Workspace]
		ch.Seq = seq
		s.changes[req.Workspace] = append(s.changes[req.Workspace], ch)
		s.byChangeID[req.Workspace][ch.ChangeID] = seq
		seqs[i] = seq
	}
	if s.dropReply {
		s.dropReply = false
		return backendrpc.AppendChangesResponse{}, errors.New("connection reset after commit")
	}
	return backendrpc.AppendChangesResponse{Seqs: seqs}, nil
}

func (s *memBlobSyncServer) ListChanges(_ context.Context, req backendrpc.ListChangesRequest) (backendrpc.ListChangesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := req.Limit
	if limit <= 0 {
		limit = 250
	}
	var out []backendrpc.BlobChange
	for _, ch := range s.changes[req.Workspace] {
		if ch.Seq > req.Since {
			out = append(out, ch)
			if len(out) >= limit {
				break
			}
		}
	}
	return backendrpc.ListChangesResponse{Changes: out, Head: s.head[req.Workspace]}, nil
}

func (s *memBlobSyncServer) PutSnapshot(_ context.Context, req backendrpc.PutSnapshotRequest) (backendrpc.PutSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.snapshots[req.Workspace]
	if req.CoversSeq <= cur.CoversSeq || req.CoversSeq > s.head[req.Workspace] {
		return backendrpc.PutSnapshotResponse{Accepted: false}, nil
	}
	s.snapshots[req.Workspace] = backendrpc.GetSnapshotResponse{
		Found:     true,
		CoversSeq: req.CoversSeq,
		Blob:      append([]byte(nil), req.Blob...),
	}
	return backendrpc.PutSnapshotResponse{Accepted: true}, nil
}

func (s *memBlobSyncServer) GetSnapshot(_ context.Context, req backendrpc.GetSnapshotRequest) (backendrpc.GetSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshots[req.Workspace], nil
}

func (s *memBlobSyncServer) pruneBefore(ws string, keepAfterSeq int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var kept []backendrpc.BlobChange
	for _, ch := range s.changes[ws] {
		if ch.Seq > keepAfterSeq {
			kept = append(kept, ch)
		}
	}
	s.changes[ws] = kept
}

type fakeCipher struct {
	readyErr error
	prefix   []byte
}

func (c *fakeCipher) Ready() error { return c.readyErr }
func (c *fakeCipher) Encrypt(_ context.Context, plain []byte) ([]byte, error) {
	if c.readyErr != nil {
		return nil, c.readyErr
	}
	p := c.prefix
	if len(p) == 0 {
		p = []byte("ENC:")
	}
	return append(append([]byte(nil), p...), plain...), nil
}
func (c *fakeCipher) Decrypt(_ context.Context, ct []byte) ([]byte, error) {
	if c.readyErr != nil {
		return nil, c.readyErr
	}
	p := c.prefix
	if len(p) == 0 {
		p = []byte("ENC:")
	}
	if !bytes.HasPrefix(ct, p) {
		return nil, ErrWrongPasscode
	}
	return append([]byte(nil), ct[len(p):]...), nil
}

type sqliteLocalAdapter struct {
	db             *store.SQLiteStore
	liveApplyCount int
	snapshotCount  int
}

func newSQLiteLocalAdapter(t *testing.T) *sqliteLocalAdapter {
	t.Helper()
	db, err := store.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &sqliteLocalAdapter{db: db}
}

func (a *sqliteLocalAdapter) ExportRedactedJSON() ([]byte, error) { return exportJSONFromStore(a.db) }
func (a *sqliteLocalAdapter) ApplyRemoteChanges(changes []store.RecordChange) error {
	a.liveApplyCount++
	return a.db.ReplayChanges(changes)
}
func (a *sqliteLocalAdapter) ApplyRemoteSnapshot(snapshotJSON []byte) error {
	a.snapshotCount++
	return loadJSONIntoStore(a.db, snapshotJSON)
}

type memStateStore struct {
	mu        sync.Mutex
	cursors   map[conflictcheckpoint.Scope]WorkspaceCursor
	outbox    []OutboxEntry
	conflicts map[conflictcheckpoint.Scope]*StoredConflict
}

func newMemStateStore() *memStateStore {
	return &memStateStore{
		cursors:   map[conflictcheckpoint.Scope]WorkspaceCursor{},
		conflicts: map[conflictcheckpoint.Scope]*StoredConflict{},
	}
}

func (m *memStateStore) LoadCursor(scope conflictcheckpoint.Scope) (WorkspaceCursor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cursors[scope], nil
}
func (m *memStateStore) SaveCursor(c WorkspaceCursor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cursors[c.Scope()] = c
	return nil
}
func (m *memStateStore) LoadOutbox() ([]OutboxEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]OutboxEntry, len(m.outbox))
	copy(out, m.outbox)
	return out, nil
}
func (m *memStateStore) SaveOutbox(entries []OutboxEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]OutboxEntry, len(entries))
	copy(out, entries)
	m.outbox = out
	return nil
}
func (m *memStateStore) LoadConflict(scope conflictcheckpoint.Scope) (*StoredConflict, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.conflicts[scope]
	if c == nil {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}
func (m *memStateStore) SaveConflict(c StoredConflict) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := c
	m.conflicts[c.Checkpoint.Scope] = &cp
	return nil
}
func (m *memStateStore) ClearConflict(scope conflictcheckpoint.Scope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conflicts, scope)
	return nil
}

func seedHousehold(t *testing.T, db *store.SQLiteStore) {
	t.Helper()
	if err := db.PutAccount(domain.Account{ID: "acc-1", Name: "Checking", Class: domain.ClassAsset, Type: domain.AccountType("checking"), Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if err := db.PutCategory(domain.Category{ID: "cat-1", Name: "Groceries", Kind: domain.CategoryKind("expense")}); err != nil {
		t.Fatal(err)
	}
	if err := db.PutTransaction(domain.Transaction{
		ID: "tx-1", AccountID: "acc-1", CategoryID: "cat-1",
		Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(-2500, "USD"), Payee: "Market", Desc: "Market",
	}); err != nil {
		t.Fatal(err)
	}
}

func captureDelta(t *testing.T, db *store.SQLiteStore, fn func()) []store.RecordChange {
	t.Helper()
	before, err := db.ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	fn()
	after, err := db.ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	page, err := db.CaptureChanges(before, after, 128)
	if err != nil {
		t.Fatal(err)
	}
	return store.CoalesceChanges(page.Changes)
}

func TestV2TwoDeviceAutoMergeLiveApplyWithoutConflict(t *testing.T) {
	srv := newMemBlobSyncServer()
	scope := conflictcheckpoint.Scope{ServerID: "srv-1", AccountID: "acct-1", WorkspaceID: "ws-default"}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

	localA := newSQLiteLocalAdapter(t)
	seedHousehold(t, localA.db)
	stateA := newMemStateStore()
	engA, err := New(Config{Scope: scope, DeviceID: "dev-a", Now: func() time.Time { return now }}, srv, &fakeCipher{}, localA, stateA)
	if err != nil {
		t.Fatal(err)
	}

	// Device A performs initial sync (seeds v2 log + compacted snapshot).
	if err := engA.SyncOnce(context.Background()); err != nil {
		t.Fatalf("device A initial sync: %v", err)
	}
	if engA.Status().Phase != syncstate.PhaseSynced || engA.Status().AppliedSeq != 1 {
		t.Fatalf("device A status = %+v", engA.Status())
	}

	// Device B bootstraps cleanly from v2 snapshot (live-apply, no reload).
	localB := newSQLiteLocalAdapter(t)
	stateB := newMemStateStore()
	engB, err := New(Config{Scope: scope, DeviceID: "dev-b", Now: func() time.Time { return now }}, srv, &fakeCipher{}, localB, stateB)
	if err != nil {
		t.Fatal(err)
	}
	if err := engB.SyncOnce(context.Background()); err != nil {
		t.Fatalf("device B bootstrap sync: %v", err)
	}
	if localB.snapshotCount != 1 {
		t.Fatalf("expected 1 bootstrap snapshot apply on B, got %d", localB.snapshotCount)
	}

	// Both devices edit DIFFERENT transactions offline (non-overlapping).
	changesA := captureDelta(t, localA.db, func() {
		_ = localA.db.PutTransaction(domain.Transaction{
			ID: "tx-a", AccountID: "acc-1", CategoryID: "cat-1",
			Date: now, Amount: money.New(-1500, "USD"), Payee: "Coffee A", Desc: "Coffee A",
		})
	})
	if err := engA.EnqueueRecords("chg-a-1", changesA); err != nil {
		t.Fatal(err)
	}

	changesB := captureDelta(t, localB.db, func() {
		_ = localB.db.PutTransaction(domain.Transaction{
			ID: "tx-b", AccountID: "acc-1", CategoryID: "cat-1",
			Date: now, Amount: money.New(-3200, "USD"), Payee: "Books B", Desc: "Books B",
		})
	})
	if err := engB.EnqueueRecords("chg-b-1", changesB); err != nil {
		t.Fatal(err)
	}

	// Device A pushes first, then Device B syncs — B must auto-merge A's tx-a
	// live in place and push tx-b with ZERO conflict!
	if err := engA.SyncOnce(context.Background()); err != nil {
		t.Fatalf("device A push: %v", err)
	}
	if err := engB.SyncOnce(context.Background()); err != nil {
		t.Fatalf("device B sync: %v", err)
	}
	if engB.Status().Phase != syncstate.PhaseSynced {
		t.Fatalf("device B phase = %q, want synced", engB.Status().Phase)
	}
	if localB.liveApplyCount != 1 {
		t.Fatalf("expected live incremental apply on B, got %d", localB.liveApplyCount)
	}

	// Device A pulls B's tx-b live in place.
	if err := engA.SyncOnce(context.Background()); err != nil {
		t.Fatalf("device A pull: %v", err)
	}
	for name, db := range map[string]*store.SQLiteStore{"A": localA.db, "B": localB.db} {
		txns, err := db.ListTransactions()
		if err != nil {
			t.Fatal(err)
		}
		if len(txns) != 3 {
			t.Fatalf("device %s txns = %d, want 3 (tx-1, tx-a, tx-b)", name, len(txns))
		}
	}
}

func TestV2TrueConflictCheckpointAndResolutions(t *testing.T) {
	modes := []string{"keep-local", "use-server", "merge-prefer-local", "merge-prefer-server"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			srv := newMemBlobSyncServer()
			scope := conflictcheckpoint.Scope{ServerID: "srv-1", AccountID: "acct-1", WorkspaceID: "ws-1"}
			now := time.Date(2026, 11, 27, 13, 0, 0, 0, time.UTC)

			localA := newSQLiteLocalAdapter(t)
			seedHousehold(t, localA.db)
			engA, _ := New(Config{Scope: scope, DeviceID: "dev-a", Now: func() time.Time { return now }}, srv, &fakeCipher{}, localA, newMemStateStore())
			if err := engA.SyncOnce(context.Background()); err != nil {
				t.Fatal(err)
			}

			localB := newSQLiteLocalAdapter(t)
			stateB := newMemStateStore()
			engB, _ := New(Config{Scope: scope, DeviceID: "dev-b", Now: func() time.Time { return now }}, srv, &fakeCipher{}, localB, stateB)
			if err := engB.SyncOnce(context.Background()); err != nil {
				t.Fatal(err)
			}

			// Both A and B edit tx-1 offline (AND B also adds tx-b-only).
			changesA := captureDelta(t, localA.db, func() {
				_ = localA.db.PutTransaction(domain.Transaction{
					ID: "tx-1", AccountID: "acc-1", CategoryID: "cat-1",
					Date: now, Amount: money.New(-9900, "USD"), Payee: "Server Merchant A", Desc: "Server Merchant A",
				})
				_ = localA.db.PutTransaction(domain.Transaction{
					ID: "tx-a-only", AccountID: "acc-1", CategoryID: "cat-1",
					Date: now, Amount: money.New(-1100, "USD"), Payee: "Only On A", Desc: "Only On A",
				})
			})
			_ = engA.EnqueueRecords("chg-a-conflict", changesA)
			if err := engA.SyncOnce(context.Background()); err != nil {
				t.Fatal(err)
			}

			changesB := captureDelta(t, localB.db, func() {
				_ = localB.db.PutTransaction(domain.Transaction{
					ID: "tx-1", AccountID: "acc-1", CategoryID: "cat-1",
					Date: now, Amount: money.New(-5500, "USD"), Payee: "Local Merchant B", Desc: "Local Merchant B",
				})
				_ = localB.db.PutTransaction(domain.Transaction{
					ID: "tx-b-only", AccountID: "acc-1", CategoryID: "cat-1",
					Date: now, Amount: money.New(-2200, "USD"), Payee: "Only On B", Desc: "Only On B",
				})
			})
			_ = engB.EnqueueRecords("chg-b-conflict", changesB)

			// Syncing B MUST enter PhaseConflict and create a durable conflictcheckpoint.
			if err := engB.SyncOnce(context.Background()); err != nil {
				t.Fatalf("device B conflict sync err: %v", err)
			}
			if engB.Status().Phase != syncstate.PhaseConflict {
				t.Fatalf("phase = %q, want conflict", engB.Status().Phase)
			}
			// Routine offline notification must NOT overwrite conflict phase.
			engB.NotifyOffline()
			if engB.Status().Phase != syncstate.PhaseConflict {
				t.Fatalf("NotifyOffline clobbered conflict phase: %q", engB.Status().Phase)
			}
			engB.NotifyOnline()

			conflict, err := stateB.LoadConflict(scope)
			if err != nil || conflict == nil {
				t.Fatalf("expected stored conflict checkpoint, got %+v err=%v", conflict, err)
			}
			if len(conflict.CollidingKeys) != 1 || conflict.CollidingKeys[0].ID != "tx-1" {
				t.Fatalf("colliding keys = %+v, want [transactions/tx-1]", conflict.CollidingKeys)
			}
			// Verify deterministic conflictcheckpoint.Compare works on stored copies.
			localSnap, _, err := DatasetToCheckpointSnapshot(conflict.LocalJSON)
			if err != nil {
				t.Fatal(err)
			}
			serverSnap, _, err := DatasetToCheckpointSnapshot(conflict.ServerJSON)
			if err != nil {
				t.Fatal(err)
			}
			cmp, err := conflictcheckpoint.Compare(localSnap, serverSnap)
			if err != nil || !cmp.Available || len(cmp.Changes) == 0 {
				t.Fatalf("Compare failed: %+v err=%v", cmp, err)
			}

			switch mode {
			case "keep-local":
				if err := engB.ResolveKeepLocal(context.Background()); err != nil {
					t.Fatal(err)
				}
				tx1, ok, _ := localB.db.GetTransaction("tx-1")
				if !ok || tx1.Payee != "Local Merchant B" {
					t.Fatalf("keep-local tx-1 = %+v", tx1)
				}
			case "use-server":
				if err := engB.ResolveUseServer(context.Background()); err != nil {
					t.Fatal(err)
				}
				tx1, ok, _ := localB.db.GetTransaction("tx-1")
				if !ok || tx1.Payee != "Server Merchant A" {
					t.Fatalf("use-server tx-1 = %+v", tx1)
				}
			case "merge-prefer-local":
				if _, err := engB.ResolveMerge(context.Background(), PreferLocal); err != nil {
					t.Fatal(err)
				}
				tx1, _, _ := localB.db.GetTransaction("tx-1")
				_, hasA, _ := localB.db.GetTransaction("tx-a-only")
				_, hasB, _ := localB.db.GetTransaction("tx-b-only")
				if tx1.Payee != "Local Merchant B" || !hasA || !hasB {
					t.Fatalf("merge-prefer-local failed: tx1=%q hasA=%v hasB=%v", tx1.Payee, hasA, hasB)
				}
			case "merge-prefer-server":
				if _, err := engB.ResolveMerge(context.Background(), PreferServer); err != nil {
					t.Fatal(err)
				}
				tx1, _, _ := localB.db.GetTransaction("tx-1")
				_, hasA, _ := localB.db.GetTransaction("tx-a-only")
				_, hasB, _ := localB.db.GetTransaction("tx-b-only")
				if tx1.Payee != "Server Merchant A" || !hasA || !hasB {
					t.Fatalf("merge-prefer-server failed: tx1=%q hasA=%v hasB=%v", tx1.Payee, hasA, hasB)
				}
			}

			if engB.Status().Phase != syncstate.PhaseSynced {
				t.Fatalf("post-resolve phase = %q, want synced", engB.Status().Phase)
			}
			// Device A catches up to B's resolution cleanly!
			if err := engA.SyncOnce(context.Background()); err != nil {
				t.Fatalf("device A catch-up after resolution: %v", err)
			}
		})
	}
}

func TestV2IdempotentRetryAfterDroppedReplyNoFalseConflict(t *testing.T) {
	srv := newMemBlobSyncServer()
	scope := conflictcheckpoint.Scope{ServerID: "srv-1", AccountID: "acct-1", WorkspaceID: "ws-1"}
	local := newSQLiteLocalAdapter(t)
	seedHousehold(t, local.db)
	st := newMemStateStore()
	eng, _ := New(Config{Scope: scope, DeviceID: "dev-1"}, srv, &fakeCipher{}, local, st)

	if err := eng.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	changes := captureDelta(t, local.db, func() {
		_ = local.db.PutTransaction(domain.Transaction{
			ID: "tx-retry", AccountID: "acc-1", CategoryID: "cat-1",
			Date: time.Now().UTC(), Amount: money.New(-500, "USD"), Payee: "Retry Cafe", Desc: "Retry Cafe",
		})
	})
	_ = eng.EnqueueRecords("stable-change-id", changes)

	// Server commits the append, then drops the WebSocket before reply arrives!
	srv.dropReply = true
	if err := eng.SyncOnce(context.Background()); err == nil {
		t.Fatal("expected error when reply was dropped")
	}
	if eng.Status().Phase != syncstate.PhaseError || eng.NextRetryDelay() <= 0 {
		t.Fatalf("expected error phase + positive retry delay, got %+v delay=%v", eng.Status(), eng.NextRetryDelay())
	}

	// Retry MUST succeed idempotently with ZERO false conflict and exact same seq!
	if err := eng.SyncOnce(context.Background()); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if eng.Status().Phase != syncstate.PhaseSynced {
		t.Fatalf("retry phase = %q, want synced", eng.Status().Phase)
	}
	if srv.head["ws-1"] != 2 {
		t.Fatalf("server head = %d, want 2 (idempotent dedup)", srv.head["ws-1"])
	}
}

func TestV2LogTruncationBootstrapsFromSnapshot(t *testing.T) {
	srv := newMemBlobSyncServer()
	scope := conflictcheckpoint.Scope{ServerID: "srv-1", AccountID: "acct-1", WorkspaceID: "ws-1"}
	localA := newSQLiteLocalAdapter(t)
	seedHousehold(t, localA.db)
	engA, _ := New(Config{Scope: scope, DeviceID: "dev-a", SnapshotInterval: 2}, srv, &fakeCipher{}, localA, newMemStateStore())
	if err := engA.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	localB := newSQLiteLocalAdapter(t)
	engB, _ := New(Config{Scope: scope, DeviceID: "dev-b"}, srv, &fakeCipher{}, localB, newMemStateStore())
	if err := engB.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Device A makes 3 edits so a compacted snapshot is uploaded, then server
	// prunes older log entries before seq 3.
	for i := 1; i <= 3; i++ {
		delta := captureDelta(t, localA.db, func() {
			_ = localA.db.PutTransaction(domain.Transaction{
				ID: fmt.Sprintf("tx-p-%d", i), AccountID: "acc-1", CategoryID: "cat-1",
				Date: time.Now().UTC(), Amount: money.New(int64(-100*i), "USD"), Payee: fmt.Sprintf("Shop %d", i), Desc: fmt.Sprintf("Shop %d", i),
			})
		})
		_ = engA.EnqueueRecords(fmt.Sprintf("chg-p-%d", i), delta)
		if err := engA.SyncOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	srv.pruneBefore("ws-1", 3)

	// Device B is still at seq 1 -> DetectLogTruncation must detect the gap and
	// seamlessly bootstrap from the compacted snapshot + tail!
	if err := engB.SyncOnce(context.Background()); err != nil {
		t.Fatalf("device B truncated catch-up failed: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if _, ok, _ := localB.db.GetTransaction(fmt.Sprintf("tx-p-%d", i)); !ok {
			t.Fatalf("missing tx-p-%d after truncated snapshot catch-up", i)
		}
	}
}

func TestV2OutboxNeverOverwritesNewerEditOnFailure(t *testing.T) {
	scope := conflictcheckpoint.Scope{ServerID: "s", AccountID: "a", WorkspaceID: "w"}
	outbox := []OutboxEntry{
		{ChangeID: "h1", ServerID: "s", AccountID: "a", WorkspaceID: "w", Kind: BatchKindRecords, Changes: []store.RecordChange{{Table: "transactions", ID: "1", After: []byte(`{"id":"1"}`)}}},
	}
	// Concurrent local edit arrives while h1 is in flight.
	outbox = EnqueueOutbox(outbox, OutboxEntry{
		ChangeID: "h2", ServerID: "s", AccountID: "a", WorkspaceID: "w", Kind: BatchKindRecords, Changes: []store.RecordChange{{Table: "transactions", ID: "2", After: []byte(`{"id":"2"}`)}},
	})
	// h1 fails -> MarkOutboxAttempt must keep h2 intact!
	outbox = MarkOutboxAttempt(outbox, map[string]bool{"h1": true}, "timeout")
	if len(outbox) != 2 || outbox[0].Attempts != 1 || outbox[1].ChangeID != "h2" || outbox[1].Attempts != 0 {
		t.Fatalf("outbox corrupted after h1 failure: %+v", outbox)
	}
	// h1 succeeds later -> RemoveOutboxByIDs leaves h2 intact!
	outbox = RemoveOutboxByIDs(outbox, map[string]bool{"h1": true})
	if len(outbox) != 1 || outbox[0].ChangeID != "h2" {
		t.Fatalf("outbox corrupted after h1 ack: %+v", outbox)
	}
	_ = scope
}

func TestDetectLogTruncation(t *testing.T) {
	cases := []struct {
		name    string
		since   int64
		head    int64
		changes []backendrpc.BlobChange
		want    bool
	}{
		{"caught up", 5, 5, nil, false},
		{"contiguous next", 5, 7, []backendrpc.BlobChange{{Seq: 6}, {Seq: 7}}, false},
		{"gap after since", 2, 7, []backendrpc.BlobChange{{Seq: 5}, {Seq: 6}}, true},
		{"empty page while behind head", 2, 7, nil, true},
		{"fresh device contiguous from 1", 0, 2, []backendrpc.BlobChange{{Seq: 1}, {Seq: 2}}, false},
		{"fresh device seq 1 pruned", 0, 5, []backendrpc.BlobChange{{Seq: 3}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectLogTruncation(tc.since, tc.head, tc.changes); got != tc.want {
				t.Fatalf("DetectLogTruncation(%d, %d, %v) = %v, want %v", tc.since, tc.head, tc.changes, got, tc.want)
			}
		})
	}
}
