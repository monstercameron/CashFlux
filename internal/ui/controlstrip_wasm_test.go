// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestControlStripGroupsControlsAndActionsWithAccessibleLabel(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ControlStrip(ControlStripProps{
		Label:         "Goal controls",
		Class:         "goal-strip",
		ControlsClass: "filter-strip-controls inline-flex",
		ActionsClass:  "filter-strip-actions goal-actions",
		Controls:      []uic.Node{Span(Attr("data-testid", "sort-control"), "Sort by")},
		Actions:       []uic.Node{Button(Attr("data-testid", "add-action"), "Add goal")},
		Attrs:         []any{Attr("data-testid", "goal-strip"), Attr("data-view", "active")},
	}))

	root := controlStripByTestID(f, "goal-strip")
	if root == nil {
		t.Fatal("control strip root is missing")
	}
	if got := root.Attr("class"); got != "filter-strip goal-strip" {
		t.Fatalf("outer class = %q, want filter-strip goal-strip", got)
	}
	if got := root.Attr("role"); got != "group" {
		t.Fatalf("role = %q, want group", got)
	}
	if got := root.Attr("aria-label"); got != "Goal controls" {
		t.Fatalf("aria-label = %q, want Goal controls", got)
	}
	if got := root.Attr("data-view"); got != "active" {
		t.Fatalf("custom attr = %q, want active", got)
	}

	groups := root.Children()
	if len(groups) != 2 || groups[0].Tag() != "div" || groups[1].Tag() != "div" {
		t.Fatalf("root children = %d groups, want separate controls and actions groups", len(groups))
	}
	if got := groups[0].Attr("class"); got != "filter-strip-controls inline-flex" {
		t.Fatalf("controls class = %q", got)
	}
	if got := groups[1].Attr("class"); got != "filter-strip-actions goal-actions" {
		t.Fatalf("actions class = %q", got)
	}
	if len(groups[0].Children()) != 1 || groups[0].Children()[0].Attr("data-testid") != "sort-control" {
		t.Fatal("controls node was not retained in the controls group")
	}
	if len(groups[1].Children()) != 1 || groups[1].Children()[0].Attr("data-testid") != "add-action" {
		t.Fatal("action node was not retained in the actions group")
	}
}

func TestControlStripDefaultsClassesAndOmitsEmptyGroups(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ControlStrip(ControlStripProps{
		Controls: []uic.Node{Span("Filter")},
		Actions:  []uic.Node{Button("Add")},
		Attrs:    []any{Attr("data-testid", "default-strip")},
	}))
	root := controlStripByTestID(f, "default-strip")
	if got := root.Attr("class"); got != "filter-strip" {
		t.Fatalf("default outer class = %q, want filter-strip", got)
	}
	groups := root.Children()
	if len(groups) != 2 || groups[0].Attr("class") != "filter-strip-controls" || groups[1].Attr("class") != "filter-strip-controls" {
		t.Fatalf("default groups = %#v, want controls and actions using filter-strip-controls", groups)
	}
	if root.Attr("role") != "" || root.Attr("aria-label") != "" {
		t.Fatal("unlabeled strip should not add an accessible group role or label")
	}

	f.Rerender(ControlStrip(ControlStripProps{Attrs: []any{Attr("data-testid", "default-strip")}}))
	root = controlStripByTestID(f, "default-strip")
	if len(root.Children()) != 0 {
		t.Fatalf("empty strip rendered %d group wrappers, want none", len(root.Children()))
	}
}

func controlStripByTestID(f *render.Fixture, testID string) *render.QueryNode {
	for _, tag := range []string{"div", "span", "button"} {
		for _, node := range f.AllByTag(tag) {
			if node.Attr("data-testid") == testID {
				return node
			}
		}
	}
	return nil
}
