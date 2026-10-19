// SPDX-License-Identifier: MIT
//go:build js && wasm

package app

import "testing"

func TestTV8JRailLabelNormalizesLegacyUppercase(t *testing.T) {
	for input, want := range map[string]string{
		"PINNED":     "Pinned",
		"DAY TO DAY": "Day to day",
		" PLAN ":     "Plan",
	} {
		if got := railLabel(input); got != want {
			t.Errorf("railLabel(%q) = %q, want %q", input, got, want)
		}
	}
}
