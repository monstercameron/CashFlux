// SPDX-License-Identifier: MIT

package i18n

// notifyHistoryKeys holds the copy for the Notifications history/archive: the
// Live/History view toggle, the search + severity filters, the clear action, and
// the empty state. Merged via init so this file does not touch en.go.
var notifyHistoryKeys = Catalog{
	// View toggle at the top of the Notifications surface.
	"notifHistory.live":    "Live",
	"notifHistory.history": "History",
	// Search + filter controls.
	"notifHistory.searchPlaceholder": "Search notifications",
	"notifHistory.searchAria":        "Search past notifications",
	"notifHistory.filterAll":         "All severities",
	// Clear action.
	"notifHistory.clear":     "Clear history",
	"notifHistory.clearAria": "Clear all past notifications",
	// Header count. %d = number of archived notifications shown.
	"notifHistory.count": "%d in history",
	// Empty states.
	"notifHistory.empty":     "No past notifications",
	"notifHistory.emptyHint": "Alerts you've seen are kept here so you can look back on them.",
	"notifHistory.noMatch":   "No notifications match your search.",
}

func init() {
	for k, v := range notifyHistoryKeys {
		english[k] = v
	}
}
