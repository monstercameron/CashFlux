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

// TestToolsSubGroups verifies the C67 navigation sub-group data layer: every
// on-rail route maps to exactly one known sub-group, and each declared sub-group
// has at least one route. The current rail uses sub-groups for every primary and
// Tools route, with the system routes also grouped under Setup.
func TestToolsSubGroups(t *testing.T) {
	valid := map[string]bool{}
	for _, sg := range ToolsSubGroups {
		valid[sg] = true
	}
	counts := map[string]int{}
	for _, r := range All() {
		if r.Group != "" {
			if !valid[r.SubGroup] {
				t.Errorf("on-rail route %q has missing/unknown sub-group %q", r.Path, r.SubGroup)
				continue
			}
			counts[r.SubGroup]++
		}
	}
	for _, sg := range ToolsSubGroups {
		if counts[sg] == 0 {
			t.Errorf("Tools sub-group %q has no screens", sg)
		}
	}
	// Sanity: the declared sub-groups cover every on-rail route (nothing orphaned).
	var railTotal, subTotal int
	for _, r := range All() {
		if r.Group != "" {
			railTotal++
		}
	}
	for _, n := range counts {
		subTotal += n
	}
	if railTotal != subTotal {
		t.Errorf("sub-groups cover %d on-rail routes, want all %d", subTotal, railTotal)
	}
}
