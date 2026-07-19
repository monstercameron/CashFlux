// SPDX-License-Identifier: MIT

package i18n

// notifTriageKeys holds the English strings added by the 2026-07-19 triage-queue
// pass on the Notifications and Smart surfaces (turn "108 findings / 21 alerts"
// walls into a short "Needs you" list, with the calmer items tucked under
// Watching). Merged via init so the shared en.go is never touched by this
// concurrent lane.
var notifTriageKeys = Catalog{
	// Notifications: the three-way triage view toggle (Needs you default /
	// Watching / History).
	"notifications.triageNeedsYou":  "Needs you",
	"notifications.triageWatching":  "Watching",
	"notifications.triageViewLabel": "Which notifications to show",
	// Calm empty states per triage bucket — an empty "Needs you" is a win, not a
	// dead end.
	"notifications.needsClear":    "You're all caught up — nothing needs you right now.",
	"notifications.watchingEmpty": "Nothing to keep an eye on here right now.",

	// Smart: the findings-feed triage segmentation (Needs you default / Watching).
	"smart.triageNeedsYou":  "Needs you",
	"smart.triageWatching":  "Watching",
	"smart.triageViewLabel": "Which findings to show",
	"smart.needsClear":      "Nothing needs a decision from you right now.",
	"smart.watchingEmpty":   "Nothing to keep an eye on here right now.",
}

func init() {
	for k, v := range notifTriageKeys {
		english[k] = v
	}
}
