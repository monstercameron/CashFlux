// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc"
)

func TestServerLoadSmokeSyncBlobAndWatch(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{
		AuthMode:     "token",
		Token:        "dev-token",
		AppOrigin:    "*",
		DataDir:      t.TempDir(),
		BlobMaxBytes: 1 << 20,
		Metrics:      NewMetrics(),
	}
	bridge := httptest.NewServer(NewMux(cfg, store))
	defer bridge.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	watchConn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: bridge.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("watch Dial: %v", err)
	}
	defer watchConn.Close()
	writeConn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: bridge.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("write Dial: %v", err)
	}
	defer writeConn.Close()

	stream, err := watchConn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, backendrpc.MethodSyncWatchWorkspaces, backendrpc.JSONCallOptions()...)
	if err != nil {
		t.Fatalf("WatchWorkspaces stream: %v", err)
	}
	if err := stream.SendMsg(&backendrpc.WatchWorkspacesRequest{IncludeDeleted: true}); err != nil {
		t.Fatalf("WatchWorkspaces send request: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("WatchWorkspaces close send: %v", err)
	}

	const workspaceCount = 8
	events := make(chan backendrpc.WatchWorkspacesResponse, workspaceCount)
	watchErr := make(chan error, 1)
	go func() {
		for i := 0; i < workspaceCount; i++ {
			var event backendrpc.WatchWorkspacesResponse
			if err := stream.RecvMsg(&event); err != nil {
				watchErr <- err
				return
			}
			events <- event
		}
		watchErr <- nil
	}()

	errs := make(chan error, workspaceCount)
	var wg sync.WaitGroup
	for i := 0; i < workspaceCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out backendrpc.PutWorkspaceResponse
			err := writeConn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
				Workspace: backendrpc.Workspace{
					ID:       fmt.Sprintf("w-load-%02d", i),
					Name:     fmt.Sprintf("Load %02d", i),
					DeviceID: "load-smoke",
				},
				Dataset:         []byte(fmt.Sprintf(`{"schemaVersion":1,"n":%d}`, i)),
				ClientUpdatedAt: time.Now().UTC().Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano),
			}, &out, backendrpc.JSONCallOptions()...)
			if err != nil {
				errs <- err
				return
			}
			if !out.Accepted {
				errs <- fmt.Errorf("workspace %d not accepted: %+v", i, out)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("sync load write: %v", err)
		}
	}
	if err := <-watchErr; err != nil {
		t.Fatalf("watch load receive: %v", err)
	}
	close(events)
	seen := map[string]bool{}
	for event := range events {
		seen[event.Workspace.ID] = true
	}
	if len(seen) != workspaceCount {
		t.Fatalf("watch events = %v, want %d distinct workspaces", seen, workspaceCount)
	}

	var list backendrpc.ListWorkspacesResponse
	if err := writeConn.Invoke(ctx, backendrpc.MethodSyncListWorkspaces, backendrpc.ListWorkspacesRequest{}, &list, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(list.Workspaces) != workspaceCount {
		t.Fatalf("workspace list has %d entries, want %d", len(list.Workspaces), workspaceCount)
	}

	for i := 0; i < 4; i++ {
		data := []byte(fmt.Sprintf("receipt-%02d", i))
		hash := sha256.Sum256(data)
		hashHex := hex.EncodeToString(hash[:])
		putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, bridge.URL+"/v1/blobs/"+hashHex+"?workspaceId=w-load-00", bytes.NewReader(data))
		if err != nil {
			t.Fatalf("build blob PUT: %v", err)
		}
		putReq.Header.Set("Authorization", "Bearer dev-token")
		putReq.Header.Set("Content-Type", "text/plain")
		putResp, err := http.DefaultClient.Do(putReq)
		if err != nil {
			t.Fatalf("blob PUT: %v", err)
		}
		_ = putResp.Body.Close()
		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("blob PUT status = %d", putResp.StatusCode)
		}

		getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, bridge.URL+"/v1/blobs/"+hashHex+"?workspaceId=w-load-00", nil)
		if err != nil {
			t.Fatalf("build blob GET: %v", err)
		}
		getReq.Header.Set("Authorization", "Bearer dev-token")
		getResp, err := http.DefaultClient.Do(getReq)
		if err != nil {
			t.Fatalf("blob GET: %v", err)
		}
		got, err := io.ReadAll(getResp.Body)
		_ = getResp.Body.Close()
		if err != nil {
			t.Fatalf("blob GET read: %v", err)
		}
		if getResp.StatusCode != http.StatusOK || !bytes.Equal(got, data) {
			t.Fatalf("blob GET status/body = %d/%q, want 200/%q", getResp.StatusCode, got, data)
		}
	}
}
