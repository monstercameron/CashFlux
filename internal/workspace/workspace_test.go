// SPDX-License-Identifier: MIT

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

func TestStartup(t *testing.T) {
	r := (Registry{}).Add("w1", "One").Add("w2", "Two").SetActive("w2")

	// Default: no pin → launch resumes the active workspace.
	if r.StartupID != "" {
		t.Errorf("StartupID should default empty, got %q", r.StartupID)
	}
	if got := r.StartupTarget(); got != "w2" {
		t.Errorf("unpinned StartupTarget should be the active workspace, got %q", got)
	}

	// Pin to a specific workspace → launch opens it regardless of active.
	r = r.SetStartup("w1")
	if r.StartupID != "w1" {
		t.Errorf("SetStartup failed, got %q", r.StartupID)
	}
	if got := r.StartupTarget(); got != "w1" {
		t.Errorf("pinned StartupTarget should be w1, got %q", got)
	}

	// Pinning to an unknown id is a no-op (stays on the previous pin).
	if got := r.SetStartup("nope"); got.StartupID != "w1" {
		t.Errorf("pinning an unknown id should be a no-op, got %q", got.StartupID)
	}

	// Clearing the pin (empty id) returns to last-active behaviour.
	if got := r.SetStartup(""); got.StartupID != "" || got.StartupTarget() != "w2" {
		t.Errorf("clearing the pin failed: id=%q target=%q", got.StartupID, got.StartupTarget())
	}

	// A dangling pin (workspace removed out from under it) resolves to active.
	dangling := Registry{Workspaces: r.Workspaces, ActiveID: "w2", StartupID: "ghost"}
	if got := dangling.StartupTarget(); got != "w2" {
		t.Errorf("dangling pin should fall back to active, got %q", got)
	}

	// Rename/SetActive must preserve the pin (clone() carries StartupID).
	if got := r.Rename("w1", "Renamed").StartupID; got != "w1" {
		t.Errorf("Rename dropped the startup pin, got %q", got)
	}
	if got := r.SetActive("w1").StartupID; got != "w1" {
		t.Errorf("SetActive dropped the startup pin, got %q", got)
	}

	// Removing the pinned workspace clears the pin (never launch into a ghost).
	if got := r.Remove("w1"); got.StartupID != "" {
		t.Errorf("removing the pinned workspace should clear the pin, got %q", got.StartupID)
	}
	// Removing a different workspace keeps the pin.
	if got := r.Remove("w2"); got.StartupID != "w1" {
		t.Errorf("removing a non-pinned workspace should keep the pin, got %q", got.StartupID)
	}
}

func TestSetColor(t *testing.T) {
	r := (Registry{}).Add("w1", "One").Add("w2", "Two")

	r = r.SetColor("w1", "#2e8b57")
	if w, _ := r.Get("w1"); w.Color != "#2e8b57" {
		t.Errorf("SetColor failed, got %q", w.Color)
	}
	// Other workspaces are untouched.
	if w, _ := r.Get("w2"); w.Color != "" {
		t.Errorf("SetColor leaked to w2, got %q", w.Color)
	}
	// Unknown id is a no-op.
	if got := r.SetColor("nope", "#fff"); !equalColors(got, r) {
		t.Error("SetColor on an unknown id should be a no-op")
	}
	// Clearing works, and the color survives a rename (clone copies it).
	if w, _ := r.SetColor("w1", "").Get("w1"); w.Color != "" {
		t.Errorf("clearing the color failed, got %q", w.Color)
	}
	if w, _ := r.Rename("w1", "Renamed").Get("w1"); w.Color != "#2e8b57" {
		t.Errorf("rename dropped the color, got %q", w.Color)
	}
}

func TestMove(t *testing.T) {
	base := (Registry{}).Add("a", "A").Add("b", "B").Add("c", "C").SetActive("b").SetStartup("c")
	order := func(r Registry) string {
		s := ""
		for _, w := range r.Workspaces {
			s += w.ID
		}
		return s
	}

	// Move first to last.
	if got := order(base.Move("a", 2)); got != "bca" {
		t.Errorf("move a→2: want bca, got %s", got)
	}
	// Move last to first.
	if got := order(base.Move("c", 0)); got != "cab" {
		t.Errorf("move c→0: want cab, got %s", got)
	}
	// Move middle up one.
	if got := order(base.Move("b", 0)); got != "bac" {
		t.Errorf("move b→0: want bac, got %s", got)
	}
	// Out-of-range index clamps.
	if got := order(base.Move("a", 99)); got != "bca" {
		t.Errorf("move a→99 should clamp to last: want bca, got %s", got)
	}
	// No-op cases keep order and don't disturb active/startup.
	moved := base.Move("a", 0) // already first
	if order(moved) != "abc" || moved.ActiveID != "b" || moved.StartupID != "c" {
		t.Errorf("no-op move changed state: order=%s active=%s startup=%s", order(moved), moved.ActiveID, moved.StartupID)
	}
	if order(base.Move("nope", 0)) != "abc" {
		t.Error("moving an unknown id should be a no-op")
	}
	one := (Registry{}).Add("only", "Only")
	if order(one.Move("only", 0)) != "only" {
		t.Error("moving in a single-element list should be a no-op")
	}
	// Active/startup survive a real move (tracked by id, not index).
	m := base.Move("c", 0)
	if m.ActiveID != "b" || m.StartupID != "c" {
		t.Errorf("move disturbed active/startup: active=%s startup=%s", m.ActiveID, m.StartupID)
	}
}

func equalColors(a, b Registry) bool {
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
