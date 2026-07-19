// SPDX-License-Identifier: MIT

package i18n

// notifAsstKeys holds English strings added by the 2026-07-19 Notifications +
// Assistant UX refinement pass (notification primary-action model + "Due today"
// temporal copy; Assistant/Smart findings-first hierarchy). Merged via init so
// the shared en.go is never touched by this concurrent lane.
var notifAsstKeys = Catalog{
	// A bill-due alert whose due date is the current day (0 days remaining, not yet
	// past). Replaces the awkward "Due in 0 days"; the "Overdue by N days" wording
	// still covers past-due alerts.
	"notifications.dueToday": "Due today.",
}

func init() {
	for k, v := range notifAsstKeys {
		english[k] = v
	}
}
