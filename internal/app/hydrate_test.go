// SPDX-License-Identifier: MIT

package app

import "testing"

func TestDecideHydrate(t *testing.T) {
	tests := []struct {
		name         string
		datasetRaw   string
		seededBefore bool
		want         hydrateAction
	}{
		{"first run ever seeds the sample", "", false, hydrateSeed},
		{"a saved dataset is imported", `{"schemaVersion":1}`, false, hydrateImport},
		{"a saved dataset is imported even after seeding", `{"schemaVersion":1}`, true, hydrateImport},
		{"wiped (empty) after first run stays empty", "", true, hydrateEmpty},
		{"blank/whitespace dataset is treated as empty", "   ", true, hydrateEmpty},
		{"blank dataset on first run still seeds", "  ", false, hydrateSeed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideHydrate(tc.datasetRaw, tc.seededBefore); got != tc.want {
				t.Errorf("decideHydrate(%q,%v) = %d, want %d", tc.datasetRaw, tc.seededBefore, got, tc.want)
			}
		})
	}
}
