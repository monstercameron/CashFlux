// SPDX-License-Identifier: MIT

package i18n

// recordSurfaceKeys holds the English strings for the redesigned /activity
// change-record surface: the hero (changes on record + figure chips + the
// plain-English takeaway) and the day-grouped timeline. Merged via init so
// this file does not touch en.go.
var recordSurfaceKeys = Catalog{
	"activity.heroTitle":    "The record",
	"activity.heroLabel":    "Changes on record",
	"activity.eyebrowTail":  "newest first · every change is undoable in order",
	"activity.chipShown":    "Shown",
	"activity.chipYou":      "By you",
	"activity.chipOthers":   "By others",
	"activity.chipUndo":     "Undo available",
	"activity.chipUndoYes":  "Yes",
	"act.takeEmpty":         "Nothing on record yet — changes appear here as you work.",
	"act.takeLast":          "Most recently: %s — %s.",
	"activity.timelineTitle": "What changed",
	"activity.dayUndated":    "Undated",
	"activity.moreChanges":   "+%d more changes",
}

func init() {
	for k, v := range recordSurfaceKeys {
		english[k] = v
	}
}
