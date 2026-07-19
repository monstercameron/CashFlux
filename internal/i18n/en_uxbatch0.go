// SPDX-License-Identifier: MIT

// Package i18n — UI/UX remediation lane-B keys (2026-07-19 review).
//
// Pattern (see en_mia.go): new keys merge into the english catalog from their
// own file so concurrent sessions never contend on en.go.
package i18n

var uxBatch0Keys = Catalog{
	// First-run heads-up that background music is on by default (task #26).
	"muzak.firstRunNotice": "Background music is on — the ♪ button in the top bar turns it off.",
}

func init() {
	for k, v := range uxBatch0Keys {
		english[k] = v
	}
}
