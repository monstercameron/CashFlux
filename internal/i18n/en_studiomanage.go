// SPDX-License-Identifier: MIT

package i18n

// studioManageKeys holds the English strings for the redesigned Manage-widgets
// surface (the Studio tab and /widget-manager): the masthead, the widget
// ledger, and the live board map. Merged via init so this file does not touch
// en.go.
var studioManageKeys = Catalog{
	"wman.eyebrow":      "Studio",
	"wman.title":        "Your dashboard, arranged",
	"wman.lede":         "Everything on your dashboard in one place — show, hide, resize, and reorder, and the board follows along live.",
	"wman.mapLabel":     "The board",
	"wman.mapHint":      "Your dashboard at true size, in live order. Click a tile to find its row.",
	"wman.visibleCount": "%d of %d visible",
	"wman.hiddenTag":    "Hidden",
	"wman.jumpTo":       "Find %s in the list",
}

func init() {
	for k, v := range studioManageKeys {
		english[k] = v
	}
}
