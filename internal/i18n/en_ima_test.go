// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestIMAStagedImportCopy(t *testing.T) {
	bundle := DefaultBundle()
	for key := range imaKeys {
		if got := bundle.T(English, key); got == "" || got == key {
			t.Errorf("missing staged import copy %s", key)
		}
	}
}
