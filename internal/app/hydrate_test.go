// SPDX-License-Identifier: MIT

package app

import "testing"

func TestDecideHydrate(t *testing.T) {
	tests := []struct {
		name         string
		datasetRaw   string
		seededBefore bool
		allowSample  bool
		want         hydrateAction
	}{
		{"portable first run seeds the sample", "", false, true, hydrateSeed},
		{"a saved dataset is imported", `{"schemaVersion":1}`, false, true, hydrateImport},
		{"a saved dataset is imported even after seeding", `{"schemaVersion":1}`, true, true, hydrateImport},
		{"portable wipe stays empty", "", true, true, hydrateEmpty},
		{"blank portable dataset after setup stays empty", "   ", true, true, hydrateEmpty},
		{"blank portable first run still seeds", "  ", false, true, hydrateSeed},
		{"hosted first run stays empty", "", false, false, hydrateEmpty},
		{"hosted blank store stays empty regardless of seed flag", "  ", true, false, hydrateEmpty},
		{"hosted saved data still imports", `{"schemaVersion":1}`, false, false, hydrateImport},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideHydrate(tc.datasetRaw, tc.seededBefore, tc.allowSample); got != tc.want {
				t.Errorf("decideHydrate(%q,%v,%v) = %d, want %d", tc.datasetRaw, tc.seededBefore, tc.allowSample, got, tc.want)
			}
		})
	}
}
