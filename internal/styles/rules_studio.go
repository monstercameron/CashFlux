// SPDX-License-Identifier: MIT

package styles

// registerStudioSurface emits the Studio formula-tab surface rules: the bento
// host, the FormulaBuilder palette's search toolbar + collapsible group heads,
// and the compound-variable (molecule) editor rows. Token-based throughout so
// everything tracks the active theme.
func registerStudioSurface() {
	rule(".bento.bento-studio",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-studio > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// The Smart surface shares the studio rules file: host grid + auto-height
	// tiles (the section cards render as bare grid children).
	rule(".bento.bento-smart",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-smart > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// Molecule rows: pin content left (the shared .row chrome centers children)
	// and let the formula block + editor stretch the full row width.
	rule(".bento-studio .row",
		prop("align-items", "stretch"),
		prop("text-align", "left"),
	)
	rule(".stu-mol-edit, .stu-mol-formula",
		prop("width", "100%"),
	)
	rule(".stu-mol-edit textarea",
		prop("width", "100%"),
	)

	// The workbench title joins the design system: the serif section-title look
	// with the accent tick (it previously rendered as plain bold sans while its
	// sibling tiles used the editorial chrome).
	rule(".fb-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.15rem"),
		prop("font-weight", "600"),
		prop("border-left", "3px solid var(--accent)"),
		prop("padding-left", "0.6rem"),
	)

	// ── FormulaBuilder palette: search + accordion heads. ───────────────────────
	rule(".fb-pal-toolbar",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("flex-wrap", "wrap"),
		prop("margin-bottom", "0.6rem"),
	)
	rule(".fb-pal-search",
		prop("max-width", "24rem"),
		prop("flex", "1"),
	)
	rule(".fb-pal-examples",
		prop("font-size", "0.72rem"),
		prop("opacity", "0.55"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
		prop("min-width", "0"),
	)
	rule(".fb-pal-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("width", "100%"),
		prop("appearance", "none"),
		prop("background", "transparent"),
		prop("border", "none"),
		prop("border-top", "1px solid var(--border)"),
		prop("padding", "0.5rem 0.15rem"),
		prop("cursor", "pointer"),
		prop("color", "inherit"),
		prop("text-align", "left"),
	)
	rule(".fb-pal-head:hover .fb-pal-title",
		prop("color", "var(--accent)"),
	)
	rule(".fb-pal-caret",
		prop("font-size", "0.7rem"),
		prop("opacity", "0.6"),
		prop("width", "1rem"),
	)
	rule(".fb-pal-count",
		prop("margin-left", "auto"),
		prop("font-size", "0.72rem"),
		prop("padding", "0.05rem 0.5rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
		prop("opacity", "0.7"),
	)

	// ── Compound-variable (molecule) editor rows. ───────────────────────────────
	rule(".stu-mol-formula",
		prop("display", "block"),
		prop("font-family", "ui-monospace, SFMono-Regular, Menlo, monospace"),
		prop("font-size", "0.75rem"),
		prop("line-height", "1.5"),
		prop("color", "var(--accent)"),
		prop("background", "var(--bg-elev)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "6px"),
		prop("padding", "0.4rem 0.6rem"),
		prop("margin-top", "0.35rem"),
		prop("overflow-x", "auto"),
		prop("white-space", "pre-wrap"),
		prop("word-break", "break-word"),
	)
	rule(".stu-mol-edit",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
		prop("margin-top", "0.5rem"),
	)
	rule(".stu-mol-edit textarea",
		prop("font-family", "ui-monospace, SFMono-Regular, Menlo, monospace"),
		prop("font-size", "0.8rem"),
		prop("min-height", "4.5rem"),
	)
	rule(".stu-mol-tag",
		prop("font-size", "0.68rem"),
		prop("padding", "0.05rem 0.5rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
		prop("opacity", "0.75"),
	)
	rule(".stu-mol-tag.is-custom",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
		prop("color", "var(--accent)"),
		prop("opacity", "1"),
	)
}
