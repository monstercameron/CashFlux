// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"sort"
	"strings"
)

// PickerOption describes a category, owner, or create-new choice.
type PickerOption struct {
	Value, Label string
	Create       bool
}

// PickerOptions searches assignment choices, with an optional create-new choice.
func PickerOptions(options []PickerOption, query string, create bool, label func(string) string) []PickerOption {
	needle := strings.ToLower(strings.TrimSpace(query))
	result := make([]PickerOption, 0, len(options))
	for _, option := range options {
		if option.Value != "" && (needle == "" || strings.Contains(strings.ToLower(option.Label), needle)) {
			result = append(result, option)
		}
	}
	if create && needle != "" && len(result) == 0 {
		result = append(result, PickerOption{Value: "__create_category__", Label: label(strings.TrimSpace(query)), Create: true})
	}
	return result
}

// PickerIndex clamps an invalid cursor or moves it one step with wraparound.
func PickerIndex(index, count, step int) int {
	if count <= 0 {
		return 0
	}
	if index < 0 || index >= count {
		index = 0
	}
	if step < 0 {
		if index == 0 {
			return count - 1
		}
		return index - 1
	}
	if step > 0 {
		if index == count-1 {
			return 0
		}
		return index + 1
	}
	return index
}

// SelectionIDs returns a deterministic selection key list. Map membership is
// preserved for legacy callers, including entries whose value is false.
func SelectionIDs(selected map[string]bool) []string {
	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Memo retains a pure result for a revision and filter. Callers must include the
// immutable app/workspace and currency identity in the memo's owning lifetime.
type Memo[T any] struct {
	revision int
	filter   txnfilter.Criteria
	value    T
	valid    bool
}

// Get computes once for identical immutable snapshot and filter inputs.
func (c *Memo[T]) Get(revision int, filter txnfilter.Criteria, compute func() T) T {
	if !c.valid || c.revision != revision || c.filter != filter {
		c.value = compute()
		c.revision, c.filter, c.valid = revision, filter, true
	}
	return c.value
}
