// SPDX-License-Identifier: MIT

package ui

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestVisibleItemsPreservesVisibleActionsAndSubmenus(t *testing.T) {
	input := []MenuItem[string]{
		{Value: "edit", Section: "Manage"},
		{Value: "hidden", Hidden: true},
		{Value: "link", Section: "Connect", Submenu: []MenuItem[string]{
			{Value: "bill"},
			{Value: "hidden-child", Hidden: true},
			{Value: "subscription"},
		}},
	}

	want := []MenuItem[string]{
		{Value: "edit", Section: "Manage"},
		{Value: "link", Section: "Connect", Submenu: []MenuItem[string]{
			{Value: "bill"},
			{Value: "subscription"},
		}},
	}
	if got := VisibleItems(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("VisibleItems() = %#v, want %#v", got, want)
	}
}

func TestMenuSectionsCollapsesSingletonHeadings(t *testing.T) {
	items := []MenuItem[string]{
		{Value: "one", Section: "Single"},
		{Value: "hidden", Section: "Pair", Hidden: true},
		{Value: "two", Section: "Pair"},
		{Value: "three", Section: "Pair"},
		{Value: "plain"},
		{Value: "also-plain"},
		{Value: "last", Section: "Single"},
	}
	want := []MenuSection[string]{
		{Items: []MenuItem[string]{{Value: "one", Section: "Single"}}},
		{Heading: "Pair", Items: []MenuItem[string]{
			{Value: "two", Section: "Pair"},
			{Value: "three", Section: "Pair"},
		}},
		{Items: []MenuItem[string]{{Value: "plain"}, {Value: "also-plain"}}},
		{Items: []MenuItem[string]{{Value: "last", Section: "Single"}}},
	}
	if got := MenuSections(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("MenuSections() = %#v, want %#v", got, want)
	}
}

func TestVisibleItemsNeverDropsAnActionProperty(t *testing.T) {
	// Each generated tree includes arbitrary nesting, section changes, and hidden
	// entries. The invariant is that every non-hidden action is present once and
	// in order at its original depth; filtering must never impose a menu-size cap.
	rng := rand.New(rand.NewSource(1))
	nextID := 0
	for trial := 0; trial < 500; trial++ {
		input := randomMenuItems(rng, 0, &nextID)
		got := VisibleItems(input)
		wantIDs := visibleActionIDs(input)
		gotIDs := visibleActionIDs(got)
		if !reflect.DeepEqual(gotIDs, wantIDs) {
			t.Fatalf("trial %d dropped or reordered actions: got %v, want %v", trial, gotIDs, wantIDs)
		}
		var sectionIDs []int
		for _, section := range MenuSections(input) {
			for _, item := range section.Items {
				sectionIDs = append(sectionIDs, visibleActionIDs([]MenuItem[int]{item})...)
			}
		}
		if !reflect.DeepEqual(sectionIDs, wantIDs) {
			t.Fatalf("trial %d section grouping dropped or reordered actions: got %v, want %v", trial, sectionIDs, wantIDs)
		}
	}
}

func randomMenuItems(rng *rand.Rand, depth int, nextID *int) []MenuItem[int] {
	count := rng.Intn(9)
	items := make([]MenuItem[int], 0, count)
	sections := [...]string{"", "Manage", "Report", "Delete"}
	for i := 0; i < count; i++ {
		id := *nextID
		*nextID = *nextID + 1
		item := MenuItem[int]{
			Value:   id,
			Hidden:  rng.Intn(4) == 0,
			Section: sections[rng.Intn(len(sections))],
		}
		if depth < 3 && rng.Intn(3) == 0 {
			item.Submenu = randomMenuItems(rng, depth+1, nextID)
		}
		items = append(items, item)
	}
	return items
}

func visibleActionIDs(items []MenuItem[int]) []int {
	var ids []int
	for _, item := range items {
		if item.Hidden {
			continue
		}
		ids = append(ids, item.Value)
		ids = append(ids, visibleActionIDs(item.Submenu)...)
	}
	return ids
}
