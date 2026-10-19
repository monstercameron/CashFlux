// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestProtectedGateEnglishActions(t *testing.T) {
	bundle := DefaultBundle()
	for key, want := range map[string]string{
		"enc31.title":    "CashFlux couldn't open your protected data",
		"enc31.unlock":   "Unlock",
		"enc31.recovery": "Use recovery key",
		"enc31.help":     "Get help",
	} {
		if got := bundle.T(English, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
