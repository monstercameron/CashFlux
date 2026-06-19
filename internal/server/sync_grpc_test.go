package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc"
)

func TestSyncServiceGRPCBridgeWorkspaceRoundTrip(t *testing.T) {
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
	if string(get.Dataset) != `{"schemaVersion":1,"accounts":[]}` {
		t.Fatalf("GetWorkspace dataset = %q", get.Dataset)
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
