// SPDX-License-Identifier: MIT

package ui

import (
	"strconv"
	"strings"
)

// PickerOption describes one selectable Picker or MultiPicker option.
type PickerOption struct {
	// Value is the stable machine-readable choice.
	Value string
	// Label is the visible and accessible option name.
	Label string
	// Secondary is supporting text such as a category path.
	Secondary string
	// Depth indents a hierarchical option.
	Depth int
	// Disabled prevents selection and keyboard activation.
	Disabled bool
	// Create marks an option-shaped sentinel. Picker renders its create row from OnCreate instead.
	Create bool
}

// FilterPickerOptions returns non-placeholder options matching query. Disabled
// options remain visible but PickerNextIndex skips them during navigation.
func FilterPickerOptions(options []PickerOption, query string) []PickerOption {
	query = strings.TrimSpace(strings.ToLower(query))
	filtered := make([]PickerOption, 0, len(options))
	for _, option := range options {
		if option.Value == "" || option.Create {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(option.Label), query) || strings.Contains(strings.ToLower(option.Secondary), query) {
			filtered = append(filtered, option)
		}
	}
	return filtered
}

// PickerNextIndex returns the next enabled option index after current, wrapping
// at either end. It returns -1 when there are no enabled options.
func PickerNextIndex(options []PickerOption, current, delta int) int {
	if len(options) == 0 || delta == 0 {
		return -1
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	index := current
	for range len(options) {
		index += step
		if index < 0 {
			index = len(options) - 1
		} else if index >= len(options) {
			index = 0
		}
		if !options[index].Disabled {
			return index
		}
	}
	return -1
}

// PickerActiveIndex chooses the first enabled option, or -1 when none exist.
func PickerActiveIndex(options []PickerOption) int {
	for index, option := range options {
		if !option.Disabled {
			return index
		}
	}
	return -1
}

// PickerOptionID returns the stable DOM id for an option at index.
func PickerOptionID(id string, index int) string {
	return id + "-option-" + strconv.Itoa(index)
}
