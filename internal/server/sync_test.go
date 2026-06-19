package server

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSyncServiceListGetDeleteAreUserScoped(t *testing.T) {
	store := openTestStore(t)
	service := NewSyncService(store)
	now := time.Date(2026, time.June, 18, 18, 0, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", now)
	seedSyncUser(t, store, "u2", now)
	for _, workspace := range []Workspace{
		{ID: "w1", UserID: "u1", Name: "Home", Sort: 1, UpdatedAt: now},
		{ID: "w2", UserID: "u1", Name: "Budget", Sort: 2, UpdatedAt: now},
		{ID: "w3", UserID: "u2", Name: "Other", Sort: 1, UpdatedAt: now},
	} {
		if err := store.PutWorkspace(workspace); err != nil {
			t.Fatalf("PutWorkspace %s: %v", workspace.ID, err)
		}
	}

	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	list, err := service.List(ctx, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].ID != "w1" || list[1].ID != "w2" {
		t.Fatalf("scoped list = %+v", list)
	}
	workspace, ok, err := service.Get(ctx, "w1")
	if err != nil || !ok || workspace.UserID != "u1" {
		t.Fatalf("Get own = %+v/%v/%v", workspace, ok, err)
	}
	if _, ok, err := service.Get(ctx, "w3"); err != nil || ok {
		t.Fatalf("Get cross-user = ok %v err %v, want missing", ok, err)
	}
	deleted, err := service.Delete(ctx, "w3", now.Add(time.Minute), "laptop")
	if err != nil || deleted {
		t.Fatalf("Delete cross-user = %v/%v, want false nil", deleted, err)
	}
	deleted, err = service.Delete(ctx, "w1", now.Add(time.Minute), "laptop")
	if err != nil || !deleted {
		t.Fatalf("Delete own = %v/%v, want true nil", deleted, err)
	}
	after, err := service.List(ctx, false)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(after) != 1 || after[0].ID != "w2" {
		t.Fatalf("active list after delete = %+v", after)
	}
}

func TestSyncServicePutWorkspaceLWW(t *testing.T) {
	store := openTestStore(t)
	service := NewSyncService(store)
	now := time.Date(2026, time.June, 18, 18, 30, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", now)
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})

	result, err := service.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Home", Color: "blue"}, now, false, now.Add(time.Minute))
	if err != nil || !result.Accepted || result.Version != 1 || !result.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("initial PutWorkspace = %+v/%v", result, err)
	}
	stale, err := service.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Stale"}, now.Add(-time.Hour), false, now.Add(2*time.Minute))
	if err != nil || stale.Accepted {
		t.Fatalf("stale PutWorkspace = %+v/%v, want reject", stale, err)
	}
	if stale.Workspace.Name != "Home" || stale.Version != 1 || !stale.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("stale result current state = %+v", stale)
	}
	fresh, err := service.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Fresh", Color: "green"}, now.Add(time.Minute), false, now.Add(3*time.Minute))
	if err != nil || !fresh.Accepted || fresh.Version != 2 {
		t.Fatalf("fresh PutWorkspace = %+v/%v", fresh, err)
	}
	forced, err := service.PutWorkspace(ctx, Workspace{ID: "w1", Name: "Forced"}, now.Add(-time.Hour), true, now.Add(4*time.Minute))
	if err != nil || !forced.Accepted || forced.Version != 3 {
		t.Fatalf("forced PutWorkspace = %+v/%v", forced, err)
	}
}

func TestSyncServicePutWorkspaceRejectsCrossUserIDTakeover(t *testing.T) {
	store := openTestStore(t)
	service := NewSyncService(store)
	now := time.Date(2026, time.June, 18, 18, 45, 0, 0, time.UTC)
	seedSyncUser(t, store, "u1", now)
	seedSyncUser(t, store, "u2", now)
	if err := store.PutWorkspace(Workspace{ID: "shared", UserID: "u2", Name: "Other", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace seed: %v", err)
	}
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	if _, err := service.PutWorkspace(ctx, Workspace{ID: "shared", Name: "Takeover"}, now, true, now.Add(time.Minute)); status.Code(err) != codes.NotFound {
		t.Fatalf("cross-user PutWorkspace = %v, want not found", err)
	}
	workspace, ok, err := store.GetWorkspace("u2", "shared")
	if err != nil || !ok || workspace.Name != "Other" {
		t.Fatalf("cross-user workspace after rejected put = %+v/%v/%v", workspace, ok, err)
	}
}

func TestSyncServiceWatchFanoutIsUserScoped(t *testing.T) {
	metrics := NewMetrics()
	service := NewSyncServiceWithLimits(openTestStore(t), 0, metrics)
	u1, unsubscribe1, err := service.subscribeWorkspaces("u1")
	if err != nil {
		t.Fatalf("subscribe u1: %v", err)
	}
	u2, unsubscribe2, err := service.subscribeWorkspaces("u2")
	if err != nil {
		t.Fatalf("subscribe u2: %v", err)
	}

	service.publishWorkspace("u1", Workspace{ID: "w1", Name: "Home", Version: 2})
	var out strings.Builder
	metrics.WritePrometheus(&out)
	if !strings.Contains(out.String(), `cashflux_queue_depth{queue="workspace_watch"} 1`) {
		t.Fatalf("queue depth after publish = %q", out.String())
	}

	select {
	case event := <-u1:
		if event.Workspace.ID != "w1" || event.Workspace.Name != "Home" || event.Workspace.Version != 2 {
			t.Fatalf("u1 event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("u1 watcher did not receive event")
	}
	select {
	case event := <-u2:
		t.Fatalf("u2 watcher received cross-user event %+v", event)
	default:
	}
	unsubscribe1()
	unsubscribe2()
	out.Reset()
	metrics.WritePrometheus(&out)
	if !strings.Contains(out.String(), `cashflux_queue_depth{queue="workspace_watch"} 0`) {
		t.Fatalf("queue depth after unsubscribe = %q", out.String())
	}
}

func TestSyncServiceWatchStreamLimit(t *testing.T) {
	metrics := NewMetrics()
	service := NewSyncServiceWithLimits(openTestStore(t), 1, metrics)
	_, unsubscribe, err := service.subscribeWorkspaces("u1")
	if err != nil {
		t.Fatalf("first subscribe: %v", err)
	}
	var out bytes.Buffer
	metrics.WritePrometheus(&out)
	if !strings.Contains(out.String(), "cashflux_grpc_streams_active 1") {
		t.Fatalf("active stream metric after subscribe = %q", out.String())
	}
	if _, _, err := service.subscribeWorkspaces("u1"); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("second subscribe = %v, want resource exhausted", err)
	}
	if _, unsubscribeOther, err := service.subscribeWorkspaces("u2"); err != nil {
		t.Fatalf("other user subscribe: %v", err)
	} else {
		unsubscribeOther()
	}
	unsubscribe()
	out.Reset()
	metrics.WritePrometheus(&out)
	if !strings.Contains(out.String(), "cashflux_grpc_streams_active 0") {
		t.Fatalf("active stream metric after unsubscribe = %q", out.String())
	}
}

func TestSyncServiceRequiresAuthenticatedUser(t *testing.T) {
	service := NewSyncService(openTestStore(t))
	if _, err := service.List(context.Background(), false); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("List unauthenticated = %v, want unauthenticated", err)
	}
	ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: "u1"})
	if _, _, err := service.Get(ctx, ""); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Get blank id = %v, want invalid argument", err)
	}
	if _, err := service.Delete(ctx, "", time.Now().UTC(), ""); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Delete blank id = %v, want invalid argument", err)
	}
}

func seedSyncUser(t *testing.T, store *Store, id string, now time.Time) {
	t.Helper()
	if err := store.UpsertUser(User{ID: id, Provider: "github", Subject: id, CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser %s: %v", id, err)
	}
}
