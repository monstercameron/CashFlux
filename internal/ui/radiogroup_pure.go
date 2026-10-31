// SPDX-License-Identifier: MIT

package ui

import "github.com/monstercameron/CashFlux/internal/icon"

// RadioOption is one choice in a RadioGroup.
type RadioOption struct {
	// Value is what the group reports when this option is chosen.
	Value string
	// Label is the visible text, or the accessible name when Icon is set.
	Label string
	// Icon, when set, shows a glyph in place of the text. Label still names the option
	// for assistive technology and as a tooltip.
	Icon icon.Name
	// TestID is an optional test id for the option.
	TestID string
	// Disabled keeps the option visible but not selectable, and skips it with the arrow keys.
	Disabled bool
}

// RadioKeyTarget returns the value a key moves the selection to from the current value.
// Arrow keys step through the enabled options and wrap at the ends, and Home and End
// jump to the first and last enabled option. handled is false for keys a radio group
// does not use, and when no option is enabled.
func RadioKeyTarget(options []RadioOption, current, key string) (value string, handled bool) {
	enabled := make([]int, 0, len(options))
	pos := -1
	for i, option := range options {
		if option.Disabled {
			continue
		}
		if option.Value == current {
			pos = len(enabled)
		}
		enabled = append(enabled, i)
	}
	if len(enabled) == 0 {
		return "", false
	}
	last := len(enabled) - 1
	switch key {
	case "ArrowRight", "ArrowDown":
		if pos < 0 || pos == last {
			return options[enabled[0]].Value, true
		}
		return options[enabled[pos+1]].Value, true
	case "ArrowLeft", "ArrowUp":
		if pos <= 0 {
			return options[enabled[last]].Value, true
		}
		return options[enabled[pos-1]].Value, true
	case "Home":
		return options[enabled[0]].Value, true
	case "End":
		return options[enabled[last]].Value, true
	}
	return "", false
}
