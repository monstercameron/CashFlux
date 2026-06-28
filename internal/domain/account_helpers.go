// SPDX-License-Identifier: MIT

package domain

import (
	"sort"
	"strings"
)

// UniqueInstitutions returns a sorted, deduplicated list of institution names
// drawn from the given accounts. Empty Institution values are skipped. When
// two accounts carry the same institution name differing only in case, the
// first-seen casing is preserved and the later occurrence is suppressed.
// The returned slice is sorted case-insensitively.
func UniqueInstitutions(accounts []Account) []string {
	// seen maps lowercased name → original-casing first occurrence.
	seen := make(map[string]string, len(accounts))
	for _, a := range accounts {
		if a.Institution == "" {
			continue
		}
		key := strings.ToLower(a.Institution)
		if _, exists := seen[key]; !exists {
			seen[key] = a.Institution
		}
	}

	// Collect originals and sort by their lowercased form.
	result := make([]string, 0, len(seen))
	for _, orig := range seen {
		result = append(result, orig)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i]) < strings.ToLower(result[j])
	})
	return result
}
