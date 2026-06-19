package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSyncServiceGRPCBridgeWorkspaceRoundTrip(t *testing.T) {
	store := openTestStore(t)
	metrics := NewMetrics()
	cfg := Config{AuthMode: "token", Token: "dev-token", AppOrigin: "*", Metrics: metrics}
	bridge := httptest.NewServer(NewMux(cfg, store))
	defer bridge.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: bridge.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	clientUpdatedAt := time.Date(2026, time.June, 18, 20, 10, 0, 0, time.UTC)
	var put backendrpc.PutWorkspaceResponse
	err = conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
		Workspace: backendrpc.Workspace{
			ID:       "w-grpc",
			Name:     "Home",
			Color:    "blue",
			Sort:     2,
			DeviceID: "browser-a",
		},
		Dataset:         []byte(`{"schemaVersion":1,"accounts":[]}`),
		ClientUpdatedAt: clientUpdatedAt.Format(time.RFC3339Nano),
	}, &put, backendrpc.JSONCallOptions()...)
	if err != nil {
		t.Fatalf("PutWorkspace invoke: %v", err)
	}
	if !put.Accepted || put.Version != 1 || put.Workspace.ID != "w-grpc" || put.Workspace.Name != "Home" {
		t.Fatalf("PutWorkspace response = %+v", put)
	}
	if string(put.Dataset) != `{"schemaVersion":1,"accounts":[]}` {
		t.Fatalf("PutWorkspace dataset = %q", put.Dataset)
	}

	var list backendrpc.ListWorkspacesResponse
	if err := conn.Invoke(ctx, backendrpc.MethodSyncListWorkspaces, backendrpc.ListWorkspacesRequest{}, &list, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("ListWorkspaces invoke: %v", err)
	}
	if len(list.Workspaces) != 1 || list.Workspaces[0].ID != "w-grpc" {
		t.Fatalf("ListWorkspaces response = %+v", list)
	}

	var stale backendrpc.PutWorkspaceResponse
	err = conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
		Workspace:       backendrpc.Workspace{ID: "w-grpc", Name: "Stale"},
		ClientUpdatedAt: clientUpdatedAt.Add(-time.Hour).Format(time.RFC3339Nano),
	}, &stale, backendrpc.JSONCallOptions()...)
	if err != nil {
		t.Fatalf("stale PutWorkspace invoke: %v", err)
	}
	if stale.Accepted || stale.Workspace.Name != "Home" || stale.Version != 1 {
		t.Fatalf("stale PutWorkspace response = %+v", stale)
	}
	if string(stale.Dataset) != `{"schemaVersion":1,"accounts":[]}` {
		t.Fatalf("stale PutWorkspace dataset = %q", stale.Dataset)
	}

	var get backendrpc.GetWorkspaceResponse
	if err := conn.Invoke(ctx, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: "w-grpc"}, &get, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("GetWorkspace invoke: %v", err)
	}
	if !get.Found || get.Workspace.ID != "w-grpc" {
		t.Fatalf("GetWorkspace response = %+v", get)
	}
	if get.ETag == "" {
		t.Fatalf("GetWorkspace ETag is empty: %+v", get)
	}
	if string(get.Dataset) != `{"schemaVersion":1,"accounts":[]}` {
		t.Fatalf("GetWorkspace dataset = %q", get.Dataset)
	}
	var cached backendrpc.GetWorkspaceResponse
	if err := conn.Invoke(ctx, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: "w-grpc", IfNoneMatch: get.ETag}, &cached, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("cached GetWorkspace invoke: %v", err)
	}
	if !cached.Found || !cached.NotModified || cached.ETag != get.ETag || len(cached.Dataset) != 0 {
		t.Fatalf("cached GetWorkspace response = %+v, want not-modified without dataset", cached)
	}

	var del backendrpc.DeleteWorkspaceResponse
	if err := conn.Invoke(ctx, backendrpc.MethodSyncDeleteWorkspace, backendrpc.DeleteWorkspaceRequest{ID: "w-grpc", DeviceID: "browser-a"}, &del, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("DeleteWorkspace invoke: %v", err)
	}
	if !del.Deleted {
		t.Fatalf("DeleteWorkspace response = %+v", del)
	}

	list = backendrpc.ListWorkspacesResponse{}
	if err := conn.Invoke(ctx, backendrpc.MethodSyncListWorkspaces, backendrpc.ListWorkspacesRequest{}, &list, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("ListWorkspaces after delete invoke: %v", err)
	}
	if len(list.Workspaces) != 0 {
		t.Fatalf("active workspaces after delete = %+v", list)
	}
	events, err := store.ListAuditEvents(0, 10)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	var sawPut, sawDelete bool
	for _, event := range events {
		if event.Action == "workspace.put" && event.TargetID == "w-grpc" && event.ActorID != "" && event.Hash != "" {
			sawPut = true
		}
		if event.Action == "workspace.delete" && event.TargetID == "w-grpc" && event.PreviousHash != "" {
			sawDelete = true
		}
	}
	if !sawPut || !sawDelete {
		t.Fatalf("sync audit events = %+v", events)
	}

	var metricsOut strings.Builder
	metrics.WritePrometheus(&metricsOut)
	for _, want := range []string{
		`cashflux_sync_pulls_total{result="found"} 1`,
		`cashflux_sync_pulls_total{result="list"} 2`,
		`cashflux_sync_pulls_total{result="not_modified"} 1`,
		`cashflux_sync_pushes_total{result="accepted"} 1`,
		`cashflux_sync_pushes_total{result="deleted"} 1`,
		`cashflux_sync_pushes_total{result="lww_rejected"} 1`,
		`cashflux_sync_lww_rejects_total 1`,
	} {
		if !strings.Contains(metricsOut.String(), want) {
			t.Fatalf("missing metric %q in:\n%s", want, metricsOut.String())
		}
	}
}

func TestSyncServiceGRPCBridgeWatchWorkspaces(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{AuthMode: "token", Token: "dev-token", AppOrigin: "*"}
	bridge := httptest.NewServer(NewMux(cfg, store))
	defer bridge.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	var put backendrpc.PutWorkspaceResponse
	if err := writeConn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
		Workspace:       backendrpc.Workspace{ID: "w-watch", Name: "Watched", DeviceID: "browser-b"},
		ClientUpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}, &put, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("PutWorkspace invoke: %v", err)
	}
	if !put.Accepted {
		t.Fatalf("PutWorkspace response = %+v", put)
	}

	var event backendrpc.WatchWorkspacesResponse
	if err := stream.RecvMsg(&event); err != nil {
		t.Fatalf("WatchWorkspaces recv: %v", err)
	}
	if event.Workspace.ID != "w-watch" || event.Workspace.Name != "Watched" || event.Workspace.Version != 1 {
		t.Fatalf("WatchWorkspaces event = %+v", event)
	}
}

func TestSyncServiceGRPCBridgeTwoDeviceLWWAndTombstone(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{AuthMode: "token", Token: "dev-token", AppOrigin: "*"}
	bridge := httptest.NewServer(NewMux(cfg, store))
	defer bridge.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deviceA, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: bridge.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("device A dial: %v", err)
	}
	defer deviceA.Close()
	deviceB, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: bridge.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("device B dial: %v", err)
	}
	defer deviceB.Close()

	watch, err := deviceB.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, backendrpc.MethodSyncWatchWorkspaces, backendrpc.JSONCallOptions()...)
	if err != nil {
		t.Fatalf("watch stream: %v", err)
	}
	if err := watch.SendMsg(&backendrpc.WatchWorkspacesRequest{IncludeDeleted: true}); err != nil {
		t.Fatalf("watch send: %v", err)
	}
	if err := watch.CloseSend(); err != nil {
		t.Fatalf("watch close send: %v", err)
	}

	base := time.Date(2026, time.June, 19, 17, 0, 0, 0, time.UTC)
	var put backendrpc.PutWorkspaceResponse
	if err := deviceA.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
		Workspace:       backendrpc.Workspace{ID: "w-two-device", Name: "Device A", DeviceID: "device-a"},
		Dataset:         []byte(`{"schemaVersion":1,"from":"device-a"}`),
		ClientUpdatedAt: base.Format(time.RFC3339Nano),
	}, &put, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("device A put: %v", err)
	}
	if !put.Accepted {
		t.Fatalf("device A put response = %+v", put)
	}
	var created backendrpc.WatchWorkspacesResponse
	if err := watch.RecvMsg(&created); err != nil {
		t.Fatalf("watch create recv: %v", err)
	}
	if created.Workspace.ID != "w-two-device" || created.Workspace.DeviceID != "device-a" || created.Workspace.Deleted {
		t.Fatalf("watch create = %+v", created)
	}

	var stale backendrpc.PutWorkspaceResponse
	if err := deviceB.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
		Workspace:       backendrpc.Workspace{ID: "w-two-device", Name: "Stale Device B", DeviceID: "device-b"},
		Dataset:         []byte(`{"schemaVersion":1,"from":"device-b"}`),
		ClientUpdatedAt: base.Add(-time.Minute).Format(time.RFC3339Nano),
	}, &stale, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("device B stale put: %v", err)
	}
	if stale.Accepted || stale.Workspace.Name != "Device A" || string(stale.Dataset) != `{"schemaVersion":1,"from":"device-a"}` {
		t.Fatalf("stale response = %+v dataset %q", stale, stale.Dataset)
	}

	var del backendrpc.DeleteWorkspaceResponse
	if err := deviceA.Invoke(ctx, backendrpc.MethodSyncDeleteWorkspace, backendrpc.DeleteWorkspaceRequest{
		ID:        "w-two-device",
		UpdatedAt: base.Add(time.Minute).Format(time.RFC3339Nano),
		DeviceID:  "device-a",
	}, &del, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("device A delete: %v", err)
	}
	if !del.Deleted {
		t.Fatalf("delete response = %+v", del)
	}
	var tombstone backendrpc.WatchWorkspacesResponse
	if err := watch.RecvMsg(&tombstone); err != nil {
		t.Fatalf("watch tombstone recv: %v", err)
	}
	if tombstone.Workspace.ID != "w-two-device" || !tombstone.Workspace.Deleted || tombstone.Workspace.DeviceID != "device-a" {
		t.Fatalf("watch tombstone = %+v", tombstone)
	}

	var list backendrpc.ListWorkspacesResponse
	if err := deviceB.Invoke(ctx, backendrpc.MethodSyncListWorkspaces, backendrpc.ListWorkspacesRequest{IncludeDeleted: true}, &list, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("device B list deleted: %v", err)
	}
	if len(list.Workspaces) != 1 || list.Workspaces[0].ID != "w-two-device" || !list.Workspaces[0].Deleted {
		t.Fatalf("deleted list = %+v", list)
	}
}

func TestSyncServiceGRPCBridgeRejectsOversizedSnapshot(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{AuthMode: "token", Token: "dev-token", AppOrigin: "*"}
	bridge := httptest.NewServer(NewMux(cfg, store))
	defer bridge.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: bridge.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	var put backendrpc.PutWorkspaceResponse
	err = conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
		Workspace:       backendrpc.Workspace{ID: "w-too-large", Name: "Too Large"},
		Dataset:         make([]byte, defaultSnapshotMaxBytes+1),
		ClientUpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}, &put, backendrpc.JSONCallOptions()...)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized PutWorkspace err = %v, want resource exhausted", err)
	}
}
