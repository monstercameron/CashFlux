// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The bug, at the service level: a device whose clock trails the server's used
// to lose every push, because the rule compared the CLIENT's wall clock with a
// stamp this server wrote itself. These tests drive PutWorkspace directly with a
// deliberately slow client clock.
func TestPutWorkspaceAcceptsACurrentEditFromASlowClock(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	serverNow := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", serverNow)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})

	first, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Home"}, serverNow, false, serverNow, 0)
	if err != nil || !first.Accepted || first.Version != 1 {
		t.Fatalf("seed push = %+v/%v", first, err)
	}

	// The device edits again. Its clock reads a full minute BEHIND the stamp the
	// server just wrote, which is ordinary drift — but it holds version 1, which
	// is current, so the edit is a valid successor.
	slow := serverNow.Add(-time.Minute)
	got, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Renamed"}, slow, false, serverNow.Add(time.Second), 1)
	if err != nil {
		t.Fatalf("second push errored: %v", err)
	}
	if !got.Accepted {
		t.Fatalf("a current edit was rejected for clock skew alone: %+v", got)
	}
	if got.Version != 2 {
		t.Errorf("Version = %d, want 2", got.Version)
	}
	// And prove the fixture really is skewed, or this test proves nothing.
	if !slow.Before(first.UpdatedAt) {
		t.Fatal("fixture no longer reproduces clock skew")
	}
}

func TestPutWorkspaceStillRejectsAnEditBasedOnASupersededVersion(t *testing.T) {
	// The protection that has to survive: device B landed a write after device A
	// read, so A's push would clobber it — even though A's clock says it is newer.
	store := openTestStore(t)
	svc := NewSyncService(store)
	serverNow := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", serverNow)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})

	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "A"}, serverNow, false, serverNow, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Device B pushes, taking the row to version 2.
	b, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "B"}, serverNow, false, serverNow.Add(time.Second), 1)
	if err != nil || !b.Accepted || b.Version != 2 {
		t.Fatalf("device B push = %+v/%v", b, err)
	}
	// Device A still thinks it is on version 1, and its clock runs an hour fast.
	a, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "A again"},
		serverNow.Add(time.Hour), false, serverNow.Add(2*time.Second), 1)
	if err != nil {
		t.Fatalf("device A push errored: %v", err)
	}
	if a.Accepted {
		t.Fatal("a push based on a superseded version was accepted — device B's write is gone")
	}
	if a.Reason != "version-stale" {
		t.Errorf("Reason = %q, want version-stale", a.Reason)
	}
	// The rejection hands back what the server actually holds, so the client can
	// resync rather than guess.
	if a.Workspace.Name != "B" || a.Version != 2 {
		t.Errorf("rejection did not report current state: %+v", a)
	}
}

func TestPutWorkspaceLegacyClientIsUnaffected(t *testing.T) {
	// A build that cannot send a base version must keep syncing exactly as it did.
	// Passing 0 selects the old timestamp rule.
	store := openTestStore(t)
	svc := NewSyncService(store)
	serverNow := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", serverNow)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})

	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "A"}, serverNow, false, serverNow, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	old, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Older"}, serverNow.Add(-time.Hour), false, serverNow.Add(time.Second), 0)
	if err != nil || old.Accepted {
		t.Fatalf("legacy older push = %+v/%v, want reject", old, err)
	}
	if old.Reason != "clock-older" {
		t.Errorf("Reason = %q, want clock-older", old.Reason)
	}
	newer, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Newer"}, serverNow.Add(time.Hour), false, serverNow.Add(2*time.Second), 0)
	if err != nil || !newer.Accepted {
		t.Fatalf("legacy newer push = %+v/%v, want accept", newer, err)
	}
}

func TestPutWorkspaceForceStillOverwrites(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	serverNow := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", serverNow)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})

	if _, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "A"}, serverNow, false, serverNow, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Hopelessly stale on both axes, but the user chose it.
	got, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Forced"},
		serverNow.Add(-24*time.Hour), true, serverNow.Add(time.Second), 1)
	if err != nil || !got.Accepted || got.Workspace.Name != "Forced" {
		t.Fatalf("forced push = %+v/%v", got, err)
	}
}

func TestVersionAdvancesOncePerAcceptedWrite(t *testing.T) {
	// The counter the whole scheme rests on. A rejected push must NOT advance it,
	// or every rejection would invalidate the version its own client just learned.
	store := openTestStore(t)
	svc := NewSyncService(store)
	serverNow := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", serverNow)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})

	base := 0
	for i := 1; i <= 5; i++ {
		got, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "n"},
			serverNow, false, serverNow.Add(time.Duration(i)*time.Second), int64(base))
		if err != nil || !got.Accepted {
			t.Fatalf("push %d = %+v/%v", i, got, err)
		}
		if got.Version != int64(i) {
			t.Fatalf("push %d gave version %d, want %d", i, got.Version, i)
		}
		base = int(got.Version)
	}
	// A rejected push leaves the counter alone.
	rej, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "stale"}, serverNow, false, serverNow.Add(time.Minute), 2)
	if err != nil || rej.Accepted {
		t.Fatalf("expected rejection, got %+v/%v", rej, err)
	}
	if rej.Version != 5 {
		t.Errorf("a rejected push moved the version to %d", rej.Version)
	}
}

func TestDeleteBumpsVersionAndRejectsQueuedPush(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	seedAt := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", seedAt)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	created, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Home"}, seedAt, false, seedAt, 0)
	if err != nil || !created.Accepted {
		t.Fatalf("create = %+v/%v", created, err)
	}
	if deleted, err := svc.Delete(ctx, "w1", seedAt.Add(-time.Hour), "device"); err != nil || !deleted {
		t.Fatalf("delete = %v/%v", deleted, err)
	}
	tombstone, ok, err := store.GetWorkspace("u1", "w1")
	if err != nil || !ok {
		t.Fatalf("read tombstone = %v/%v", ok, err)
	}
	if !tombstone.Deleted || tombstone.Version != created.Version+1 {
		t.Fatalf("tombstone = %+v, want deleted at version %d", tombstone, created.Version+1)
	}
	if tombstone.UpdatedAt.Before(time.Now().UTC().Add(-time.Minute)) {
		t.Fatalf("tombstone retained a client timestamp: %v", tombstone.UpdatedAt)
	}
	queued, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "resurrected"}, seedAt.Add(24*time.Hour), false, seedAt.Add(24*time.Hour), created.Version)
	if err != nil {
		t.Fatalf("queued push: %v", err)
	}
	if queued.Accepted || queued.Reason != "deleted" || !queued.Workspace.Deleted || queued.Version != tombstone.Version {
		t.Fatalf("queued push result = %+v, want deleted rejection with tombstone", queued)
	}
}

func TestOversizedAtomicDatasetDoesNotAdvanceWorkspaceVersion(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()
	seedSyncUser(t, store, "u1", now)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	created, err := svc.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Home"}, now, false, now, 0)
	if err != nil || !created.Accepted {
		t.Fatalf("create = %+v/%v", created, err)
	}
	_, err = store.PutWorkspaceAtomic(AtomicPutWorkspaceInput{
		Workspace: Workspace{ID: "w1", UserID: "u1", Name: "too large", UpdatedAt: now.Add(time.Second)},
		Dataset:   make([]byte, 8), MaxDatasetBytes: 7, BaseVersion: created.Version, HasBaseVersion: true,
	})
	if !errors.Is(err, errPayloadTooLarge) {
		t.Fatalf("oversized write error = %v, want payload too large", err)
	}
	current, ok, err := store.GetWorkspace("u1", "w1")
	if err != nil || !ok {
		t.Fatalf("read workspace = %v/%v", ok, err)
	}
	if current.Version != created.Version || current.Name != "Home" {
		t.Fatalf("oversized write changed workspace: %+v", current)
	}
}
