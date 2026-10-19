// SPDX-License-Identifier: MIT

// Package ui contains pure helpers shared by CashFlux UI components.
package ui

// MenuItem is the platform-independent shape of one menu action. Value carries
// the caller's action data unchanged; Hidden controls visibility; Section groups
// adjacent items; and Submenu preserves nested actions.
type MenuItem[T any] struct {
	Value   T
	Hidden  bool
	Section string
	Submenu []MenuItem[T]
}

// MenuSection is a contiguous group of visible menu items. Heading is empty for
// unsectioned items and for single-item sections, whose heading is intentionally
// collapsed to keep a menu concise.
type MenuSection[T any] struct {
	Heading string
	Items   []MenuItem[T]
}

// VisibleItems removes explicitly hidden items while preserving the order,
// values, section names, and submenu structure of every remaining action.
// Hidden submenu entries are filtered recursively. A visible parent remains
// visible even when all of its submenu entries are hidden.
func VisibleItems[T any](items []MenuItem[T]) []MenuItem[T] {
	visible := make([]MenuItem[T], 0, len(items))
	for _, item := range items {
		if item.Hidden {
			continue
		}
		if item.Submenu != nil {
			item.Submenu = VisibleItems(item.Submenu)
		}
		visible = append(visible, item)
	}
	return visible
}

// MenuSections groups adjacent visible items with the same Section value.
// Unsectioned items form their own groups. A heading is shown only for a group
// with at least two visible items, so hiding actions cannot leave a lonely
// section heading. Item order and submenu structure are preserved.
func MenuSections[T any](items []MenuItem[T]) []MenuSection[T] {
	visible := VisibleItems(items)
	sections := make([]MenuSection[T], 0, len(visible))
	for _, item := range visible {
		if len(sections) == 0 || sections[len(sections)-1].Heading != item.Section ||
			(item.Section == "" && sections[len(sections)-1].Heading != "") {
			sections = append(sections, MenuSection[T]{Heading: item.Section})
		}
		section := &sections[len(sections)-1]
		section.Items = append(section.Items, item)
	}
	for i := range sections {
		if len(sections[i].Items) == 1 {
			sections[i].Heading = ""
		}
	}
	return sections
}
