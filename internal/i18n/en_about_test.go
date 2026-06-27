// SPDX-License-Identifier: MIT

// C290/C293 — unit guard for the About & Privacy screen i18n keys.
//
// Asserts that every key used by internal/screens/about.go is present in the
// default English bundle and resolves to a non-empty, non-raw-key value.
// A missing or blank key would render the raw key string in the UI.
package i18n

import (
	"testing"
)

func TestAboutI18NKeys(t *testing.T) {
	b := DefaultBundle()

	keys := []struct {
		key  string
		desc string
	}{
		// Identity card.
		{"about.headingIdentity", "app name heading"},
		{"about.tagline", "app tagline"},

		// Privacy card (C290, C293).
		{"about.headingPrivacy", "privacy section heading"},
		{"about.privacyLocalFirst", "local-first data statement"},
		{"about.privacyExport", "data export assurance"},
		{"about.privacyNoTracking", "no-tracking statement"},

		// Cloud-sync card (C291).
		{"about.headingCloudSync", "cloud sync section heading"},
		{"about.cloudSyncOff", "cloud sync off by default"},
		{"about.cloudSyncOn", "what syncs when on"},
		{"about.cloudSyncControl", "user control statement"},

		// AI card (C292).
		{"about.headingAI", "AI section heading"},
		{"about.aiKeyOwnKey", "bring-your-own-key statement"},
		{"about.aiKeyStorage", "key stored locally statement"},
		{"about.aiKeyUsage", "when data goes to OpenAI"},
		{"about.aiKeySettings", "link to AI settings"},

		// Version card.
		{"about.headingVersion", "version section heading"},
		{"about.versionLabel", "version label"},
		{"about.changelogLink", "changelog link label"},
		{"about.changelogHref", "changelog URL"},
		{"about.sourceLink", "GitHub source link label"},
		{"about.sourceHref", "GitHub source URL"},
		{"about.licenseNote", "license note"},
		{"about.licenseLink", "license link label"},
		{"about.licenseHref", "license URL"},
	}

	for _, tc := range keys {
		t.Run(tc.key, func(t *testing.T) {
			got := b.T(English, tc.key)
			if got == "" || got == tc.key {
				t.Errorf("i18n key %q (%s): got %q — missing or falls back to raw key", tc.key, tc.desc, got)
			}
		})
	}
}
