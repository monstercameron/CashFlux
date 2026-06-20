//go:build js && wasm

package screens

import (
	"strings"
	"testing"
)

// The screen registry drives BOTH routing (app.Run registers one route per entry)
// and the left rail (derived from Group). If an entry is malformed — duplicate or
// non-rooted path, missing view, unknown group — a rail item either fails to
// register (falling through to the "*" catch-all, i.e. "not navigable") or never
// appears in the nav. These invariants guard against that regression.

func TestRailRoutesResolve(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("registry is empty")
	}

	seen := map[string]bool{}
	groups := map[string]bool{GroupPrimary: true, GroupTools: true, GroupSystem: true}
	for _, r := range all {
		if !strings.HasPrefix(r.Path, "/") {
			t.Errorf("route %q: path must start with /", r.Path)
		}
		if r.Path == "/p/" || strings.HasPrefix(r.Path, "/p/") {
			t.Errorf("route %q collides with the /p/:slug custom-page pattern", r.Path)
		}
		if seen[r.Path] {
			t.Errorf("duplicate route path %q (only the first would register)", r.Path)
		}
		seen[r.Path] = true
		if r.View == nil {
			t.Errorf("route %q has a nil View (would render blank)", r.Path)
		}
		if strings.TrimSpace(r.Label) == "" || strings.TrimSpace(r.Title) == "" {
			t.Errorf("route %q is missing its Label/Title i18n key", r.Path)
		}
		if !groups[r.Group] {
			t.Errorf("route %q has unknown rail group %q — it would not appear in the nav", r.Path, r.Group)
		}
	}

	// The dashboard must exist and be the registry head: app.Run uses All()[0] as
	// the home/fallback, and "/" is the default route.
	if all[0].Path != "/" {
		t.Errorf("registry head = %q, want \"/\" (home/fallback depends on it)", all[0].Path)
	}
}

// TestEveryRailGroupHasScreens makes sure each rail section actually has entries,
// so a section header never renders above an empty list.
func TestEveryRailGroupHasScreens(t *testing.T) {
	counts := map[string]int{}
	for _, r := range All() {
		counts[r.Group]++
	}
	for _, g := range []string{GroupPrimary, GroupTools, GroupSystem} {
		if counts[g] == 0 {
			t.Errorf("rail group %q has no screens", g)
		}
	}
}

// TestToolsSubGroups verifies the C67 sub-group data layer: every Tools route maps
// to exactly one known sub-group, no non-Tools route carries one, and each declared
// sub-group is non-empty (so the rail never renders an empty sub-section header).
func TestToolsSubGroups(t *testing.T) {
	valid := map[string]bool{}
	for _, sg := range ToolsSubGroups {
		valid[sg] = true
	}
	counts := map[string]int{}
	for _, r := range All() {
		if r.Group == GroupTools {
			if !valid[r.SubGroup] {
				t.Errorf("Tools route %q has missing/unknown sub-group %q", r.Path, r.SubGroup)
				continue
			}
			counts[r.SubGroup]++
		} else if r.SubGroup != "" {
			t.Errorf("non-Tools route %q must not carry a sub-group (got %q)", r.Path, r.SubGroup)
		}
	}
	for _, sg := range ToolsSubGroups {
		if counts[sg] == 0 {
			t.Errorf("Tools sub-group %q has no screens", sg)
		}
	}
	// Sanity: the four sub-groups partition all Tools routes (nothing orphaned).
	var toolsTotal, subTotal int
	for _, r := range All() {
		if r.Group == GroupTools {
			toolsTotal++
		}
	}
	for _, n := range counts {
		subTotal += n
	}
	if toolsTotal != subTotal {
		t.Errorf("sub-groups cover %d Tools routes, want all %d", subTotal, toolsTotal)
	}
}
