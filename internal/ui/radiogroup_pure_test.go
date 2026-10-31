// SPDX-License-Identifier: MIT

package ui

import "testing"

func TestRadioKeyTarget(t *testing.T) {
	options := []RadioOption{
		{Value: "all"},
		{Value: "assets"},
		{Value: "off", Disabled: true},
		{Value: "liabilities"},
	}
	tests := []struct {
		name    string
		current string
		key     string
		want    string
		handled bool
	}{
		{"right moves to the next enabled option", "assets", "ArrowRight", "liabilities", true},
		{"right skips a disabled option and wraps", "liabilities", "ArrowRight", "all", true},
		{"left wraps to the last enabled option", "all", "ArrowLeft", "liabilities", true},
		{"down acts as right", "all", "ArrowDown", "assets", true},
		{"up acts as left", "assets", "ArrowUp", "all", true},
		{"home goes to the first", "liabilities", "Home", "all", true},
		{"end goes to the last enabled", "all", "End", "liabilities", true},
		{"an unselected group starts from the first on right", "", "ArrowRight", "all", true},
		{"an unselected group starts from the last on left", "", "ArrowLeft", "liabilities", true},
		{"other keys are not handled", "all", "a", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, handled := RadioKeyTarget(options, tt.current, tt.key)
			if got != tt.want || handled != tt.handled {
				t.Errorf("RadioKeyTarget(%q, %q) = (%q, %v), want (%q, %v)", tt.current, tt.key, got, handled, tt.want, tt.handled)
			}
		})
	}
}

func TestRadioKeyTargetWithNoEnabledOption(t *testing.T) {
	options := []RadioOption{{Value: "a", Disabled: true}}
	if _, handled := RadioKeyTarget(options, "a", "ArrowRight"); handled {
		t.Error("a group with no enabled option should not handle keys")
	}
}
