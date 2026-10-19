// SPDX-License-Identifier: MIT

package ui

import (
	"reflect"
	"testing"
)

func TestFilterPickerOptions(t *testing.T) {
	options := []PickerOption{
		{Value: "", Label: "Category"},
		{Value: "food", Label: "Food", Secondary: "Home / Food"},
		{Value: "rent", Label: "Rent"},
		{Value: "disabled", Label: "Food disabled", Disabled: true},
		{Value: "create", Label: "Create", Create: true},
	}
	for _, test := range []struct {
		name  string
		query string
		want  []string
	}{
		{name: "all non-placeholder options", want: []string{"food", "rent", "disabled"}},
		{name: "case-insensitive label match", query: "fOo", want: []string{"food", "disabled"}},
		{name: "secondary path match", query: "home", want: []string{"food"}},
		{name: "no match", query: "missing", want: []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotOptions := FilterPickerOptions(options, test.query)
			got := make([]string, len(gotOptions))
			for index, option := range gotOptions {
				got[index] = option.Value
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("FilterPickerOptions(%q) = %v, want %v", test.query, got, test.want)
			}
		})
	}
}

func TestPickerNavigation(t *testing.T) {
	options := []PickerOption{{Value: "a"}, {Value: "b", Disabled: true}, {Value: "c"}}
	for _, test := range []struct {
		name    string
		current int
		delta   int
		want    int
	}{
		{name: "first forward", current: -1, delta: 1, want: 0},
		{name: "skip disabled", current: 0, delta: 1, want: 2},
		{name: "wrap forward", current: 2, delta: 1, want: 0},
		{name: "wrap backward", current: 0, delta: -1, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := PickerNextIndex(options, test.current, test.delta); got != test.want {
				t.Fatalf("PickerNextIndex() = %d, want %d", got, test.want)
			}
		})
	}
	if got := PickerActiveIndex([]PickerOption{{Disabled: true}}); got != -1 {
		t.Fatalf("PickerActiveIndex() = %d, want -1", got)
	}
}

func TestPickerOptionID(t *testing.T) {
	if got := PickerOptionID("category", 12); got != "category-option-12" {
		t.Fatalf("PickerOptionID() = %q", got)
	}
}
