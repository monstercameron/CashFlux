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

	// Accounts stale badge suffix — "· 47d" → "· 47 days" (pluralized string
	// supplied by the caller). Renders in the badge pill's existing uppercase
	// style, consistent with the sibling badges (review #36).
	"accounts.staleDaysSuffix": " · %s",

	// Recurring "Post due now" — when nothing is auto-postable but items are
	// overdue, explain that those overdue items are manual and posted by hand,
	// so the "OVERDUE 3 / Post due now (0)" pairing reads honestly (review #23).
	"recurring.postDueTitleManual": "Nothing here to auto-post. The %s above must be entered by hand — open each flow to record it.",
}

func init() {
	for k, v := range uxbatch1Keys {
		english[k] = v
	}
}
