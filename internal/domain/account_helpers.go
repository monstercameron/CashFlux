// SPDX-License-Identifier: MIT

package domain

import (
	"sort"
	"strings"
)

// UniqueInstitutions returns a sorted, deduplicated list of institution names
// drawn from the given accounts. Names are trimmed and blank ones are skipped.
// When two accounts carry the same institution name differing only in case, the
// first-seen casing is preserved and the later occurrence is suppressed. The
// returned slice is sorted case-insensitively.
func UniqueInstitutions(accounts []Account) []string {
	names := make([]string, 0, len(accounts))
	for _, a := range accounts {
		names = append(names, a.Institution)
	}
	return UniqueNames(names)
}

// UniqueNames trims the names, drops blank ones, removes case-insensitive
// duplicates keeping the first-seen casing, and returns the rest sorted with
// NameLess.
func UniqueNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	var result []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, name)
	}
	sort.SliceStable(result, func(i, j int) bool { return NameLess(result[i], result[j]) })
	return result
}
