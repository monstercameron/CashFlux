// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
)

// The screen registry drives BOTH routing (app.Run registers one route per entry)
// and the left rail (derived from Group). If an entry is malformed — duplicate or
// non-rooted path, missing view — the route either fails to register (falling
// through to the "*" catch-all, i.e. "not navigable") or renders blank. These
// invariants guard against that.
//
// A route with an empty Group is OFF-RAIL by design (e.g. /plans, /setup,
// /duplicates — reached via CTAs, not the nav rail); it needs a Path, Title, and
// View but no rail Label/Group. Only ON-rail routes (Group set) must carry a valid
// group and a rail Label.

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
		// Every route needs a Title (the page header i18n key).
		if strings.TrimSpace(r.Title) == "" {
			t.Errorf("route %q is missing its Title i18n key", r.Path)
		}
		// On-rail routes (Group set) additionally need a valid group + rail Label.
		if r.Group != "" {
			if !groups[r.Group] {
				t.Errorf("route %q has unknown rail group %q — it would not appear in the nav", r.Path, r.Group)
			}
			if strings.TrimSpace(r.Label) == "" {
				t.Errorf("on-rail route %q is missing its rail Label i18n key", r.Path)
			}
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
