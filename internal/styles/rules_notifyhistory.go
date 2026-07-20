// SPDX-License-Identifier: MIT

package styles

// registerNotifyHistorySurface emits the chrome for the Notifications history /
// archive: the Live/History view toggle, the search + severity filter bar, the
// clear action, the archived-row list, and the empty state. All colours use the
// theme tokens (var(--text)/(--border)/(--bg-card)/(--bg-elev)/(--accent)) so it
// tracks both themes; never var(--fg)/(--line)/(--dim)/(--faint) (undefined —
// they render dark in both themes). Every selector is prefixed .nhx- so this file
// stays conflict-free and is registered by the styles coordinator (install.go),
// not edited here.
func registerNotifyHistorySurface() {
	const hair = "1px solid color-mix(in srgb, var(--border) 60%, transparent)"

	// --- Surface shell + view toggle (Live | History) ----------------------------
	rule(".nhx-head",
		display("flex"),
		alignItems("center"),
		justifyContent("flex-start"),
		marginBottom("1.1rem"),
	)
	rule(".nhx-toggle",
		display("inline-flex"),
		alignItems("center"),
		gap("0.15rem"),
		padding("0.2rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-pill)"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
	)
	rule(".nhx-toggle-btn",
		appearance("none"),
		border("0"),
		background("transparent"),
		color("var(--text)"),
		font("inherit"),
		fontSize("var(--type-14)"),
		fontWeight("500"),
		padding("0.35rem 0.95rem"),
		borderRadius("var(--radius-pill)"),
		cursor("pointer"),
		transition("background 0.15s ease, color 0.15s ease"),
	)
	rule(".nhx-toggle-btn:hover",
		background("color-mix(in srgb, var(--bg-elev) 70%, transparent)"),
	)
	// Selected tab: a quiet tinted state (not a solid accent fill, which is reserved
	// for the primary CTA) so the Live/History switch doesn't read as a call to action.
	rule(".nhx-toggle-btn[aria-selected=\"true\"]",
		background("color-mix(in srgb, var(--accent) 14%, transparent)"),
		color("var(--accent)"),
		fontWeight("600"),
	)

	// --- Toolbar: search + severity + clear --------------------------------------
	rule(".nhx-bar",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.6rem"),
		marginBottom("1rem"),
	)
	rule(".nhx-search",
		flex("1 1 220px"),
		minWidth("0"),
		appearance("none"),
		padding("0.5rem 0.85rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("var(--bg-card)"),
		color("var(--text)"),
		font("inherit"),
		fontSize("0.9rem"),
		transition("border-color 0.15s ease"),
	)
	rule(".nhx-search:focus",
		outline("none"),
		borderColor("var(--accent)"),
	)
	rule(".nhx-select",
		appearance("none"),
		padding("0.5rem 0.85rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("var(--bg-card)"),
		color("var(--text)"),
		font("inherit"),
		fontSize("0.9rem"),
		cursor("pointer"),
	)
	rule(".nhx-clear",
		appearance("none"),
		padding("0.5rem 0.9rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("transparent"),
		color("var(--text)"),
		font("inherit"),
		fontSize("var(--type-14)"),
		fontWeight("500"),
		cursor("pointer"),
		transition("border-color 0.15s ease, color 0.15s ease, background 0.15s ease"),
	)
	rule(".nhx-clear:hover",
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--bg-elev) 60%, transparent)"),
	)
	rule(".nhx-count",
		fontSize("var(--type-13)"),
		color("var(--text)"),
		opacity("0.7"),
		marginLeft("auto"),
		fontVariantNumeric("tabular-nums"),
	)

	// --- Row list ----------------------------------------------------------------
	rule(".nhx-list",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		listStyle("none"),
		margin("0"),
		padding("0"),
	)
	rule(".nhx-row",
		display("flex"),
		alignItems("flex-start"),
		gap("0.8rem"),
		padding("0.85rem 1rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-xl)"),
		background("color-mix(in srgb, var(--bg-elev) 40%, transparent)"),
		transition("border-color 0.15s ease, background 0.15s ease"),
	)
	rule(".nhx-row:hover",
		borderColor("color-mix(in srgb, var(--accent) 25%, var(--border))"),
		background("color-mix(in srgb, var(--bg-elev) 65%, transparent)"),
	)
	// Unread rows carry a faint accent rail so read/unread is not colour-only.
	rule(".nhx-row.is-unread",
		boxShadow("inset 3px 0 0 var(--accent)"),
	)
	rule(".nhx-row.is-read",
		opacity("0.72"),
	)

	// Severity medallion (dot + optional icon).
	rule(".nhx-dot",
		flexShrink("0"),
		width("0.7rem"),
		height("0.7rem"),
		marginTop("0.28rem"),
		borderRadius("var(--radius-pill)"),
		background("var(--accent)"),
	)
	rule(".nhx-dot.sev-critical",
		background("var(--danger, #e5484d)"),
	)
	rule(".nhx-dot.sev-warning",
		background("var(--warning, #f5a524)"),
	)
	rule(".nhx-dot.sev-info",
		background("color-mix(in srgb, var(--accent) 80%, var(--text))"),
	)

	rule(".nhx-body",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		minWidth("0"),
		flex("1 1 auto"),
	)
	rule(".nhx-msg",
		color("var(--text)"),
		fontSize("0.92rem"),
		lineHeight("1.35"),
		overflowWrap("anywhere"),
	)
	rule(".nhx-foot",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.4rem 0.6rem"),
	)
	rule(".nhx-sev-tag",
		fontSize("var(--type-11)"),
		fontWeight("600"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text)"),
		opacity("0.65"),
	)
	rule(".nhx-time",
		fontSize("var(--type-12)"),
		color("var(--text)"),
		opacity("0.6"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".nhx-sep",
		color("var(--text)"),
		opacity("0.35"),
	)

	// --- Empty state -------------------------------------------------------------
	rule(".nhx-empty",
		display("flex"),
		flexDirection("column"),
		alignItems("center"),
		gap("0.4rem"),
		textAlign("center"),
		padding("3rem 1.5rem"),
		border(hair),
		borderRadius("14px"),
		background("color-mix(in srgb, var(--bg-elev) 30%, transparent)"),
	)
	rule(".nhx-empty-title",
		color("var(--text)"),
		fontSize("1rem"),
		fontWeight("600"),
	)
	rule(".nhx-empty-hint",
		color("var(--text)"),
		opacity("0.65"),
		fontSize("0.88rem"),
		maxWidth("34ch"),
		lineHeight("1.5"),
	)
}
