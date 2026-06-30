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

// The curated default layout must be a strict, duplicate-free subset of the full
// catalog — every seeded tile is a real, registrable widget.
func TestDefaultLayoutItemsSubsetOfCatalog(t *testing.T) {
	catalog := make(map[string]bool, len(DefaultItems()))
	for _, d := range DefaultItems() {
		catalog[d.ID] = true
	}
	seen := map[string]bool{}
	for _, it := range DefaultLayoutItems() {
		if !catalog[it.ID] {
			t.Errorf("curated layout has %q, which is not in the catalog", it.ID)
		}
		if seen[it.ID] {
			t.Errorf("curated layout lists %q more than once", it.ID)
		}
		seen[it.ID] = true
	}
	if len(DefaultLayoutItems()) >= len(DefaultItems()) {
		t.Errorf("curated layout (%d) should be smaller than the catalog (%d) — it deduplicates",
			len(DefaultLayoutItems()), len(DefaultItems()))
	}
}

// The curated default layout must tile the 4-column grid with no empty cells —
// every row between the top and the last placed tile is fully filled — so the
// dashboard never shows a dead gap in its default arrangement.
func TestDefaultLayoutPacksGapFree(t *testing.T) {
	const cols = 4
	placed := Pack(DefaultLayoutItems(), cols)
	occupied := map[[2]int]bool{}
	maxRow, area := 0, 0
	for _, p := range placed {
		for dr := 0; dr < p.RowSpan; dr++ {
			for dc := 0; dc < p.ColSpan; dc++ {
				occupied[[2]int{p.Row + dr, p.Col + dc}] = true
			}
			if p.Row+dr > maxRow {
				maxRow = p.Row + dr
			}
		}
		area += p.ColSpan * p.RowSpan
	}
	if area != maxRow*cols {
		t.Fatalf("curated layout leaves gaps: covers %d cells but spans %d rows × %d cols = %d", area, maxRow, cols, maxRow*cols)
	}
	for r := 1; r <= maxRow; r++ {
		for c := 1; c <= cols; c++ {
			if !occupied[[2]int{r, c}] {
				t.Errorf("empty cell at row %d, col %d in the default layout", r, c)
			}
		}
	}
}

// Reconcile is stable on the curated layout, and never auto-adds a catalog-only
// widget the curated layout deliberately omits (e.g. the hero-duplicate KPIs).
func TestReconcileHonorsCuratedOmissions(t *testing.T) {
	got := Reconcile(DefaultLayoutItems())
	if !equalStrings(idsOf(got), idsOf(DefaultLayoutItems())) {
		t.Fatalf("reconciling the curated layout changed it: %v", idsOf(got))
	}
	// A widget omitted from the curated layout but present in the catalog.
	curated := map[string]bool{}
	for _, it := range DefaultLayoutItems() {
		curated[it.ID] = true
	}
	var omitted string
	for _, d := range DefaultItems() {
		if !curated[d.ID] {
			omitted = d.ID
			break
		}
	}
	if omitted == "" {
		t.Skip("no catalog-only widget to check")
	}
	for _, it := range got {
		if it.ID == omitted {
			t.Fatalf("Reconcile re-added the curated-omitted widget %q", omitted)
		}
	}
}
