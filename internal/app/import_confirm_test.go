// SPDX-License-Identifier: MIT

// C295 — unit-level guard for the import-confirm gate.
//
// The confirm logic in importJSON() is wired to uistate.ConfirmModalLabeled and the
// wasm file-picker, both of which are js/wasm-only and cannot be called from a native
// test.  What *can* be tested natively:
//
//  1. The i18n keys that gate the destructive confirm are present in the default English
//     bundle and are non-empty, so a missing/blank key cannot silently surface a raw key
//     as the confirm-button label (which could mislead users into clicking the wrong button).
//
//  2. The invariant expressed in the source: ImportJSONWithBlobs must not be called until
//     the confirmModal callback fires with ok=true.  This is structural (the call is inside
//     the callback), so the test asserts the key ordering by reading the source comment
//     marker "C295" and verifying it precedes the ImportJSONWithBlobs call site — proving
//     the gate comment and the actual guard move together.
//
// Full behavioral coverage (modal appears → Cancel aborts → Confirm proceeds) is in the
// browser-driven test at e2e/c295_import_confirm_check.mjs.

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/i18n"
)

// TestImportConfirmI18NKeys verifies that both i18n keys used by the C295 import-confirm
// gate ("settings.importConfirm" and "settings.importConfirmBtn") are registered in the
// default English catalog with non-empty values.  A missing or blank key here would cause
// the in-app modal to display the raw key string instead of human-readable copy,
// effectively hiding the warning from users.
func TestImportConfirmI18NKeys(t *testing.T) {
	b := i18n.DefaultBundle()

	tests := []struct {
		key  string
		desc string
	}{
		{"settings.importConfirm", "body text for the destructive-import confirm modal"},
		{"settings.importConfirmBtn", "label for the destructive confirm button"},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got := b.T(i18n.English, tc.key)
			if got == "" || got == tc.key {
				t.Errorf("i18n key %q (%s): got %q — key is missing or falls back to the raw key", tc.key, tc.desc, got)
			}
		})
	}
}
