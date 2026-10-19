// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConcurrentSameBaseWorkspaceWritesAcceptExactlyOne(t *testing.T) {
	for iteration := 0; iteration < 50; iteration++ {
		store := openTestStore(t)
		svc := NewSyncService(store)
		now := time.Now().UTC()
		seedSyncUser(t, store, "u1", now)
		ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
		if _, err := svc.PutWorkspace(ctx, Workspace{ID: "shared", Name: "initial"}, now, false, now, 0); err != nil {
			t.Fatalf("iteration %d seed: %v", iteration, err)
		}

		const writers = 16
		start := make(chan struct{})
		type outcome struct {
			result PutWorkspaceResult
			err    error
		}
		outcomes := make(chan outcome, writers)
		var wg sync.WaitGroup
		for i := 0; i < writers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				result, err := svc.PutWorkspace(ctx, Workspace{ID: "shared", Name: fmt.Sprintf("writer-%02d", i)}, now, false, now.Add(time.Duration(i+1)*time.Second), 1)
				outcomes <- outcome{result: result, err: err}
			}(i)
		}
		close(start)
		wg.Wait()
		close(outcomes)
		accepted, rejected := 0, 0
		for got := range outcomes {
			if got.err != nil {
				t.Fatalf("iteration %d write errored: %v", iteration, got.err)
			}
			if got.result.Accepted {
				accepted++
				continue
			}
			rejected++
			if got.result.Version != 2 || got.result.Workspace.Version != 2 {
				t.Fatalf("iteration %d rejection omitted current version: %+v", iteration, got.result)
			}
			if got.result.Reason != "version-stale" {
				t.Fatalf("iteration %d rejection reason = %q", iteration, got.result.Reason)
			}
		}
		if accepted != 1 || rejected != writers-1 {
			t.Fatalf("iteration %d accepted/rejected = %d/%d, want 1/%d", iteration, accepted, rejected, writers-1)
		}
	}
}

func TestConcurrentWorkspaceCreatesAcceptExactlyOne(t *testing.T) {
	for iteration := 0; iteration < 50; iteration++ {
		store := openTestStore(t)
		now := time.Now().UTC()
		seedSyncUser(t, store, "u1", now)
		const writers = 2
		start := make(chan struct{})
		results := make(chan PutWorkspaceResult, writers)
		errors := make(chan error, writers)
		var wg sync.WaitGroup
		for i := 0; i < writers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				result, err := store.PutWorkspaceAtomic(AtomicPutWorkspaceInput{
					Workspace:   Workspace{ID: "new", UserID: "u1", Name: fmt.Sprintf("creator-%d", i), UpdatedAt: now},
					BaseVersion: 0, HasBaseVersion: true, ClientUpdatedAt: now,
				})
				results <- PutWorkspaceResult{Accepted: result.Accepted, Workspace: result.Workspace, Version: result.Workspace.Version, Reason: result.Reason}
				errors <- err
			}(i)
		}
		close(start)
		wg.Wait()
		close(results)
		close(errors)
		accepted, rejected := 0, 0
		for result := range results {
			if result.Accepted {
				accepted++
			} else {
				rejected++
				if result.Version != 1 || result.Reason != "version-stale" {
					t.Fatalf("iteration %d create rejection = %+v", iteration, result)
				}
			}
		}
		for err := range errors {
			if err != nil {
				t.Fatalf("iteration %d create errored: %v", iteration, err)
			}
		}
		if accepted != 1 || rejected != 1 {
			t.Fatalf("iteration %d create accepted/rejected = %d/%d, want 1/1", iteration, accepted, rejected)
		}
	}
}
