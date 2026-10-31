// SPDX-License-Identifier: MIT

// Package i18n — UX copy batch 1 overrides/additions.
//
// Pattern (mirrors en_mia.go): keys live here; init() merges them into the
// english catalog last, overriding the same keys in the dirty en.go without
// touching that file (it is under concurrent WIP). Duration abbreviations are
// spelled out and a plan sentence gains its missing word.
package i18n

var uxbatch1Keys = Catalog{
	// Reports "plan" sentence — was missing "up" after "frees" (review #12).
	"rpta.planTrimDetail": "It has been running at %s a month recently — returning to its own median frees up ≈ %s a year.",

	// Dashboard "needs attention" chip — spell out the day count in plain
	// English; the call site now passes a pluralized "N day(s)" string, so the
	// template takes %s here instead of "%dd" (review #36).
	"dashboard.attentionTaskOverdue": "%s · %s overdue",
}

func init() {
	for k, v := range uxbatch1Keys {
		english[k] = v
	}
}
