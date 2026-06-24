// SPDX-License-Identifier: MIT

package dashlayout

import "testing"

func TestReconcileKeepsCustomCards(t *testing.T) {
	saved := []Item{
		{ID: "kpi-networth", ColSpan: 1, RowSpan: 1},
		{ID: "wb:my chart", ColSpan: 2, RowSpan: 2},
	}
	got := Reconcile(saved)
	found := false
	for _, it := range got {
		if it.ID == "wb:my chart" {
			found = true
			if it.ColSpan != 2 || it.RowSpan != 2 {
				t.Errorf("custom card lost its spans: %+v", it)
			}
		}
	}
	if !found {
		t.Error("Reconcile dropped a published custom card (wb: id)")
	}
	for _, it := range Reconcile([]Item{{ID: "retired-widget"}}) {
		if it.ID == "retired-widget" {
			t.Error("Reconcile kept a retired built-in id")
		}
	}
}

func TestIsCustomID(t *testing.T) {
	if !IsCustomID("wb:my chart") || IsCustomID("kpi-networth") || IsCustomID("recent") {
		t.Error("IsCustomID misclassified an id")
	}
}

func TestReconcileSurfacesNewWidgetAtTopAndKeepsOrder(t *testing.T) {
	// A realistic saved layout from before "attention" existed: every other widget
	// is present (the user reordered the front and widened todo), only attention is
	// missing.
	var saved []Item
	for _, d := range DefaultItems() {
		if d.ID == "attention" {
			continue
		}
		if d.ID == "todo" {
			d.ColSpan = 2 // user widened it
		}
		saved = append(saved, d)
	}
	got := Reconcile(saved)

	// attention is the only missing default → prepended at the top.
	if got[0].ID != "attention" {
		t.Fatalf("attention should surface at the top, got %v", idsOf(got))
	}
	if got[0].ColSpan != 4 || got[0].RowSpan != 1 {
		t.Fatalf("attention should adopt its 4x1 default span, got %dx%d", got[0].ColSpan, got[0].RowSpan)
	}
	// The user's items, order, and widened todo are preserved after it.
	if want := idsOf(saved); !equalStrings(idsOf(got[1:]), want) {
		t.Fatalf("saved order not preserved: got %v want %v", idsOf(got[1:]), want)
	}
	for _, it := range got {
		if it.ID == "todo" && it.ColSpan != 2 {
			t.Fatalf("user-set span lost on todo: %dx%d", it.ColSpan, it.RowSpan)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestReconcileDropsUnknownIDs(t *testing.T) {
	saved := []Item{
		{ID: "attention", ColSpan: 4, RowSpan: 1},
		{ID: "retired-widget", ColSpan: 1, RowSpan: 1},
		{ID: "todo", ColSpan: 1, RowSpan: 1},
	}
	got := Reconcile(saved)
	for _, it := range got {
		if it.ID == "retired-widget" {
			t.Fatalf("unknown id should be dropped, got %v", idsOf(got))
		}
	}
}

func TestReconcileIdempotentOnDefaults(t *testing.T) {
	got := Reconcile(DefaultItems())
	if len(got) != len(DefaultItems()) {
		t.Fatalf("reconciling the defaults changed the count: %d vs %d", len(got), len(DefaultItems()))
	}
	for i, it := range got {
		if it.ID != DefaultItems()[i].ID {
			t.Fatalf("reconciling the defaults reordered them at %d: %v", i, idsOf(got))
		}
	}
}
