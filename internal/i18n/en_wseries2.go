// SPDX-License-Identifier: MIT

package i18n

import "maps"

// wSeries2Keys holds the English copy for the 2026-07-19 W-series Accounts-lane
// work: collapsible institution/group sections (C412), the account-detail balance
// chart's range picker (C413), and linking the investments experience from
// Accounts (C365). Kept in its own extension file and merged via init, mirroring
// the en_i18nsweep.go pattern so the screens layer stays at zero hardcoded copy
// (internal/screenlint ratchet).
var wSeries2Keys = Catalog{
	// C412 — collapsible group sections.
	"accountsGroup.collapse":   "Collapse group",
	"accountsGroup.expand":     "Expand group",
	"accountsGroup.toggleAria": "%s section",

	// C413 — account-detail balance chart range picker.
	"accountsRange.label":      "Balance history range",
	"accountsRange.d90":        "90d",
	"accountsRange.m12":        "12m",
	"accountsRange.all":        "All",
	"accountsRange.caption12m": "Balance, last 12 months",
	"accountsRange.captionAll": "Balance, full history",
	"accountsRange.aria12m":    "%s balance over the last 12 months",
	"accountsRange.ariaAll":    "%s balance over its full history",
	"accountsRange.flat12m":    "%s balance has not moved in the last 12 months",
	"accountsRange.flatAll":    "%s balance has not moved on record",
}

func init() {
	maps.Copy(english, wSeries2Keys)
}
