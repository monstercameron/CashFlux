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
}

func init() {
	maps.Copy(english, wSeries2Keys)
}
