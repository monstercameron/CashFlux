// SPDX-License-Identifier: MIT

package styles

// registerSavedViews styles the transactions toolbar "Views" popover (TX3): the
// saved-view rows (name + live count/total + actions), the inline save/alert
// forms, the crossed-threshold notice, and the pinned dashboard tile. All colors
// come from theme tokens so the surface tracks light/dark.
func registerSavedViews() {
	rule(".saved-views-menu",
		minWidth("18rem"),
		maxWidth("22rem"),
		padding("0.5rem"),
	)
	rule(".saved-views-head",
		padding("0.25rem 0.5rem"),
	)
	rule(".saved-views-save",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		width("100%"),
	)
	rule(".saved-views-save.is-disabled",
		opacity("0.5"),
		cursor("not-allowed"),
	)
	rule(".saved-views-empty",
		padding("0.5rem"),
		fontSize("0.85rem"),
	)
	rule(".saved-views-list",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		marginTop("0.25rem"),
	)
	rule(".saved-view-row",
		borderTop("1px solid var(--border)"),
		padding("0.35rem 0.25rem"),
	)
	rule(".saved-view-main",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
	)
	rule(".saved-view-apply",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		flex("1 1 auto"),
		gap("0.1rem"),
		padding("0.3rem 0.4rem"),
		border("none"),
		background("transparent"),
		color("var(--text)"),
		cursor("pointer"),
		borderRadius("6px"),
		textAlign("left"),
	)
	rule(".saved-view-apply:hover",
		background("var(--bg-elev)"),
	)
	rule(".saved-view-name",
		fontWeight("600"),
		fontSize("0.9rem"),
	)
	rule(".saved-view-actions",
		display("flex"),
		gap("0.15rem"),
		flexShrink("0"),
	)
	rule(".saved-view-alert",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		marginTop("0.35rem"),
		padding("0.4rem 0.5rem"),
		borderRadius("6px"),
		background("var(--accent-dim)"),
		color("var(--text)"),
		fontSize("0.82rem"),
	)
	rule(".saved-view-alert-form",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		marginTop("0.35rem"),
		padding("0.4rem"),
		borderRadius("6px"),
		background("var(--bg-elev)"),
	)
	rule(".saved-views-form",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		padding("0.5rem"),
	)
	rule(".saved-views-form-actions",
		display("flex"),
		justifyContent("flex-end"),
		gap("0.4rem"),
	)
	// The pinned dashboard tile: a full-bleed click-through card showing the view's
	// live total over its match-count sub-label.
	rule(".saved-view-tile",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		gap("0.25rem"),
		width("100%"),
		padding("0.5rem 0.25rem"),
		border("none"),
		background("transparent"),
		color("var(--text)"),
		cursor("pointer"),
		textAlign("left"),
	)
	rule(".saved-view-tile-total",
		fontSize("1.5rem"),
		fontWeight("700"),
	)
}
