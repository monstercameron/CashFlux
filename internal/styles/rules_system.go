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
	// readable form measure, and the strip goes static (the modal's sticky rule
	// would wedge it under the page's own sticky top bar).
	rule(".settings-page",
		prop("max-width", "72rem"),
		prop("margin", "0 auto"),
		prop("padding-bottom", "1.5rem"),
	)
	rule(".settings-page .set-tab-strip",
		prop("position", "static"),
		prop("background", "transparent"),
	)
}
