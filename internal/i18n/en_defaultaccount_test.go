// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestDefaultAccountEnglishCopyIsPresent(t *testing.T) {
	bundle := DefaultBundle()
	for key, want := range defaultAccountKeys {
		if got := bundle.T(English, key); got == key || got == "" {
			t.Errorf("English default-account copy %q is missing", key)
		} else if got != want {
			t.Errorf("English default-account copy %q = %q, want %q", key, got, want)
		}
	}
}
