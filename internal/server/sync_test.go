package server

import (
	"context"
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
