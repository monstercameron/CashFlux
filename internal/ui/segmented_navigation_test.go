// SPDX-License-Identifier: MIT

package ui

import "testing"

func TestSegmentKeyboardTargetsSkipDisabledWrapAndUseCurrentKeys(t *testing.T) {
	options := []SegmentOption{{Key: "a"}, {Key: "disabled", Disabled: true}, {Key: "b"}, {Key: "c"}}
	for _, tc := range []struct {
		current, key, want string
		ok                 bool
	}{
		{"a", "ArrowRight", "b", true},
		{"b", "ArrowLeft", "a", true},
		{"c", "ArrowRight", "a", true},
		{"a", "ArrowLeft", "c", true},
		{"b", "Home", "a", true},
		{"a", "End", "c", true},
		{"missing", "ArrowRight", "a", true},
		{"missing", "ArrowLeft", "c", true},
		{"disabled", "ArrowRight", "b", true},
		{"a", "ArrowDown", "", false},
		{"a", "Enter", "", false},
	} {
		t.Run(tc.current+"/"+tc.key, func(t *testing.T) {
			if got, ok := segmentKeyTarget(options, tc.current, tc.key); got != tc.want || ok != tc.ok {
				t.Fatalf("target=(%q,%t), want (%q,%t)", got, ok, tc.want, tc.ok)
			}
		})
	}
	for _, options := range [][]SegmentOption{nil, {{Key: "a", Disabled: true}}} {
		for _, key := range []string{"ArrowLeft", "ArrowRight", "Home", "End"} {
			if _, ok := segmentKeyTarget(options, "a", key); ok {
				t.Fatal("a list without enabled choices produced a focus target")
			}
		}
	}
	reordered := []SegmentOption{options[3], options[0], options[2]}
	if got, _ := segmentKeyTarget(reordered, "b", "ArrowRight"); got != "c" {
		t.Fatal("reordered navigation used a stale positional index")
	}
}

func TestSegmentTabStopPreservesEnabledSelectionAndHandlesEmptyKeys(t *testing.T) {
	options := []SegmentOption{{Key: "", Disabled: false}, {Key: "disabled", Disabled: true}, {Key: "b"}}
	for _, tc := range []struct{ selected, want string }{{"b", "b"}, {"disabled", ""}, {"missing", ""}, {"", ""}} {
		if got := segmentTabStop(options, tc.selected); got != tc.want {
			t.Fatalf("selected %q tab stop %q, want %q", tc.selected, got, tc.want)
		}
	}
}
