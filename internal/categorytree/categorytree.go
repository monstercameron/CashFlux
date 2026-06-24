// SPDX-License-Identifier: MIT

// Package categorytree organizes a flat category list into a parent/child
// hierarchy for display and grouping. It is defensive about bad data — an orphan
// (parent not present) is treated as a root, and cycles can never loop — so the
// UI can trust the result. Pure Go, unit-tested on native Go.
package categorytree

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Node is a category with its child categories nested beneath it.
type Node struct {
	Category domain.Category
	Children []Node
}

// Flat is a category paired with its depth in the tree (roots are depth 0), for
// rendering an indented list.
type Flat struct {
	Category domain.Category
	Depth    int
}

// Build returns the category forest: root categories (no parent, or a parent not
// in the set) each with their descendants nested. Siblings are sorted by name.
// Categories caught in a cycle are dropped rather than looping.
func Build(cats []domain.Category) []Node {
	ids := make(map[string]bool, len(cats))
	for _, c := range cats {
		ids[c.ID] = true
	}
	byParent := map[string][]domain.Category{}
	for _, c := range cats {
		parent := c.ParentID
		if parent == "" || !ids[parent] || parent == c.ID {
			parent = "" // root: no parent, missing parent, or self-reference
		}
		byParent[parent] = append(byParent[parent], c)
	}
	for k := range byParent {
		sort.SliceStable(byParent[k], func(i, j int) bool { return byParent[k][i].Name < byParent[k][j].Name })
	}

	visited := make(map[string]bool, len(cats))
	var build func(parentID string) []Node
	build = func(parentID string) []Node {
		var out []Node
		for _, c := range byParent[parentID] {
			if visited[c.ID] {
				continue // already placed (cycle guard)
			}
			visited[c.ID] = true
			out = append(out, Node{Category: c, Children: build(c.ID)})
		}
		return out
	}
	return build("")
}

// Flatten returns the tree as a depth-tagged list in display order (parent before
// its children), suitable for an indented dropdown or list.
func Flatten(cats []domain.Category) []Flat {
	var out []Flat
	var walk func(nodes []Node, depth int)
	walk = func(nodes []Node, depth int) {
		for _, n := range nodes {
			out = append(out, Flat{Category: n.Category, Depth: depth})
			walk(n.Children, depth+1)
		}
	}
	walk(Build(cats), 0)
	return out
}

// Descendants returns the set of category ids made up of rootID plus every
// category nested beneath it at any depth — so callers can roll a sub-category's
// data (spend, budget coverage) up into its parent. It is cycle-safe (each id is
// visited at most once) and returns just {rootID} when the category has no
// children. An empty rootID returns an empty set.
func Descendants(cats []domain.Category, rootID string) map[string]bool {
	if rootID == "" {
		return map[string]bool{}
	}
	byParent := make(map[string][]string, len(cats))
	for _, c := range cats {
		if c.ParentID != "" && c.ParentID != c.ID {
			byParent[c.ParentID] = append(byParent[c.ParentID], c.ID)
		}
	}
	out := make(map[string]bool, 1)
	var walk func(id string)
	walk = func(id string) {
		if out[id] {
			return // already placed — guards against cycles
		}
		out[id] = true
		for _, child := range byParent[id] {
			walk(child)
		}
	}
	walk(rootID)
	return out
}

// VisibleUnderCollapsed returns the set of category IDs that should be shown
// when some parent categories are collapsed. A category is included in the
// result only when none of its ancestors appear in the collapsed map with a
// true value. Root categories (no parent, or a missing parent) are always
// visible regardless of the collapsed map.
func VisibleUnderCollapsed(cats []domain.Category, collapsed map[string]bool) map[string]bool {
	// Build a fast parent-lookup.
	parentOf := make(map[string]string, len(cats))
	ids := make(map[string]bool, len(cats))
	for _, c := range cats {
		parentOf[c.ID] = c.ParentID
		ids[c.ID] = true
	}

	visible := make(map[string]bool, len(cats))
	for _, c := range cats {
		// Walk up the ancestor chain; hide if any ancestor is collapsed.
		hidden := false
		cur := c.ParentID
		seen := make(map[string]bool) // cycle guard
		for cur != "" && ids[cur] && !seen[cur] {
			seen[cur] = true
			if collapsed[cur] {
				hidden = true
				break
			}
			cur = parentOf[cur]
		}
		if !hidden {
			visible[c.ID] = true
		}
	}
	return visible
}

// ReparentOnDelete returns the direct children of deletedID, each re-pointed to
// deletedID's own parent, so deleting a parent category re-homes its children to
// the grandparent (or to the root when the deleted category was top-level)
// instead of leaving them with a dangling ParentID (orphaned). Only the children
// that actually need a change are returned; callers persist those.
func ReparentOnDelete(cats []domain.Category, deletedID string) []domain.Category {
	if deletedID == "" {
		return nil
	}
	var newParent string
	for _, c := range cats {
		if c.ID == deletedID {
			if c.ParentID != deletedID { // guard a self-parent cycle
				newParent = c.ParentID
			}
			break
		}
	}
	var out []domain.Category
	for _, c := range cats {
		if c.ParentID == deletedID && c.ID != deletedID {
			c.ParentID = newParent
			out = append(out, c)
		}
	}
	return out
}
