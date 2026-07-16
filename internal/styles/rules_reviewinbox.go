// SPDX-License-Identifier: MIT

package styles

// registerReviewInboxSurface emits the transaction Review inbox (CG-S2): a
// focused triage card with a progress bar, the transaction under review, a
// category picker + one-click suggestion, and the step actions. Theme tokens
// only, so it tracks light/dark.
func registerReviewInboxSurface() {
	rule(".rvw",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
		padding("1.25rem"),
	)

	// Progress: a count line over a slim accent-filled track.
	rule(".rvw-progress",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".rvw-progress-count",
		fontSize("0.75rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
	)
	rule(".rvw-progress-track",
		height("6px"),
		borderRadius("999px"),
		background("var(--bg-elev)"),
		overflow("hidden"),
	)
	rule(".rvw-progress-fill",
		height("100%"),
		minWidth("4px"), // never render as an invisible sliver at "1 of N"
		background("var(--accent)"),
		borderRadius("999px"),
		transition("width .2s ease"),
	)

	// The transaction under review.
	rule(".rvw-card",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		padding("1rem"),
		border("1px solid var(--border)"),
		borderRadius("12px"),
		background("var(--bg-card)"),
	)
	rule(".rvw-card-top",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".rvw-reason",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		padding("0.1rem 0.5rem"),
		borderRadius("999px"),
		whiteSpace("nowrap"),
	)
	rule(".rvw-reason.is-uncat",
		color("#d98c00"),
		background("color-mix(in srgb, #d98c00 15%, transparent)"),
	)
	rule(".rvw-reason.is-flagged",
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 15%, transparent)"),
	)
	rule(".rvw-date",
		fontSize("0.75rem"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".rvw-payee",
		fontSize("1.15rem"),
		fontWeight("700"),
		color("var(--text)"),
		overflowWrap("anywhere"),
	)
	// The raw bank descriptor, shown small + muted under the cleaned name so the
	// user still sees the literal string without it dominating the card.
	rule(".rvw-rawpayee",
		fontSize("0.72rem"),
		color("var(--text-dim)"),
		fontFamily("var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)"),
		overflowWrap("anywhere"),
	)
	rule(".rvw-meta",
		display("flex"),
		alignItems("baseline"),
		gap("0.75rem"),
		flexWrap("wrap"),
	)
	rule(".rvw-amount",
		fontSize("1.1rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	rule(".rvw-amount.is-income", color("var(--accent)"))
	rule(".rvw-acct",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
	)

	// Category picker + one-click suggestion chip.
	rule(".rvw-assign",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
	)
	rule(".rvw-assign-label",
		fontSize("0.7rem"),
		fontWeight("600"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)
	rule(".rvw-suggest",
		display("inline-flex"),
		alignItems("center"),
		alignSelf("flex-start"),
		gap("0.4rem"),
		padding("0.35rem 0.7rem"),
		borderRadius("999px"),
		border("1px solid color-mix(in srgb, var(--accent) 40%, transparent)"),
		background("color-mix(in srgb, var(--accent) 10%, transparent)"),
		color("var(--accent)"),
		fontSize("0.82rem"),
		fontWeight("600"),
		cursor("pointer"),
		transition("background .12s ease"),
	)
	rule(".rvw-suggest:hover",
		background("color-mix(in srgb, var(--accent) 18%, transparent)"),
	)

	// The SMART (deterministic) suggestion and the SMART+ (AI) button sit on one
	// wrapping row so they read as sibling quick-picks under the category select.
	rule(".rvw-sugg-row",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		gap("0.5rem"),
	)
	// SMART+ AI button — a distinct accent-outlined pill with the branded AI glyph.
	rule(".rvw-ai",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		padding("0.35rem 0.7rem"),
		borderRadius("999px"),
		border("1px solid color-mix(in srgb, var(--accent) 45%, transparent)"),
		background("transparent"),
		color("var(--accent)"),
		fontSize("0.82rem"),
		fontWeight("600"),
		cursor("pointer"),
		transition("background .12s ease"),
	)
	rule(".rvw-ai:hover",
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".rvw-ai[aria-disabled=\"true\"]",
		opacity("0.6"),
		pointerEvents("none"),
	)
	rule(".rvw-ai-err",
		fontSize("0.78rem"),
		color("var(--danger)"),
	)

	// "Also apply to N others from this merchant" — a quiet opt-in under the picker.
	rule(".rvw-similar",
		display("flex"),
		alignItems("center"),
		gap("0.45rem"),
		fontSize("0.82rem"),
		color("var(--text-dim)"),
		cursor("pointer"),
	)
	rule(".rvw-similar input",
		width("1rem"),
		height("1rem"),
		accentColor("var(--accent)"),
		cursor("pointer"),
	)

	// Step actions: a dominant primary confirm, then a quiet secondary skip.
	rule(".rvw-actions",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		marginTop("0.25rem"),
	)
	rule(".rvw-commit",
		flex("1 1 auto"),
	)
	// Disarmed until a category is chosen — dimmed and click-inert.
	rule(".rvw-commit.is-disabled",
		opacity("0.5"),
		pointerEvents("none"),
	)

	// All-caught-up state.
	rule(".rvw-done",
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		textAlign("center"),
		gap("0.6rem"),
		padding("2.5rem 1rem"),
	)
	rule(".rvw-done-icon",
		color("var(--accent)"),
	)
	rule(".rvw-done-title",
		fontSize("1.15rem"),
		fontWeight("700"),
		color("var(--text)"),
	)
	rule(".rvw-done-sub",
		fontSize("0.85rem"),
		color("var(--text-dim)"),
		maxWidth("32ch"),
	)
}
