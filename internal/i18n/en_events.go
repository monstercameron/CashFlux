// SPDX-License-Identifier: MIT

package i18n

// eventsKeys holds the English strings for Events (TX10): a first-class spending
// event (a trip, a project) that transactions are mapped to. Defined in their own
// file and merged via init so this does not touch the shared en.go.
var eventsKeys = Catalog{
	"nav.events":           "Events",
	"screen.eventsSub":     "Group a trip or project's spending",
	"events.title":         "Events",
	"events.add":           "Add event",
	"events.addTitle":      "Create a new spending event",
	"events.empty":         "No events yet. Create one to group a trip or project's spending.",
	"events.name":          "Name",
	"events.namePh":        "Portugal trip",
	"events.start":         "Start date",
	"events.end":           "End date",
	"events.endHint":       "Leave blank for an open-ended event.",
	"events.note":          "Note",
	"events.notePh":        "Optional description",
	"events.save":          "Save event",
	"events.cancel":        "Cancel",
	"events.edit":          "Edit",
	"events.delete":        "Delete",
	"events.deleteConfirm": "Delete \"%s\"? Its transactions stay put — they're just unmapped from the event.",
	"events.view":          "View transactions",
	"events.viewTitle":     "Show this event's date range in the ledger",
	// %s = count. Shown after auto-association on create.
	"events.tagged":     "Tagged %s transactions in range.",
	"events.taggedNone": "No transactions fell in this event's range yet.",
	// %d = count, %s = formatted total.
	"events.rowMeta":   "%d transactions · %s",
	"events.openEnded": "open-ended",
	"events.chipTitle": "Part of the \"%s\" event",
	"events.saved":     "Event saved.",
}

func init() {
	for k, v := range eventsKeys {
		english[k] = v
	}
}
