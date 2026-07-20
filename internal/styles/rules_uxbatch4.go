// SPDX-License-Identifier: MIT

package styles

// registerUxbatch4 holds the CSS for UX review batch #4 (subscriptions/bills
// kebab overflow, notifications destructive parity, settings alert thresholds).
// It must be registered AFTER registerGenerated() so its rules win the cascade
// over the base .toggle-row declarations they refine.
func registerUxbatch4() {
	// Settings → Alerts (review #29): the per-rule threshold input used a bare
	// `.toggle-row`, which is `justify-content: space-between` — so the number input
	// floated to the far right edge, a screen-width from its "$"/"days" label. Give
	// the threshold row its own compact, left-aligned layout indented under the alert
	// toggle it belongs to, matching the "Freshness reminders" (.rate-row) rows. The
	// compound `.toggle-row.alert-thresh-row` selector outranks the base `.toggle-row`
	// regardless of registration order; the shared `.toggle-row .rate-in` 90px width
	// rule still applies because the row keeps its `toggle-row` class.
	rule(".toggle-row.alert-thresh-row",
		prop("justify-content", "flex-start"),
		prop("gap", "0.6rem"),
		prop("padding", "0.1rem 0 0.55rem 1.5rem"),
		prop("border-bottom", "none"),
		prop("font-size", "var(--type-14)"),
	)
}
