// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerNotifTriage adds the small polish the 2026-07-19 triage-queue pass
// needs on the Notifications and Smart surfaces. The three-way triage tabs
// themselves reuse the shared .nhx-toggle chrome (registered by
// registerNotifyHistorySurface); this file only adds the Smart findings-feed
// segment spacing and a calm treatment for an empty "Needs you" bucket, all on
// theme tokens so light and dark both read correctly.
func registerNotifTriage() {
	// The Smart feed's Needs-you / Watching segment leads the finer filter row;
	// give it breathing room so the two control rows don't crowd.
	rule("[data-testid=\"smart-triage-view\"]",
		prop("margin-bottom", "0.75rem"),
	)

	// A calm, centered empty state — an empty "Needs you" queue is a win, so it
	// reads as a quiet all-clear rather than a terse error line.
	rule("[data-testid=\"smart-triage-empty\"]",
		prop("padding", "1.25rem 0.5rem"),
		prop("text-align", "center"),
		prop("color", "var(--text-dim)"),
	)
	rule(".notif-list .empty",
		prop("padding", "1.25rem 0.5rem"),
		prop("text-align", "center"),
		prop("color", "var(--text-dim)"),
	)
}
