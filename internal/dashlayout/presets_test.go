// SPDX-License-Identifier: MIT

package dashlayout

import "testing"

// TestPresetsReferenceOnlyCatalogWidgets guards every preset against typos and
// retired widgets: an id absent from DefaultItems would render an empty tile.
func TestPresetsReferenceOnlyCatalogWidgets(t *testing.T) {
	catalog := map[string]bool{}
	for _, it := range DefaultItems() {
		catalog[it.ID] = true
	}
	for _, key := range PresetKeys {
		items, ok := PresetItems(key)
		if !ok {
			t.Fatalf("PresetKeys lists %q but PresetItems doesn't know it", key)
		}
		if len(items) < 3 {
			t.Errorf("preset %q has only %d widgets", key, len(items))
		}
		seen := map[string]bool{}
		for _, it := range items {
			if !catalog[it.ID] {
				t.Errorf("preset %q references unknown widget %q", key, it.ID)
			}
			if seen[it.ID] {
				t.Errorf("preset %q lists %q twice", key, it.ID)
			}
			seen[it.ID] = true
			if it.ColSpan < 1 || it.ColSpan > 4 || it.RowSpan < 1 {
				t.Errorf("preset %q widget %q has bad spans %dx%d", key, it.ID, it.ColSpan, it.RowSpan)
			}
		}
	}
	// Unknown keys report false, and the returned slice is a copy.
	if _, ok := PresetItems("nope"); ok {
		t.Error("unknown preset key should report false")
	}
	a, _ := PresetItems("daily")
	a[0].ID = "mutated"
	b, _ := PresetItems("daily")
	if b[0].ID == "mutated" {
		t.Error("PresetItems must return a copy")
	}
}
