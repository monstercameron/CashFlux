package server

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRepositoryUserAndWorkspaceFlow(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.June, 18, 16, 12, 0, 0, time.UTC)
	for _, u := range []User{
		{ID: "u1", Provider: "github", Subject: "alice", Email: "alice@example.com", CreatedAt: now},
		{ID: "u2", Provider: "github", Subject: "bob", Email: "bob@example.com", CreatedAt: now},
	} {
		if err := s.UpsertUser(u); err != nil {
			t.Fatalf("UpsertUser %s: %v", u.ID, err)
		}
	}
	for _, w := range []Workspace{
		{ID: "w2", UserID: "u1", Name: "Budget", Sort: 2, Version: 1, UpdatedAt: now, DeviceID: "laptop"},
		{ID: "w1", UserID: "u1", Name: "Home", Sort: 1, Version: 3, UpdatedAt: now.Add(time.Minute), DeviceID: "phone"},
		{ID: "w3", UserID: "u2", Name: "Other user", Sort: 1, Version: 1, UpdatedAt: now, DeviceID: "tablet"},
	} {
		if err := s.PutWorkspace(w); err != nil {
			t.Fatalf("PutWorkspace %s: %v", w.ID, err)
		}
	}

	got, err := s.ListWorkspaces("u1", false)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(got) != 2 || got[0].ID != "w1" || got[1].ID != "w2" {
		t.Fatalf("u1 workspaces ordered/scoped = %+v", got)
	}
	w, ok, err := s.GetWorkspace("u1", "w1")
	if err != nil || !ok {
		t.Fatalf("GetWorkspace = %+v/%v/%v", w, ok, err)
	}
	if w.Version != 3 || !w.UpdatedAt.Equal(now.Add(time.Minute)) || w.DeviceID != "phone" {
		t.Fatalf("workspace metadata = %+v", w)
	}
	if _, ok, err := s.GetWorkspace("u2", "w1"); err != nil || ok {
		t.Fatalf("cross-user get = ok %v err %v, want not found", ok, err)
	}
}

func TestRepositoryValidationAndSoftDelete(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.June, 18, 16, 20, 0, 0, time.UTC)
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", Email: "alice@example.com", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	cases := []struct {
		name string
		run  func() error
	}{
		{name: "missing user id", run: func() error { return s.UpsertUser(User{Provider: "github", Subject: "x"}) }},
		{name: "missing workspace name", run: func() error { return s.PutWorkspace(Workspace{ID: "w1", UserID: "u1"}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	if err := s.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	deleted, err := s.SoftDeleteWorkspace("u1", "w1", now.Add(time.Hour), "phone")
	if err != nil || !deleted {
		t.Fatalf("SoftDeleteWorkspace = %v/%v, want deleted", deleted, err)
	}
	active, err := s.ListWorkspaces("u1", false)
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active workspaces after delete = %+v", active)
	}
	all, err := s.ListWorkspaces("u1", true)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 1 || !all[0].Deleted || all[0].DeviceID != "phone" {
		t.Fatalf("deleted workspace = %+v", all)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
