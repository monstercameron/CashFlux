// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The server-side half of the 2026-08-19 data-loss guard. The client fix stops
// a foreign dataset being pushed in the first place; this stops the damage even
// when a client is old, buggy, or lying.

// syncDataset builds a dataset payload with the given populations.
func syncDataset(txns, accounts, categories int) []byte {
	var b strings.Builder
	b.WriteString(`{"schemaVersion":1,"transactions":[`)
	for i := 0; i < txns; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"t%d"}`, i)
	}
	b.WriteString(`],"accounts":[`)
	for i := 0; i < accounts; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"a%d"}`, i)
	}
	b.WriteString(`],"categories":[`)
	for i := 0; i < categories; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"c%d"}`, i)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

func newSyncGRPC(t *testing.T) (*SyncService, *Store, context.Context) {
	t.Helper()
	store := openTestStore(t)
	svc := NewSyncService(store)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	seedSyncUser(t, store, "u1", time.Now().UTC())
	return svc, store, ctx
}

// put pushes a dataset through the real RPC entry point, which is where the
// guard lives — not through the store, which would skip it.
func put(t *testing.T, svc *SyncService, ctx context.Context, dataset []byte, at time.Time, force bool) error {
	t.Helper()
	_, err := svc.PutWorkspaceRPC(ctx, backendrpc.PutWorkspaceRequest{
		Workspace:       backendrpc.Workspace{ID: "default", Name: "Default"},
		Dataset:         dataset,
		ClientUpdatedAt: at.Format(time.RFC3339Nano),
		Force:           force,
	})
	return err
}

// The incident: a stale dataset from another account, with a NEWER timestamp,
// replacing a household's records. Last-write-wins accepted it because it only
// compares clocks.
func TestDestructiveOverwriteIsRefused(t *testing.T) {
	svc, store, ctx := newSyncGRPC(t)
	base := time.Now().UTC()

	if err := put(t, svc, ctx, syncDataset(528, 15, 52), base, false); err != nil {
		t.Fatalf("first push: %v", err)
	}

	// The clobber, one minute newer — it would win LWW outright.
	err := put(t, svc, ctx, syncDataset(432, 15, 10), base.Add(time.Minute), false)
	if err == nil {
		t.Fatal("the destructive write was accepted — the guard did not fire")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", status.Code(err))
	}
	if !strings.Contains(err.Error(), "categories") {
		t.Fatalf("the error should say what would be lost: %v", err)
	}

	// And the stored copy is untouched.
	snap, ok, serr := store.GetSnapshotForUser("u1", "default")
	if serr != nil || !ok {
		t.Fatalf("stored snapshot: ok=%v err=%v", ok, serr)
	}
	if !strings.Contains(string(snap.Dataset), `"t527"`) {
		t.Fatal("the stored dataset was modified by a refused write")
	}
}

// Force is the escape hatch: a person who really is deleting most of their
// records confirms it, and the same call comes back with Force set.
func TestDestructiveOverwriteSucceedsWhenForced(t *testing.T) {
	svc, store, ctx := newSyncGRPC(t)
	base := time.Now().UTC()

	if err := put(t, svc, ctx, syncDataset(528, 15, 52), base, false); err != nil {
		t.Fatalf("first push: %v", err)
	}
	if err := put(t, svc, ctx, syncDataset(432, 15, 10), base.Add(time.Minute), true); err != nil {
		t.Fatalf("forced push: %v", err)
	}
	snap, ok, _ := store.GetSnapshotForUser("u1", "default")
	if !ok || strings.Contains(string(snap.Dataset), `"t527"`) {
		t.Fatal("the forced write did not replace the stored copy")
	}
}

// Ordinary syncing must be completely unaffected — a guard that interrupts
// normal use is worse than the bug.
func TestOrdinaryWritesAreUnaffected(t *testing.T) {
	svc, _, ctx := newSyncGRPC(t)
	base := time.Now().UTC()

	for i, tc := range []struct {
		name    string
		dataset []byte
	}{
		{"first ever write", syncDataset(528, 15, 52)},
		{"adding records", syncDataset(565, 15, 58)},
		{"a small deletion", syncDataset(540, 15, 58)},
		{"no change at all", syncDataset(540, 15, 58)},
		{"growing again", syncDataset(700, 16, 60)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := put(t, svc, ctx, tc.dataset, base.Add(time.Duration(i+1)*time.Minute), false); err != nil {
				t.Fatalf("%s was refused: %v", tc.name, err)
			}
		})
	}
}

// A first write has nothing to lose and must never be blocked, however small.
func TestFirstWriteIsNeverGuarded(t *testing.T) {
	svc, _, ctx := newSyncGRPC(t)
	if err := put(t, svc, ctx, syncDataset(1, 1, 1), time.Now().UTC(), false); err != nil {
		t.Fatalf("first write refused: %v", err)
	}
}

// An App-Lock-encrypted payload cannot be inspected by design. The guard must
// stand aside rather than refuse it — otherwise turning on encryption would
// break syncing entirely, which is a worse failure than the one being guarded.
func TestEncryptedPayloadsAreNotBlocked(t *testing.T) {
	svc, _, ctx := newSyncGRPC(t)
	base := time.Now().UTC()

	if err := put(t, svc, ctx, syncDataset(528, 15, 52), base, false); err != nil {
		t.Fatalf("first push: %v", err)
	}
	sealed := append([]byte("\x00cf1\x00"), []byte(`{"v":1,"alg":"AES-GCM-PBKDF2","cipher":"..."}`)...)
	if err := put(t, svc, ctx, sealed, base.Add(time.Minute), false); err != nil {
		t.Fatalf("an encrypted snapshot was refused: %v", err)
	}
}

// Replacing an encrypted stored copy is equally unjudgeable, and equally must
// not be blocked.
func TestWritingOverAnEncryptedStoredCopyIsNotBlocked(t *testing.T) {
	svc, _, ctx := newSyncGRPC(t)
	base := time.Now().UTC()

	sealed := append([]byte("\x00cf1\x00"), []byte(`{"v":1,"alg":"AES-GCM-PBKDF2","cipher":"..."}`)...)
	if err := put(t, svc, ctx, sealed, base, false); err != nil {
		t.Fatalf("first push: %v", err)
	}
	if err := put(t, svc, ctx, syncDataset(1, 1, 1), base.Add(time.Minute), false); err != nil {
		t.Fatalf("refused a write over an unreadable stored copy: %v", err)
	}
}

// One account's data must not influence another's verdict — the guard reads the
// snapshot for the CALLING user, and workspace ids collide across accounts.
func TestGuardIsScopedToTheCallersOwnCopy(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()
	seedSyncUser(t, store, "u1", now)
	seedSyncUser(t, store, "u2", now)

	ctx1 := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	ctx2 := ContextWithAuthUser(context.Background(), AuthUser{ID: "u2"})

	if err := put(t, svc, ctx1, syncDataset(528, 15, 52), now, false); err != nil {
		t.Fatalf("u1 push: %v", err)
	}
	// u2's first write to ITS OWN "default" is small, and must not be judged
	// against u1's large one.
	if err := put(t, svc, ctx2, syncDataset(3, 1, 1), now, false); err != nil {
		t.Fatalf("u2 was blocked by another account's data: %v", err)
	}
}
