// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func migrationEnvelope() []byte {
	return cryptobox.Marshal(cryptobox.Envelope{V: cryptobox.CurrentVersion, Alg: cryptobox.AlgAESGCM, Salt: "c2FsdA==", IV: "aXY=", Cipher: "Y2lwaGVy"})
}

func TestLegacyWorkspaceEncryptionMigration(t *testing.T) {
	svc, store, ctx := newSyncGRPC(t)
	now := time.Now().UTC()
	putCopy := func(dataset []byte, base int64, force bool) (backendrpc.PutWorkspaceResponse, error) {
		return svc.PutWorkspaceRPC(ctx, backendrpc.PutWorkspaceRequest{
			Workspace: backendrpc.Workspace{ID: "default", Name: "Family"}, Dataset: dataset,
			ClientUpdatedAt: now.Format(time.RFC3339Nano), BaseVersion: base, BaseVersionSet: true, Force: force,
		})
	}
	for base := int64(0); base < 2; base++ {
		out, err := putCopy([]byte(`{"schemaVersion":1,"accounts":[]}`), base, false)
		if err != nil || !out.Accepted {
			t.Fatalf("legacy put %d = %+v, %v", base, out, err)
		}
	}
	envelope := migrationEnvelope()
	stale, err := putCopy(envelope, 1, false)
	if err != nil || stale.Accepted {
		t.Fatalf("stale migration = %+v, %v", stale, err)
	}
	user, _ := AuthUserFromContext(ctx)
	history, err := store.SnapshotHistory(user.ID, "default", 0)
	if err != nil || len(history) != 1 {
		t.Fatalf("rejected migration changed history: %+v, %v", history, err)
	}
	out, err := putCopy(envelope, 2, false)
	if err != nil || !out.Accepted || out.Version != 3 {
		t.Fatalf("migration = %+v, %v", out, err)
	}
	got, err := svc.GetWorkspaceRPC(ctx, backendrpc.GetWorkspaceRequest{ID: "default"})
	if err != nil || !bytes.Equal(got.Dataset, envelope) {
		t.Fatalf("encrypted read = %+v, %v", got, err)
	}
	history, err = store.SnapshotHistory(user.ID, "default", 0)
	if err != nil || len(history) != 0 {
		t.Fatalf("plaintext history survived accepted migration: %+v, %v", history, err)
	}
	for _, force := range []bool{false, true} {
		_, err := putCopy([]byte(`{"schemaVersion":1,"accounts":[]}`), 3, force)
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("plaintext downgrade force=%v = %v", force, err)
		}
	}
	w, _, _ := store.GetWorkspace(user.ID, "default")
	if w.Version != 3 {
		t.Fatalf("refused downgrade advanced version: %d", w.Version)
	}
	// Even losing both the current ciphertext and all retained history cannot
	// erase the server-side latch. Simulate a damaged/legacy restored row.
	if _, err := store.db.Exec(`UPDATE snapshots SET dataset_json = ? WHERE user_id = ? AND workspace_id = ?`, []byte(`{"plaintext":true}`), user.ID, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetWorkspaceRPC(ctx, backendrpc.GetWorkspaceRequest{ID: "default"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("served downgraded snapshot: %v", err)
	}
	// The same workspace ID under another account remains legacy-readable.
	seedSyncUser(t, store, "other-migration-user", now)
	other := ContextWithAuthUser(context.Background(), AuthUser{ID: "other-migration-user"})
	otherPut, err := svc.PutWorkspaceRPC(other, backendrpc.PutWorkspaceRequest{Workspace: backendrpc.Workspace{ID: "default", Name: "Other"}, Dataset: []byte(`{"schemaVersion":1}`), BaseVersionSet: true})
	if err != nil || !otherPut.Accepted {
		t.Fatalf("other account inherited marker: %+v, %v", otherPut, err)
	}
	if _, err := svc.GetWorkspaceRPC(other, backendrpc.GetWorkspaceRequest{ID: "default"}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceEncryptionMarkerBackfillsExistingEvidence(t *testing.T) {
	for _, evidence := range []string{"current", "history"} {
		t.Run(evidence, func(t *testing.T) {
			svc, store, ctx := newSyncGRPC(t)
			user, _ := AuthUserFromContext(ctx)
			now := time.Now().UTC()
			if err := store.PutWorkspace(Workspace{ID: "legacy", UserID: user.ID, Name: "Family", UpdatedAt: now}); err != nil {
				t.Fatal(err)
			}
			if err := store.PutSnapshot(Snapshot{UserID: user.ID, WorkspaceID: "legacy", Dataset: migrationEnvelope(), Version: 1, UpdatedAt: now}, 0, 0); err != nil {
				t.Fatal(err)
			}
			if _, err := store.db.Exec(`DELETE FROM server_meta WHERE name = ?`, encryptionMarkerName(user.ID, "legacy")); err != nil {
				t.Fatal(err)
			}
			if evidence == "history" {
				if _, err := store.db.Exec(`INSERT INTO snapshot_history(user_id, workspace_id, dataset_json, version, updated_at) SELECT user_id, workspace_id, dataset_json, version, updated_at FROM snapshots WHERE user_id = ? AND workspace_id = ?`, user.ID, "legacy"); err != nil {
					t.Fatal(err)
				}
				if _, err := store.db.Exec(`UPDATE snapshots SET dataset_json = ? WHERE user_id = ? AND workspace_id = ?`, []byte(`{"legacy":true}`), user.ID, "legacy"); err != nil {
					t.Fatal(err)
				}
			}
			_, err := svc.GetWorkspaceRPC(ctx, backendrpc.GetWorkspaceRequest{ID: "legacy"})
			if evidence == "current" && err != nil || evidence == "history" && status.Code(err) != codes.FailedPrecondition {
				t.Fatalf("backfill read = %v", err)
			}
			var marker string
			if err := store.db.QueryRow(`SELECT value FROM server_meta WHERE name = ?`, encryptionMarkerName(user.ID, "legacy")).Scan(&marker); err != nil {
				t.Fatalf("marker not durable: %v", err)
			}
			err = store.PutSnapshot(Snapshot{UserID: user.ID, WorkspaceID: "legacy", Dataset: []byte(`{}`), Version: 2, UpdatedAt: now.Add(-time.Hour)}, 0, 0)
			if !errors.Is(err, errWorkspaceEncryptionDowngrade) {
				t.Fatalf("direct snapshot downgrade = %v", err)
			}
		})
	}
}
