//go:build soak

// SPDX-License-Identifier: MIT

// Package server — soak tests for the sync path.
//
// Run explicitly with:
//
//	go test -tags soak ./internal/server/... -run Soak -v -timeout 5m
//
// These tests are excluded from normal "go test ./..." runs by the "soak" build
// tag so CI stays fast.
package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"net/http/httptest"
)

// TestSoakSyncPushPull hammers the push (PutWorkspace) path with many
// concurrent goroutines and verifies that every accepted write is subsequently
// readable via ListWorkspaces.
func TestSoakSyncPushPull(t *testing.T) {
	const (
		workers        = 32
		writesPerWorker = 10
	)

	store := openTestStore(t)
	cfg := Config{
		AuthMode:     "token",
		Token:        "dev-token",
		AppOrigin:    "*",
		DataDir:      t.TempDir(),
		BlobMaxBytes: 1 << 20,
		Metrics:      NewMetrics(),
	}
	srv := httptest.NewServer(NewMux(cfg, store))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var (
		accepted atomic.Int64
		rejected atomic.Int64
		errCount atomic.Int64
	)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := syncbridge.Dial(ctx, syncbridge.Config{
				ServerURL: srv.URL,
				Token:     "dev-token",
			})
			if err != nil {
				t.Errorf("worker %d: dial: %v", w, err)
				errCount.Add(1)
				return
			}
			defer conn.Close()

			for i := 0; i < writesPerWorker; i++ {
				wsID := fmt.Sprintf("soak-w%02d-i%02d", w, i)
				var out backendrpc.PutWorkspaceResponse
				err := conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
					Workspace: backendrpc.Workspace{
						ID:       wsID,
						Name:     fmt.Sprintf("Soak worker %d run %d", w, i),
						DeviceID: fmt.Sprintf("device-%02d", w),
					},
					Dataset:         []byte(fmt.Sprintf(`{"soak":true,"w":%d,"i":%d}`, w, i)),
					ClientUpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
				}, &out, backendrpc.JSONCallOptions()...)
				if err != nil {
					t.Errorf("worker %d write %d: %v", w, i, err)
					errCount.Add(1)
					continue
				}
				if out.Accepted {
					accepted.Add(1)
				} else {
					rejected.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if errCount.Load() > 0 {
		t.Fatalf("soak push: %d errors", errCount.Load())
	}
	t.Logf("push/pull soak: accepted=%d rejected=%d errors=%d",
		accepted.Load(), rejected.Load(), errCount.Load())

	// All writes use unique workspace IDs per worker-iteration so every one
	// should be accepted (no LWW conflict possible).
	want := int64(workers * writesPerWorker)
	if got := accepted.Load(); got != want {
		t.Errorf("accepted = %d, want %d", got, want)
	}

	// Pull: list all workspaces and verify count.
	readConn, err := syncbridge.Dial(ctx, syncbridge.Config{
		ServerURL: srv.URL,
		Token:     "dev-token",
	})
	if err != nil {
		t.Fatalf("list dial: %v", err)
	}
	defer readConn.Close()

	var list backendrpc.ListWorkspacesResponse
	if err := readConn.Invoke(ctx, backendrpc.MethodSyncListWorkspaces,
		backendrpc.ListWorkspacesRequest{}, &list, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := int64(len(list.Workspaces)); got != want {
		t.Errorf("list count = %d, want %d", got, want)
	}
}

// TestSoakSyncConflictFanout exercises the LWW conflict path: multiple workers
// race to update the same workspace with different payloads. Exactly one write
// per round wins (Accepted=true); the rest are rejected (Accepted=false, the
// LWW loser). After all rounds, the workspace must be readable and consistent.
func TestSoakSyncConflictFanout(t *testing.T) {
	const (
		workers = 16
		rounds  = 20
	)

	store := openTestStore(t)
	cfg := Config{
		AuthMode:     "token",
		Token:        "dev-token",
		AppOrigin:    "*",
		DataDir:      t.TempDir(),
		BlobMaxBytes: 1 << 20,
		Metrics:      NewMetrics(),
	}
	srv := httptest.NewServer(NewMux(cfg, store))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const wsID = "soak-conflict-ws"

	// Seed the workspace first so every round is an update (not a create).
	seedConn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: srv.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("seed dial: %v", err)
	}
	var seedOut backendrpc.PutWorkspaceResponse
	if err := seedConn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
		Workspace:       backendrpc.Workspace{ID: wsID, Name: "Seed", DeviceID: "seed"},
		Dataset:         []byte(`{"seed":true}`),
		ClientUpdatedAt: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano),
	}, &seedOut, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("seed: %v", err)
	}
	seedConn.Close()

	var totalAccepted atomic.Int64
	var totalRejected atomic.Int64
	var errCount atomic.Int64

	for round := 0; round < rounds; round++ {
		// All workers race to update the same workspace at roughly the same time.
		// Spread timestamps so the server can order them; the highest one wins.
		base := time.Now().UTC()

		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			w := w
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := syncbridge.Dial(ctx, syncbridge.Config{
					ServerURL: srv.URL,
					Token:     "dev-token",
				})
				if err != nil {
					errCount.Add(1)
					return
				}
				defer conn.Close()

				ts := base.Add(time.Duration(w) * time.Millisecond)
				var out backendrpc.PutWorkspaceResponse
				err = conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
					Workspace: backendrpc.Workspace{
						ID:       wsID,
						Name:     fmt.Sprintf("round %d worker %d", round, w),
						DeviceID: fmt.Sprintf("device-%02d", w),
					},
					Dataset:         []byte(fmt.Sprintf(`{"round":%d,"worker":%d}`, round, w)),
					ClientUpdatedAt: ts.Format(time.RFC3339Nano),
				}, &out, backendrpc.JSONCallOptions()...)
				if err != nil {
					errCount.Add(1)
					return
				}
				if out.Accepted {
					totalAccepted.Add(1)
				} else {
					totalRejected.Add(1)
				}
			}()
		}
		wg.Wait()
	}

	if errCount.Load() > 0 {
		t.Fatalf("conflict fanout: %d RPC errors", errCount.Load())
	}

	total := int64(workers * rounds)
	if got := totalAccepted.Load() + totalRejected.Load(); got != total {
		t.Errorf("accepted+rejected = %d, want %d", got, total)
	}
	// At least one write per round should be accepted.
	if totalAccepted.Load() < int64(rounds) {
		t.Errorf("accepted = %d, want at least %d (one per round)", totalAccepted.Load(), rounds)
	}
	t.Logf("conflict fanout soak: accepted=%d rejected=%d errors=%d",
		totalAccepted.Load(), totalRejected.Load(), errCount.Load())

	// Workspace must still be readable.
	readConn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: srv.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("read dial: %v", err)
	}
	defer readConn.Close()
	var getResp backendrpc.GetWorkspaceResponse
	if err := readConn.Invoke(ctx, backendrpc.MethodSyncGetWorkspace,
		backendrpc.GetWorkspaceRequest{WorkspaceID: wsID}, &getResp, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	if getResp.Workspace.ID != wsID {
		t.Errorf("workspace ID = %q, want %q", getResp.Workspace.ID, wsID)
	}
}
