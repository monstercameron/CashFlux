// SPDX-License-Identifier: MIT

package styles

// registerSystemSurface emits the shared host for the redesigned system pages
// (/help, /about, /appearance): the auto-row bento grid in the Understand
// language. Hero/section/takeaway chrome reuses the shared rpt-*/debt-* rules.
// Registered from Register().
func registerSystemSurface() {
	rule(".bento.bento-sys",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-sys > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// Prose sections keep a readable measure.
	rule(".bento-sys .sys-prose",
		prop("max-width", "62ch"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
	)
	// The setup checklist's step rows: tick column + label.
	rule(".sys-step",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("padding", "0.3rem 0"),
	)
	rule(".sys-step-mark",
		prop("width", "1.25rem"),
		prop("text-align", "center"),
		prop("flex-shrink", "0"),
	)
	// The tabbed settings panel's strip breathes under the flip header.
	rule(".set-tab-strip",
		prop("position", "sticky"),
		prop("top", "0"),
		prop("z-index", "5"),
		prop("background", "var(--bg-card)"),
		prop("padding-top", "0.25rem"),
	)
	// The routed /settings page hosts the same form in the content column: a
	// readable form measure, and the strip re-sticks BELOW the page's sticky
	// top bar (the modal's top:0 would wedge it underneath) so a long tab —
	// Household, Alerts — can switch tabs from anywhere in the scroll. 3.5rem
	// is the topbar's min-height, the same offset the ledger tables' sticky
	// headers use (--dt-sticky-top).
	rule(".settings-page",
		prop("max-width", "72rem"),
		prop("margin", "0 auto"),
		prop("padding-bottom", "1.5rem"),
	)
	rule(".settings-page .set-tab-strip",
		prop("top", "3.5rem"),
		prop("background", "var(--bg)"),
	)

	// ── Sidebar footer (shell.go HouseholdCard) ──────────────────────────────────
	// A tidy, intentional footer: the household identity, a local-first privacy
	// assurance, and a meta row (About & privacy / version), grouped under a hairline
	// so the foot of the rail reads as one deliberate block rather than loose lines.
	rule(".rail-foot-info",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
		prop("margin-top", "0.85rem"),
		prop("padding-top", "0.8rem"),
		prop("padding-bottom", "0.5rem"),
		prop("border-top", "1px solid var(--border)"),
	)
	// Collapsed rail: hide the whole footer block as a unit (the old per-element
	// hide rules targeted direct span/a children this wrapper now nests).
	rule("aside.rail.collapsed .rail-foot-info",
		prop("display", "none"),
	)
	rule(".rail-foot-hh",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.1rem"),
	)
	rule(".rail-foot-hh-name",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("line-height", "1.2"),
		prop("color", "var(--text-dim)"),
	)
	rule(".rail-foot-hh-sub",
		prop("font-size", "var(--type-11)"),
		prop("line-height", "1.2"),
		prop("color", "var(--text-faint)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	// The privacy assurance: a small lock glyph beside a muted two-line note.
	rule(".rail-foot-privacy",
		prop("display", "flex"),
		prop("align-items", "flex-start"),
		prop("gap", "0.4rem"),
		prop("font-size", "var(--type-11)"),
		prop("line-height", "1.4"),
		prop("color", "var(--text-dim)"),
	)
	rule(".rail-foot-privacy svg",
		prop("color", "var(--text-faint)"),
		prop("margin-top", "0.12rem"),
	)
	// Meta row: About & privacy pinned left, version pinned right, on one baseline.
	rule(".rail-foot-meta",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "space-between"),
		prop("gap", "0.5rem"),
	)
	rule(".rail-foot-about",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-faint)"),
		prop("text-decoration", "none"),
		prop("transition", "color 120ms ease"),
	)
	rule(".rail-foot-about:hover",
		prop("color", "var(--text)"),
		prop("text-decoration", "underline"),
	)
	rule(".rail-foot .app-version",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-faint)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("letter-spacing", "0.02em"),
	)

	// Workspace switcher — styled like the app's standard select control: a bordered
	// box that stacks an uppercase heading ("Workspace") over the selected option
	// (colour dot + name) with a trailing chevron, so both the label and the value read.
	rule(".ws-switch-trigger",
		prop("width", "100%"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
		prop("padding", "0.4rem 0.7rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "9px"),
		prop("background", "var(--bg-elev)"),
		prop("cursor", "pointer"),
		prop("text-align", "left"),
		prop("transition", "border-color 0.14s ease, background 0.14s ease"),
	)
	rule(".ws-switch-trigger:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
	)
	rule(".ws-switch-head",
		prop("font-size", "0.62rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.07em"),
		prop("text-transform", "uppercase"),
		prop("color", "var(--text-faint)"),
	)
	rule(".ws-switch-value",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "space-between"),
		prop("gap", "0.5rem"),
		prop("font-size", "0.9rem"),
		prop("font-weight", "500"),
		prop("color", "var(--text)"),
		prop("min-width", "0"),
	)
}
