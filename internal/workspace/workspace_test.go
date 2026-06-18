package workspace

import "testing"

func TestAddAndActive(t *testing.T) {
	var r Registry
	if _, ok := r.Active(); ok {
		t.Error("empty registry should have no active workspace")
	}

	r = r.Add("w1", "Real money")
	if r.ActiveID != "w1" {
		t.Errorf("first add should become active, got %q", r.ActiveID)
	}
	r = r.Add("w2", "Sandbox")
	if r.ActiveID != "w1" {
		t.Errorf("second add must not steal active, got %q", r.ActiveID)
	}
	if len(r.Workspaces) != 2 {
		t.Fatalf("want 2 workspaces, got %d", len(r.Workspaces))
	}

	// Duplicate id and empty id are ignored.
	if got := r.Add("w1", "dup"); len(got.Workspaces) != 2 {
		t.Error("duplicate id should be ignored")
	}
	if got := r.Add("", "nameless"); len(got.Workspaces) != 2 {
		t.Error("empty id should be ignored")
	}
}

func TestActiveFallback(t *testing.T) {
	r := Registry{Workspaces: []Workspace{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}, ActiveID: "gone"}
	w, ok := r.Active()
	if !ok || w.ID != "a" {
		t.Errorf("dangling ActiveID should fall back to the first, got %q ok=%v", w.ID, ok)
	}
}

func TestSetActiveAndRename(t *testing.T) {
	r := (Registry{}).Add("w1", "One").Add("w2", "Two")
	r = r.SetActive("w2")
	if r.ActiveID != "w2" {
		t.Errorf("SetActive failed, got %q", r.ActiveID)
	}
	if r.SetActive("nope").ActiveID != "w2" {
		t.Error("SetActive to an unknown id should be a no-op")
	}
	r = r.Rename("w1", "Renamed")
	if w, _ := r.Get("w1"); w.Name != "Renamed" {
		t.Errorf("rename failed, got %q", w.Name)
	}
	if got := r.Rename("nope", "x"); !equalNames(got, r) {
		t.Error("renaming an unknown id should be a no-op")
	}
}

func TestRemove(t *testing.T) {
	r := (Registry{}).Add("w1", "One").Add("w2", "Two").Add("w3", "Three").SetActive("w2")

	// Removing the active workspace promotes the first survivor.
	r2 := r.Remove("w2")
	if r2.Has("w2") {
		t.Error("w2 should be gone")
	}
	if r2.ActiveID != "w1" {
		t.Errorf("removing the active should fall back to the first, got %q", r2.ActiveID)
	}
	if len(r2.Workspaces) != 2 {
		t.Fatalf("want 2 left, got %d", len(r2.Workspaces))
	}

	// Removing a non-active keeps the active.
	r3 := r.Remove("w3")
	if r3.ActiveID != "w2" {
		t.Errorf("removing a non-active changed active to %q", r3.ActiveID)
	}

	// The last workspace can't be removed.
	one := (Registry{}).Add("only", "Only")
	if got := one.Remove("only"); len(got.Workspaces) != 1 {
		t.Error("the last workspace must not be removable")
	}

	// Unknown id is a no-op.
	if got := r.Remove("nope"); len(got.Workspaces) != 3 {
		t.Error("removing an unknown id should be a no-op")
	}
}

func equalNames(a, b Registry) bool {
	if len(a.Workspaces) != len(b.Workspaces) {
		return false
	}
	for i := range a.Workspaces {
		if a.Workspaces[i] != b.Workspaces[i] {
			return false
		}
	}
	return true
}
