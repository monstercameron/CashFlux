// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func blobSyncContext(user string) context.Context {
	return ContextWithAuthUser(context.Background(), AuthUser{ID: user})
}

func TestBlobSyncConcurrentAppendIdempotentAndGapFree(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	svc := NewBlobSyncService(s, Config{})
	const workers, perWorker = 32, 50
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]int64{}
	errs := make(chan error, workers*perWorker*2)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(worker + 1)))
			changes := make([]backendrpc.BlobChange, perWorker)
			for n := 0; n < perWorker; n++ {
				id := fmt.Sprintf("%02d-%03d", worker, n)
				changes[n] = backendrpc.BlobChange{ChangeID: id, DeviceID: fmt.Sprintf("d%d", worker), Blob: []byte("ciphertext:" + id)}
			}
			resp, e := svc.AppendChanges(blobSyncContext("u"), backendrpc.AppendChangesRequest{Workspace: "default", Changes: changes})
			if e != nil {
				errs <- e
				return
			}
			for n, change := range changes {
				seq := resp.Seqs[n]
				mu.Lock()
				seen[change.ChangeID] = seq
				mu.Unlock()
				if r.Intn(2) == 0 {
					retry, e := svc.AppendChanges(blobSyncContext("u"), backendrpc.AppendChangesRequest{Workspace: "default", Changes: []backendrpc.BlobChange{change}})
					if e != nil {
						errs <- e
					} else if retry.Seqs[0] != seq {
						errs <- fmt.Errorf("retry %s returned seq %d, original %d", change.ChangeID, retry.Seqs[0], seq)
					}
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	const total = workers * perWorker
	if len(seen) != total {
		t.Fatalf("unique ids=%d want %d", len(seen), total)
	}
	rows, err := s.db.Query(`SELECT seq,change_id,blob FROM change_log WHERE user_id='u' AND workspace_id='default' ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var seq int64
	for rows.Next() {
		var got int64
		var id string
		var blob []byte
		if err := rows.Scan(&got, &id, &blob); err != nil {
			t.Fatal(err)
		}
		seq++
		if got != seq {
			t.Fatalf("seq=%d, expected gap-free %d", got, seq)
		}
		if seen[id] != got {
			t.Fatalf("stored seq for %q=%d, returned=%d", id, got, seen[id])
		}
		if string(blob) != "ciphertext:"+id {
			t.Fatalf("blob changed for %q: %q", id, blob)
		}
	}
	if seq != total {
		t.Fatalf("stored rows=%d want %d", seq, total)
	}
}

func TestBlobSyncTenantIsolationAndOpaqueBytes(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	svc := NewBlobSyncService(s, Config{})
	ctxA, ctxB := blobSyncContext("user-a"), blobSyncContext("user-b")
	opaque := []byte{0, 255, 1, 128, 2}
	a, err := svc.AppendChanges(ctxA, backendrpc.AppendChangesRequest{Workspace: "same", Changes: []backendrpc.BlobChange{{ChangeID: "c1", DeviceID: "d1", Blob: opaque}}})
	if err != nil {
		t.Fatal(err)
	}
	if a.Seqs[0] != 1 {
		t.Fatalf("seq=%v", a.Seqs)
	}
	b, err := svc.ListChanges(ctxB, backendrpc.ListChangesRequest{Workspace: "same", Since: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Changes) != 0 {
		t.Fatal("user B listed user A's changes")
	}
	foreignSnapshot, err := svc.PutSnapshot(ctxB, backendrpc.PutSnapshotRequest{Workspace: "same", CoversSeq: 1, Blob: []byte("wrong tenant")})
	if err != nil || foreignSnapshot.Accepted {
		t.Fatalf("user B snapshotted user A's workspace: %+v err=%v", foreignSnapshot, err)
	}
	if _, err := svc.AppendChanges(ctxB, backendrpc.AppendChangesRequest{Workspace: "same", Changes: []backendrpc.BlobChange{{ChangeID: "c1", DeviceID: "db", Blob: []byte("B")}}}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.db.QueryContext(ctxA, `SELECT blob FROM change_log WHERE user_id=? AND workspace_id=? AND seq=1`, "user-a", "same")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		_ = rows.Close()
		t.Fatal("missing user A row")
	}
	var stored []byte
	if err := rows.Scan(&stored); err != nil {
		_ = rows.Close()
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(opaque) {
		t.Fatalf("stored ciphertext changed: %v", stored)
	}
	other, err := svc.ListChanges(ctxA, backendrpc.ListChangesRequest{Workspace: "same", Since: 0})
	if err != nil || len(other.Changes) != 1 || string(other.Changes[0].Blob) != string(opaque) {
		t.Fatalf("user B append affected user A: %+v err=%v", other, err)
	}
	if _, err := svc.PutSnapshot(ctxA, backendrpc.PutSnapshotRequest{Workspace: "same", CoversSeq: 1, Blob: opaque}); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.GetSnapshot(ctxB, backendrpc.GetSnapshotRequest{Workspace: "same"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Found {
		t.Fatal("user B read user A snapshot")
	}
	key, err := svc.GetKeyring(ctxB, backendrpc.GetKeyringRequest{Workspace: "same"})
	if err != nil {
		t.Fatal(err)
	}
	if key.Found {
		t.Fatal("user B read user A keyring")
	}
	cas, err := svc.PutKeyring(ctxA, backendrpc.PutKeyringRequest{Workspace: "same", ExpectedVersion: 0, Blob: opaque})
	if err != nil || !cas.Updated || cas.Version != 1 {
		t.Fatalf("keyring CAS=%+v err=%v", cas, err)
	}
	cas, err = svc.PutKeyring(ctxB, backendrpc.PutKeyringRequest{Workspace: "same", ExpectedVersion: 0, Blob: []byte("independent tenant keyring")})
	if err != nil || !cas.Updated || cas.Version != 1 {
		t.Fatalf("independent tenant keyring=%+v err=%v", cas, err)
	}
	if _, err := svc.DeleteWorkspace(ctxA, backendrpc.BlobSyncDeleteWorkspaceRequest{Workspace: "same"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendChanges(ctxA, backendrpc.AppendChangesRequest{Workspace: "same", Changes: []backendrpc.BlobChange{{ChangeID: "after-delete", DeviceID: "d1", Blob: opaque}}}); err == nil {
		t.Fatal("append after tombstone succeeded")
	}
}

func TestBlobSyncSnapshotValidationAndKeyringCAS(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	svc := NewBlobSyncService(s, Config{})
	ctx := blobSyncContext("u")
	if _, err := svc.PutSnapshot(ctx, backendrpc.PutSnapshotRequest{Workspace: "w", CoversSeq: 0, Blob: []byte("x")}); err == nil {
		t.Fatal("accepted zero snapshot cursor")
	}
	_, err = svc.AppendChanges(ctx, backendrpc.AppendChangesRequest{Workspace: "w", Changes: []backendrpc.BlobChange{{ChangeID: "one", DeviceID: "d", Blob: []byte("x")}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, covers := range []int64{2} {
		got, err := svc.PutSnapshot(ctx, backendrpc.PutSnapshotRequest{Workspace: "w", CoversSeq: covers, Blob: []byte("snapshot")})
		if err != nil {
			t.Fatal(err)
		}
		if got.Accepted {
			t.Fatalf("accepted invalid/nonadvancing snapshot cursor %d", covers)
		}
	}
	got, err := svc.PutSnapshot(ctx, backendrpc.PutSnapshotRequest{Workspace: "w", CoversSeq: 1, Blob: []byte("snapshot")})
	if err != nil || !got.Accepted {
		t.Fatalf("valid snapshot=%+v err=%v", got, err)
	}
	first, err := svc.PutKeyring(ctx, backendrpc.PutKeyringRequest{Workspace: "w", ExpectedVersion: 0, Blob: []byte("key")})
	if err != nil || !first.Updated || first.Version != 1 {
		t.Fatalf("first CAS=%+v err=%v", first, err)
	}
	stale, err := svc.PutKeyring(ctx, backendrpc.PutKeyringRequest{Workspace: "w", ExpectedVersion: 0, Blob: []byte("overwrite")})
	if err != nil || stale.Updated || stale.Version != 1 {
		t.Fatalf("stale CAS=%+v err=%v", stale, err)
	}
}

func TestBlobSyncPairingRelayExpiresAndIsTenantScoped(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	svc := NewBlobSyncService(s, Config{})
	ctxA, ctxB := blobSyncContext("a"), blobSyncContext("b")
	if _, err := svc.CreatePairing(ctxA, backendrpc.CreatePairingRequest{ID: "relay", Blob: []byte("opaque"), TTLSeconds: 30}); err != nil {
		t.Fatal(err)
	}
	other, err := svc.ClaimPairing(ctxB, backendrpc.ClaimPairingRequest{ID: "relay"})
	if err != nil || other.Found {
		t.Fatalf("cross-user claim=%+v err=%v", other, err)
	}
	if _, err := svc.CreatePairing(ctxB, backendrpc.CreatePairingRequest{ID: "relay", Blob: []byte("user b relay"), TTLSeconds: 30}); err != nil {
		t.Fatalf("same pairing id in separate tenant: %v", err)
	}
	claimed, err := svc.ClaimPairing(ctxA, backendrpc.ClaimPairingRequest{ID: "relay"})
	if err != nil || !claimed.Found || string(claimed.Blob) != "opaque" {
		t.Fatalf("claim=%+v err=%v", claimed, err)
	}
	if err := s.sweepPairingRelay(context.Background(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestBlobSyncCreatePairingRejectsEmptyAndOversizedEnvelope(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	svc := NewBlobSyncService(s, Config{BlobSyncPairingMaxBytes: 4})
	ctx := blobSyncContext("u")

	if _, err := svc.CreatePairing(ctx, backendrpc.CreatePairingRequest{ID: "empty"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty envelope error=%v, want InvalidArgument", err)
	}
	if _, err := svc.CreatePairing(ctx, backendrpc.CreatePairingRequest{ID: "oversized", Blob: []byte("12345")}); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized envelope error=%v, want ResourceExhausted", err)
	}
	var stored int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pairing_relay`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatalf("invalid pairing requests stored %d envelopes", stored)
	}
}

func TestBlobSyncPairingDefaultEnvelopeLimit(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	svc := NewBlobSyncService(s, Config{})
	_, err = svc.CreatePairing(blobSyncContext("u"), backendrpc.CreatePairingRequest{ID: "too-large", Blob: make([]byte, defaultBlobSyncPairingMaxBytes+1)})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("default oversized envelope error=%v, want ResourceExhausted", err)
	}
}

func TestBlobSyncRateLimitDefaultsAndRejectsExcess(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	svc := NewBlobSyncService(s, Config{BlobSyncRatePerSecond: 1, BlobSyncRateBurst: 1})
	ctx := blobSyncContext("u")
	for i := 0; i < 2; i++ {
		_, err := svc.ListChanges(ctx, backendrpc.ListChangesRequest{Workspace: "w"})
		if i == 0 && err != nil {
			t.Fatal(err)
		}
		if i == 1 && err == nil {
			t.Fatal("rate limit did not reject second request")
		}
	}
}

func TestBlobSyncRateLimitEvictsOnlyFullyRefilledIdleBuckets(t *testing.T) {
	svc := NewBlobSyncService(nil, Config{BlobSyncRatePerSecond: 1, BlobSyncRateBurst: 1})
	if err := svc.allow("active"); err != nil {
		t.Fatal(err)
	}
	if err := svc.allow("active"); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("second active call error=%v, want ResourceExhausted", err)
	}
	svc.mu.Lock()
	svc.buckets["idle"] = blobSyncBucket{tokens: 0, at: time.Now().Add(-2 * blobSyncBucketIdleWindow(1, 1))}
	svc.mu.Unlock()

	if err := svc.allow("new-user"); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	_, idleStillPresent := svc.buckets["idle"]
	_, activeStillPresent := svc.buckets["active"]
	svc.mu.Unlock()
	if idleStillPresent {
		t.Fatal("idle token bucket was not evicted")
	}
	if !activeStillPresent {
		t.Fatal("active token bucket was evicted")
	}
	if err := svc.allow("active"); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("active user's next call error=%v, want ResourceExhausted", err)
	}
}

func TestBlobSyncRateLimitBucketCapPreservesExistingLimits(t *testing.T) {
	svc := NewBlobSyncService(nil, Config{BlobSyncRatePerSecond: 1, BlobSyncRateBurst: 1})
	if err := svc.allow("active"); err != nil {
		t.Fatal(err)
	}
	if err := svc.allow("active"); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("second active call error=%v, want ResourceExhausted", err)
	}
	svc.mu.Lock()
	for i := 0; i < maxBlobSyncRateBuckets-1; i++ {
		svc.buckets[fmt.Sprintf("user-%d", i)] = blobSyncBucket{tokens: 1, at: time.Now()}
	}
	svc.mu.Unlock()

	if err := svc.allow("new-user"); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("new user at bucket cap error=%v, want ResourceExhausted", err)
	}
	if err := svc.allow("active"); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("active user's next call error=%v, want ResourceExhausted", err)
	}
}

func TestMigrateV18ToV19AndDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v18.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"change_log", "ws_head", "ws_snapshot", "keyring", "pairing_relay", "server_meta"} {
		if _, err := s.db.Exec(`DROP TABLE ` + table); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`UPDATE schema_meta SET version=18 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.SchemaVersion()
	if err != nil || v != CurrentServerSchemaVersion {
		t.Fatalf("migrated version=%d err=%v", v, err)
	}
	for _, table := range []string{"change_log", "ws_head", "ws_snapshot", "keyring", "pairing_relay", "server_meta"} {
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&exists); err != nil {
			t.Fatalf("check v19 table %q: %v", table, err)
		}
		if exists != 1 {
			t.Errorf("v19 table %q missing after migration", table)
		}
	}
	id1, err := s.ServerID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.ServerID(context.Background())
	if err != nil || id1 != id2 || len(id1) != 36 {
		t.Fatalf("server id %q then %q, err=%v", id1, id2, err)
	}
	_ = s.Close()
	dry, err := DryRunStoreMigrations(path)
	if err != nil || dry != CurrentServerSchemaVersion {
		t.Fatalf("migrate-check dry run version=%d err=%v", dry, err)
	}
}
