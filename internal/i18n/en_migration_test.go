// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestLegacyMigrationEnglishCopy(t *testing.T) {
	bundle := DefaultBundle()
	for _, key := range []string{"migration.title", "migration.intro", "migration.enableLock", "migration.syncNeeded", "migration.lockFailed"} {
		if text := bundle.T(English, key); text == key || strings.TrimSpace(text) == "" {
			t.Errorf("missing English copy for %s", key)
		}
	}
	if got := bundle.T(English, "bv6d.unsaved"); got != "Changes aren't saved yet" {
		t.Fatalf("save failure copy duplicates Retry: %q", got)
	}
}
