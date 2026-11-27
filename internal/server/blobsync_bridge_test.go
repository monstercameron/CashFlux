// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/backendrpc/grpccodec"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBlobSyncRegisteredOnGRPCBridgeWithWatchAndLargeSnapshot(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "v2bridge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	cfg := Config{
		AuthMode:  "token",
		Token:     "v2-secret-token",
		AppOrigin: "*",
	}

	srv := httptest.NewServer(NewGRPCBridgeHandler(cfg, st))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := syncbridge.Dial(ctx, syncbridge.Config{
		ServerURL: srv.URL,
		Token:     cfg.Token,
	})
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer conn.Close()

	// Open event-driven v2 Watch stream BEFORE appending so we verify instant
	// push wakeup (no 1-second polling lag).
	watchStream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, backendrpc.MethodBlobSyncWatch, grpccodec.CallOptions()...)
	if err != nil {
		t.Fatalf("open v2 watch: %v", err)
	}
	if err := watchStream.SendMsg(&backendrpc.WatchRequest{Workspace: "ws-1", Since: 0}); err != nil {
		t.Fatalf("send watch req: %v", err)
	}
	if err := watchStream.CloseSend(); err != nil {
		t.Fatalf("close send watch: %v", err)
	}

	// Give server goroutine a brief moment to register subscriber before append.
	time.Sleep(50 * time.Millisecond)

	var appendResp backendrpc.AppendChangesResponse
	err = conn.Invoke(ctx, backendrpc.MethodBlobSyncAppendChanges, backendrpc.AppendChangesRequest{
		Workspace: "ws-1",
		Changes: []backendrpc.BlobChange{
			{ChangeID: "chg-1", DeviceID: "dev-1", Blob: []byte("opaque-change-1")},
		},
	}, &appendResp, grpccodec.CallOptions()...)
	if err != nil {
		t.Fatalf("AppendChanges over bridge: %v", err)
	}
	if len(appendResp.Seqs) != 1 || appendResp.Seqs[0] != 1 {
		t.Fatalf("AppendChanges seqs = %v, want [1]", appendResp.Seqs)
	}

	// Watch must deliver head=1 promptly (well under the old 1s poll timer).
	evCh := make(chan backendrpc.WatchEvent, 1)
	errCh := make(chan error, 1)
	go func() {
		var ev backendrpc.WatchEvent
		if err := watchStream.RecvMsg(&ev); err != nil {
			errCh <- err
			return
		}
		evCh <- ev
	}()
	select {
	case ev := <-evCh:
		if ev.Workspace != "ws-1" || ev.Head != 1 {
			t.Fatalf("watch event = %+v, want ws-1 head=1", ev)
		}
	case err := <-errCh:
		t.Fatalf("watch recv err: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("watch event timed out")
	}

	// Verify >4 MiB snapshot succeeds end-to-end (previously hit gRPC 4 MiB default).
	bigSnap := bytes.Repeat([]byte("A"), 5<<20)
	var putSnapResp backendrpc.PutSnapshotResponse
	err = conn.Invoke(ctx, backendrpc.MethodBlobSyncPutSnapshot, backendrpc.PutSnapshotRequest{
		Workspace: "ws-1",
		CoversSeq: 1,
		Blob:      bigSnap,
	}, &putSnapResp, grpccodec.CallOptions()...)
	if err != nil {
		t.Fatalf("PutSnapshot (5 MiB) over bridge: %v", err)
	}
	if !putSnapResp.Accepted {
		t.Fatal("PutSnapshot (5 MiB) was not accepted")
	}

	var getSnapResp backendrpc.GetSnapshotResponse
	err = conn.Invoke(ctx, backendrpc.MethodBlobSyncGetSnapshot, backendrpc.GetSnapshotRequest{
		Workspace: "ws-1",
	}, &getSnapResp, grpccodec.CallOptions()...)
	if err != nil {
		t.Fatalf("GetSnapshot (5 MiB) over bridge: %v", err)
	}
	if !getSnapResp.Found || getSnapResp.CoversSeq != 1 || !bytes.Equal(getSnapResp.Blob, bigSnap) {
		t.Fatalf("GetSnapshot mismatch: found=%v seq=%d len=%d", getSnapResp.Found, getSnapResp.CoversSeq, len(getSnapResp.Blob))
	}
}

func TestBlobSyncEnforcesViewerAndDeleteAccountPurge(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "v2acl.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.UpsertUser(User{ID: "viewer-1", Provider: "test", Subject: "viewer-1", Email: "v@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserRole("viewer-1", RoleViewer); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertUser(User{ID: "member-1", Provider: "test", Subject: "member-1", Email: "m@example.com"}); err != nil {
		t.Fatal(err)
	}

	svc := NewBlobSyncService(st, Config{})
	viewerCtx := blobSyncContext("viewer-1")
	if _, err := svc.AppendChanges(viewerCtx, backendrpc.AppendChangesRequest{
		Workspace: "ws",
		Changes:   []backendrpc.BlobChange{{ChangeID: "c1", DeviceID: "d1", Blob: []byte("x")}},
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("viewer AppendChanges = %v, want PermissionDenied", err)
	}

	memberCtx := blobSyncContext("member-1")
	if _, err := svc.AppendChanges(memberCtx, backendrpc.AppendChangesRequest{
		Workspace: "ws",
		Changes:   []backendrpc.BlobChange{{ChangeID: "c1", DeviceID: "d1", Blob: []byte("x")}},
	}); err != nil {
		t.Fatalf("member AppendChanges: %v", err)
	}
	if _, err := svc.PutSnapshot(memberCtx, backendrpc.PutSnapshotRequest{
		Workspace: "ws", CoversSeq: 1, Blob: []byte("snap"),
	}); err != nil {
		t.Fatalf("member PutSnapshot: %v", err)
	}

	deleted, err := st.DeleteAccount("member-1")
	if err != nil || !deleted {
		t.Fatalf("DeleteAccount = (%v, %v)", deleted, err)
	}
	for _, table := range []string{"change_log", "ws_snapshot", "ws_head", "keyring", "pairing_relay"} {
		var count int
		if err := st.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, "member-1").Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s still has %d rows after DeleteAccount", table, count)
		}
	}
}
