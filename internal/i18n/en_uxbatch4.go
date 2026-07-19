// SPDX-License-Identifier: MIT

// Package i18n — UX review batch #4 keys.
//
// Pattern (mirrors en_mia.go): add new keys here only; init() merges them into
// the english catalog without touching the dirty en.go which is under concurrent
// WIP.
package i18n

var uxbatch4Keys = Catalog{
	// Per-row overflow (⋯) menu labels for the subscriptions and bills rows,
	// where the rarer actions moved behind a kebab (reviews #20 / #21).
	"subs.moreActions":  "More actions",
	"bills.moreActions": "More actions",
}

func init() {
	for k, v := range uxbatch4Keys {
		english[k] = v
	}
}
