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
	// The pinned dashboard tile: one click-through card that reads as a saved view —
	// an eyebrow (filter icon + "Saved view"), the live total as a serif hero figure,
	// the match count, and an accent "Open ›" affordance. Sits inside the standard
	// Widget chrome (which carries the view name as the tile title).
	// The tile fills the resizable widget body (flex:1) and is a size container, so its
	// content responds to whatever dimensions the user drags it to — the figure scales
	// with the tile width and truncates rather than overflowing, and the labels drop
	// out at small sizes (see the @container rules below).
	rule(".saved-view-tile",
		display("flex"),
		flexDirection("column"),
		justifyContent("center"),
		gap("0.5rem"),
		width("100%"),
		flex("1"),
		minWidth("0"),
		minHeight("0"),
		padding("0.15rem"),
		border("none"),
		background("transparent"),
		color("var(--text)"),
		cursor("pointer"),
		textAlign("left"),
		overflow("hidden"),
		prop("container-type", "inline-size"),
	)
	rule(".saved-view-tile-eyebrow",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		minWidth("0"),
		fontSize("0.64rem"),
		fontWeight("700"),
		letterSpacing("0.08em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".saved-view-tile-eyebrow span",
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".saved-view-tile-ico",
		width("0.85rem"),
		height("0.85rem"),
		color("var(--accent)"),
		flexShrink("0"),
	)
	rule(".saved-view-tile-body",
		display("flex"),
		alignItems("flex-end"),
		justifyContent("space-between"),
		gap("0.6rem"),
		minWidth("0"),
	)
	rule(".saved-view-tile-figs",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
		minWidth("0"),
	)
	rule(".saved-view-tile-total",
		prop("font-family", "var(--font-display), Fraunces, Georgia, serif"),
		// Scale with the tile width so a narrow (1-col) tile shrinks the figure instead
		// of clipping it; clamp keeps it readable and never larger than the 2-col hero.
		fontSize("clamp(1.05rem, 8cqw, 1.95rem)"),
		fontWeight("600"),
		letterSpacing("-0.01em"),
		lineHeight("1.05"),
		prop("font-variant-numeric", "tabular-nums"),
		color("var(--text)"),
		maxWidth("100%"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".saved-view-tile-sub",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
		maxWidth("100%"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".saved-view-tile-go",
		display("inline-flex"),
		alignItems("center"),
		gap("0.15rem"),
		flexShrink("0"),
		fontSize("0.78rem"),
		fontWeight("600"),
		color("var(--accent)"),
	)
	rule(".saved-view-tile-go-ico",
		width("0.95rem"),
		height("0.95rem"),
	)
	// Hover: nudge the affordance so the whole card reads as tappable.
	rule(".saved-view-tile:hover .saved-view-tile-go",
		textDecoration("underline"),
	)
	// Responsive degradation, keyed off the tile's own width (it is an inline-size
	// container): on a narrow tile drop the eyebrow's "Saved view" text (keep the
	// icon) and the Open label (keep the chevron), so the figure keeps room.
	rawBlock("@container (max-width:210px){.saved-view-tile-eyebrow span{display:none}.saved-view-tile-go span{display:none}}")
}
