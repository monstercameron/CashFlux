// SPDX-License-Identifier: MIT

package ui

// SegmentOption describes one keyed choice in a segmented control.
type SegmentOption struct {
	Key, Label, Class, TestID, AriaLabel, Title string
	Count                                       int
	ShowCount, Disabled                         bool
}

// segmentTabStop keeps the selected key as the tab stop when it is enabled;
// otherwise the first enabled option gives an empty or changed tablist an entry.
func segmentTabStop(options []SegmentOption, selected string) string {
	first := ""
	hasFirst := false
	for _, option := range options {
		if option.Disabled {
			continue
		}
		if option.Key == selected {
			return selected
		}
		if !hasFirst {
			first = option.Key
			hasFirst = true
		}
	}
	return first
}

// segmentKeyTarget resolves horizontal tab navigation by the focused option's
// key, so a reorder never reinterprets a saved positional index as another tab.
func segmentKeyTarget(options []SegmentOption, current, key string) (string, bool) {
	step := 1
	index := -1
	switch key {
	case "Home":
	case "End":
		step, index = -1, len(options)
	case "ArrowLeft", "ArrowRight":
		if key == "ArrowLeft" {
			step, index = -1, 0
		}
		for i, option := range options {
			if option.Key == current {
				index = i
				break
			}
		}
	default:
		return "", false
	}
	for checked := 0; checked < len(options); checked++ {
		index = (index + step + len(options)) % len(options)
		if !options[index].Disabled {
			return options[index].Key, true
		}
	}
	return "", false
}
