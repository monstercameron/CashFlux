// SPDX-License-Identifier: MIT

package ui

import (
	"testing"
)

// ---------------------------------------------------------------------------
// OptionsFrom
// ---------------------------------------------------------------------------

func TestOptionsFrom(t *testing.T) {
	type account struct {
		ID   string
		Name string
	}
	accounts := []account{
		{ID: "a1", Name: "Checking"},
		{ID: "a2", Name: "Savings"},
		{ID: "a3", Name: "Credit Card"},
	}

	tests := []struct {
		name     string
		items    []account
		selected string
		want     []SelectOption
	}{
		{
			name:     "nil slice produces empty result",
			items:    nil,
			selected: "",
			want:     []SelectOption{},
		},
		{
			name:     "empty slice produces empty result",
			items:    []account{},
			selected: "",
			want:     []SelectOption{},
		},
		{
			name:     "all items mapped correctly",
			items:    accounts,
			selected: "a2",
			want: []SelectOption{
				{Value: "a1", Label: "Checking"},
				{Value: "a2", Label: "Savings"},
				{Value: "a3", Label: "Credit Card"},
			},
		},
		{
			name:     "no matching selected leaves options unchanged",
			items:    accounts,
			selected: "a99",
			want: []SelectOption{
				{Value: "a1", Label: "Checking"},
				{Value: "a2", Label: "Savings"},
				{Value: "a3", Label: "Credit Card"},
			},
		},
		{
			name:     "single item",
			items:    []account{{ID: "x", Name: "Only"}},
			selected: "x",
			want:     []SelectOption{{Value: "x", Label: "Only"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := OptionsFrom(tc.items,
				func(a account) string { return a.ID },
				func(a account) string { return a.Name },
				tc.selected,
			)
			if len(got) != len(tc.want) {
				t.Fatalf("OptionsFrom() len=%d; want %d", len(got), len(tc.want))
			}
			for i, w := range tc.want {
				g := got[i]
				if g.Value != w.Value || g.Label != w.Label {
					t.Errorf("OptionsFrom()[%d] = {%q,%q}; want {%q,%q}",
						i, g.Value, g.Label, w.Value, w.Label)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// OptionsFrom with strings (simpler T type)
// ---------------------------------------------------------------------------

func TestOptionsFromStrings(t *testing.T) {
	names := []string{"Alice", "Bob", "Carol"}
	got := OptionsFrom(names,
		func(s string) string { return s },
		func(s string) string { return s + " (member)" },
		"Bob",
	)
	if len(got) != 3 {
		t.Fatalf("len=%d; want 3", len(got))
	}
	if got[1].Value != "Bob" || got[1].Label != "Bob (member)" {
		t.Errorf("got[1] = %+v; want {Bob, Bob (member)}", got[1])
	}
}

// ---------------------------------------------------------------------------
// SelectOption zero value
// ---------------------------------------------------------------------------

func TestSelectOptionZeroValue(t *testing.T) {
	var opt SelectOption
	if opt.Value != "" || opt.Label != "" {
		t.Errorf("zero SelectOption should have empty fields, got %+v", opt)
	}
}
