// SPDX-License-Identifier: MIT

package scope_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/scope"
)

// ---- helpers ----

func makeView(id, name string, s scope.ReportScope) scope.SavedView {
	return scope.SavedView{ID: id, Name: name, Scope: s}
}

// ---- ListSavedViews ----

func TestListSavedViews_Empty(t *testing.T) {
	got := scope.ListSavedViews(nil)
	if len(got) != 0 {
		t.Errorf("expected empty list for nil map, got %v", got)
	}
	got = scope.ListSavedViews(map[string]string{})
	if len(got) != 0 {
		t.Errorf("expected empty list for empty map, got %v", got)
	}
}

func TestListSavedViews_BadJSONSkipped(t *testing.T) {
	kv := map[string]string{
		"good": `{"id":"v1","name":"My view","scope":{}}`,
		"bad":  `not valid json`,
	}
	got := scope.ListSavedViews(kv)
	if len(got) != 1 {
		t.Fatalf("expected 1 view (bad JSON skipped), got %d: %v", len(got), got)
	}
	if got[0].ID != "v1" {
		t.Errorf("unexpected view ID: %q", got[0].ID)
	}
}

func TestListSavedViews_SortedByNameCaseInsensitive(t *testing.T) {
	kv := make(map[string]string)
	kv = scope.PutSavedView(kv, makeView("c", "Zebra view", scope.ReportScope{}))
	kv = scope.PutSavedView(kv, makeView("a", "apple view", scope.ReportScope{}))
	kv = scope.PutSavedView(kv, makeView("b", "Mango view", scope.ReportScope{}))

	got := scope.ListSavedViews(kv)
	if len(got) != 3 {
		t.Fatalf("expected 3 views, got %d", len(got))
	}
	// Expected order: apple < Mango < Zebra (case-insensitive)
	wantOrder := []string{"a", "b", "c"}
	for i, want := range wantOrder {
		if got[i].ID != want {
			t.Errorf("position %d: got ID %q, want %q", i, got[i].ID, want)
		}
	}
}

// ---- PutSavedView ----

func TestPutSavedView_NilMap(t *testing.T) {
	v := makeView("v1", "My view", scope.ReportScope{Owners: []string{"alice"}})
	kv := scope.PutSavedView(nil, v)
	if len(kv) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(kv))
	}
	if _, ok := kv["v1"]; !ok {
		t.Error("expected key v1 to exist")
	}
}

func TestPutSavedView_Overwrite(t *testing.T) {
	v1 := makeView("v1", "Old name", scope.ReportScope{})
	v2 := makeView("v1", "New name", scope.ReportScope{Institutions: []string{"Chase"}})

	kv := scope.PutSavedView(nil, v1)
	kv = scope.PutSavedView(kv, v2)
	if len(kv) != 1 {
		t.Fatalf("expected 1 entry after overwrite, got %d", len(kv))
	}

	views := scope.ListSavedViews(kv)
	if views[0].Name != "New name" {
		t.Errorf("expected overwritten name 'New name', got %q", views[0].Name)
	}
}

// ---- DeleteSavedView ----

func TestDeleteSavedView_Existing(t *testing.T) {
	kv := scope.PutSavedView(nil, makeView("v1", "Keep", scope.ReportScope{}))
	kv = scope.PutSavedView(kv, makeView("v2", "Delete me", scope.ReportScope{}))
	kv = scope.DeleteSavedView(kv, "v2")

	if len(kv) != 1 {
		t.Fatalf("expected 1 entry after delete, got %d", len(kv))
	}
	if _, ok := kv["v1"]; !ok {
		t.Error("expected v1 to survive")
	}
}

func TestDeleteSavedView_NonExistent_NoOp(t *testing.T) {
	kv := scope.PutSavedView(nil, makeView("v1", "Keep", scope.ReportScope{}))
	kv = scope.DeleteSavedView(kv, "does-not-exist")
	if len(kv) != 1 {
		t.Errorf("expected 1 entry unchanged, got %d", len(kv))
	}
}

func TestDeleteSavedView_NilMap(t *testing.T) {
	// Must not panic.
	kv := scope.DeleteSavedView(nil, "any-id")
	if kv != nil {
		t.Errorf("expected nil return for nil input, got %v", kv)
	}
}

// ---- Round-trip: Put → List → Delete ----

func TestSavedView_RoundTrip(t *testing.T) {
	views := []scope.SavedView{
		makeView("v1", "Alice only", scope.ReportScope{Owners: []string{"alice"}}),
		makeView("v2", "Chase accounts", scope.ReportScope{Institutions: []string{"Chase"}}),
		makeView("v3", "All", scope.ReportScope{}),
	}

	kv := make(map[string]string)
	for _, v := range views {
		kv = scope.PutSavedView(kv, v)
	}

	list := scope.ListSavedViews(kv)
	if len(list) != 3 {
		t.Fatalf("expected 3 views, got %d", len(list))
	}

	// Delete one and verify.
	kv = scope.DeleteSavedView(kv, "v2")
	list = scope.ListSavedViews(kv)
	if len(list) != 2 {
		t.Fatalf("expected 2 views after delete, got %d", len(list))
	}
	for _, v := range list {
		if v.ID == "v2" {
			t.Error("deleted view v2 still present")
		}
	}

	// Verify scope round-trips.
	kv2 := scope.PutSavedView(nil, makeView("s1", "Scoped", scope.ReportScope{
		Institutions: []string{"Wells Fargo"},
		Owners:       []string{"bob"},
	}))
	out := scope.ListSavedViews(kv2)
	if len(out) != 1 {
		t.Fatalf("expected 1, got %d", len(out))
	}
	if len(out[0].Scope.Institutions) != 1 || out[0].Scope.Institutions[0] != "Wells Fargo" {
		t.Errorf("institution not round-tripped: %v", out[0].Scope.Institutions)
	}
	if len(out[0].Scope.Owners) != 1 || out[0].Scope.Owners[0] != "bob" {
		t.Errorf("owner not round-tripped: %v", out[0].Scope.Owners)
	}
}
