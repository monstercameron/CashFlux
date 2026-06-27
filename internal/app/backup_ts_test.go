// SPDX-License-Identifier: MIT

// C299 — unit-level guard for the "Last backed up" timestamp line.
//
// The view helper lastBackupSummary() and the loadLastBackup() reader call
// browserstore (localStorage), which is js/wasm-only, so the full round-trip
// cannot be exercised in a native test.  What can be tested natively:
//
//  1. The i18n keys that drive the timestamp line are present in the default
//     English bundle with non-empty values, so the UI never falls through to
//     the raw key string.
//
//  2. The keys are correctly formatted: the "backed-up" key includes a %s
//     verb (the formatted date is interpolated there at runtime), and the
//     "never" key does not (it is a fixed friendly string).
//
// Full behavioral coverage (export → "Last backed up" appears) is in the
// browser-driven test at e2e/c299_last_backup_check.mjs.

package app

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/i18n"
)

// TestLastBackupI18NKeys verifies that the two C299 i18n keys are registered
// in the default English catalog with non-empty values and correct formatting.
func TestLastBackupI18NKeys(t *testing.T) {
	b := i18n.DefaultBundle()

	tests := []struct {
		key     string
		desc    string
		hasVerb bool // true → value must contain %s for date interpolation
	}{
		{
			key:     "settings.lastBackup",
			desc:    "timestamp line shown after a backup has been taken",
			hasVerb: true,
		},
		{
			key:     "settings.lastBackupNever",
			desc:    "nudge shown when the user has never exported a backup",
			hasVerb: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got := b.T(i18n.English, tc.key)
			if got == "" || got == tc.key {
				t.Errorf("i18n key %q (%s): got %q — key is missing or falls back to the raw key", tc.key, tc.desc, got)
			}
			if tc.hasVerb && !strings.Contains(got, "%s") {
				t.Errorf("i18n key %q: expected a %%s verb for date interpolation, got %q", tc.key, got)
			}
			if !tc.hasVerb && strings.Contains(got, "%s") {
				t.Errorf("i18n key %q: unexpected %%s verb in a static string: %q", tc.key, got)
			}
		})
	}
}
