// SPDX-License-Identifier: MIT

package styles

// registerNetWorthSurface emits the from-scratch "balance sheet" vocabulary for
// /networth. The prefix is `nws-` and, like the Bills & Recurring rhythm
// surface, it is deliberately NOT the bento tile kit: a stack of full-width
// sections carrying two signature graphics — THE BRIDGE (a waterfall that
// decomposes the window's movement, residual included) and TWO SIDES (a
// mirrored area chart with assets stacked up, liabilities down, and the net
// worth line through the middle).
//
// Garnishes are the app's shared ones, matched deliberately to /budgets and
// /reports: the 12px `--radius-xl` page-section step (never `--radius`, which is
// the theme-editor knob and ships at 0px), the same two-layer per-theme
// elevation `.rhy-section` and the bento `.w` widget carry, the `.card-title`
// section-title treatment, and the `--radius-lg` step for nested boxes.
//
// Every colour is a theme token. The two graphics are built from --accent
// (assets, gains) and --text at reduced alpha (liabilities, losses) rather than
// a red/green pair, because DEBT IS STRUCTURE, NOT AN EMERGENCY — red is left
// to --danger and spent only on a genuinely negative net worth or an alarm-band
// ratio. Registered from Register().
func registerNetWorthSurface() {
	registerNwsShell()
	registerNwsHero()
	registerNwsBridge()
	registerNwsSides()
	registerNwsDetail()
	registerNwsBreakpoints()

	// Liability share bars read in the money-negative tone, not the accent.
	// Still used by the household and member split bars.
	rule(".share-bar-fill.nw-bar-down",
		prop("background", "var(--down, #d8716f)"),
	)
}

// registerNwsShell emits the page stack, the section chrome, and the view toggle.
func registerNwsShell() {
	rule(".nws",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1.15rem"),
		prop("width", "100%"),
	)
	// A deferred section's position holder: display:contents costs no box and no
	// gap while empty, and lets the arriving section become the flex item
	// directly — so the stack looks identical before and after it lands.
	rule(".nws-slot",
		prop("display", "contents"),
	)
	// The page's largest wrapper wears the app's shared page-section garnish:
	// the fixed 12px --radius-xl step plus the same two-layer elevation the bento
	// `.w` widget and `.rhy-section` carry, per theme.
	rule(".nws-section",
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-xl)"),
		prop("padding", "1.1rem 1.15rem"),
		prop("box-shadow", "0 1px 1px rgba(0,0,0,0.20), 0 10px 26px -18px rgba(0,0,0,0.55),"+
			" inset 0 1px 0 rgba(255,255,255,0.035)"),
	)
	rule("[data-theme=\"light\"] .nws-section",
		prop("box-shadow", "0 1px 2px rgba(17,24,39,0.05), 0 12px 28px -20px rgba(17,24,39,0.16),"+
			" inset 0 1px 0 rgba(255,255,255,0.7)"),
	)
	// The hero draws its own field, so it drops the card seam.
	rule(".nws-section.nws-flush",
		prop("padding", "0"),
		prop("border", "0"),
		prop("background", "transparent"),
		prop("box-shadow", "none"),
	)
	rule("[data-theme=\"light\"] .nws-section.nws-flush",
		prop("box-shadow", "none"),
	)
	rule(".nws-sec-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
		prop("flex-wrap", "wrap"),
		prop("margin-bottom", "0.6rem"),
	)
	// Verbatim the app's shared section-title garnish (the `.card-title` /
	// `.rhy-sec-title` treatment): sans, 16px, 600, -0.01em, 1.35 leading.
	rule(".nws-sec-title",
		prop("font-size", "var(--type-16)"),
		prop("font-weight", "600"),
		prop("line-height", "1.35"),
		prop("letter-spacing", "-0.01em"),
		prop("margin", "0"),
	)
	rule(".nws-sec-note",
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
		prop("margin", "0 0 0.7rem"),
	)

	// ── Glance | Detail toggle: the same segmented affordance the Reports
	// Summary | Full report pair uses, at the page's own scale.
	rule(".nws-views",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("flex-wrap", "wrap"),
	)
	rule(".nws-viewset",
		prop("display", "inline-flex"),
		prop("gap", "0.25rem"),
		prop("padding", "0.2rem"),
		prop("background", "var(--bg)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
	)
	rule(".nws-view",
		prop("appearance", "none"),
		prop("background", "transparent"),
		prop("border", "0"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("padding", "0.35rem 0.9rem"),
		prop("border-radius", "999px"),
		prop("cursor", "pointer"),
		prop("transition", "background var(--motion-fast, 120ms) var(--ease-standard, ease),"+
			" color var(--motion-fast, 120ms) var(--ease-standard, ease)"),
	)
	rule(".nws-view:hover",
		prop("color", "var(--text)"),
	)
	rule(".nws-view.is-on",
		prop("background", "var(--bg-card)"),
		prop("color", "var(--text)"),
		prop("box-shadow", "0 1px 2px rgba(0,0,0,0.18)"),
	)
	rule(".nws-window",
		prop("margin-left", "auto"),
	)
}

// registerNwsHero emits the calm headline: the net figure in the display serif,
// its window delta, and the two side totals.
func registerNwsHero() {
	rule(".nws-hero",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) auto"),
		prop("gap", "1.25rem"),
		prop("align-items", "end"),
	)
	rule(".nws-hero-eyebrow",
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
		prop("margin", "0 0 0.25rem"),
	)
	rule(".nws-hero-value",
		prop("font-size", "clamp(2.4rem, 5.2vw, 3.6rem)"),
		prop("font-weight", "600"),
		prop("line-height", "1.05"),
		prop("letter-spacing", "-0.02em"),
		prop("margin", "0"),
	)
	// Only a genuinely negative net worth earns the alarm colour.
	rule(".nws-hero-value.is-negative",
		prop("color", "var(--danger)"),
	)
	rule(".nws-hero-delta",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
		prop("margin-top", "0.45rem"),
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("color", "var(--text-dim)"),
	)
	rule(".nws-hero-sides",
		prop("display", "flex"),
		prop("gap", "1.5rem"),
		prop("align-items", "flex-end"),
	)
	rule(".nws-side",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
		prop("min-width", "0"),
	)
	rule(".nws-side-label",
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
	)
	rule(".nws-side-value",
		prop("font-size", "var(--type-20, 1.25rem)"),
		prop("font-weight", "600"),
		prop("letter-spacing", "-0.01em"),
	)
	// The asset side wears the accent; the liability side stays neutral. Painting
	// it red would say "emergency" about an ordinary mortgage.
	rule(".nws-side.is-assets .nws-side-value",
		prop("color", "var(--accent)"),
	)
	rule(".nws-side.is-liabilities .nws-side-value",
		prop("color", "var(--text)"),
		prop("opacity", "0.8"),
	)

	// The takeaway: one plain-English read of the window, in the app's takeaway
	// idiom (serif, generous leading, accent left edge).
	rule(".nws-takeaway",
		prop("margin", "0"),
		prop("padding", "0.7rem 0.9rem"),
		prop("border-left", "3px solid var(--accent)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg)"),
		prop("font-size", "1.05rem"),
		prop("line-height", "1.5"),
	)

	// ── Ratios, each with its interpretation. Never a bare percentage.
	rule(".nws-ratios",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(auto-fit, minmax(15rem, 1fr))"),
		prop("gap", "0.75rem"),
	)
	rule(".nws-ratio",
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg)"),
		prop("padding", "0.75rem 0.85rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.2rem"),
		prop("min-width", "0"),
	)
	rule(".nws-ratio-label",
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
	)
	rule(".nws-ratio-value",
		prop("font-size", "1.5rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "-0.01em"),
		prop("line-height", "1.2"),
	)
	rule(".nws-ratio-read",
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.45"),
	)
	// Bands. Only `alarm` is allowed the danger colour.
	rule(".nws-ratio.is-strong .nws-ratio-value",
		prop("color", "var(--accent)"),
	)
	rule(".nws-ratio.is-watch .nws-ratio-value",
		// The app's shared warn hue (tw.cWarn), reached the same way the money
		// tones are: a token with the shared hex as its fallback.
		prop("color", "var(--warn, #cfa14e)"),
	)
	rule(".nws-ratio.is-alarm .nws-ratio-value",
		prop("color", "var(--danger)"),
	)
	rule(".nws-ratio.is-alarm",
		prop("border-color", "var(--danger)"),
	)
}

// registerNwsBridge emits signature graphic #1: the waterfall.
func registerNwsBridge() {
	rule(".nws-bridge",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
		prop("min-width", "0"),
	)
	// The bars carry no text, so the viewBox may be stretched freely: the X axis
	// is measured in COLUMNS (column i spans x = i…i+1), which is what lets the
	// HTML label grid below line up with the bars exactly, at any width, with no
	// measurement.
	rule(".nws-bridge-svg",
		prop("display", "block"),
		prop("width", "100%"),
		prop("height", "210px"),
		prop("overflow", "visible"),
	)
	rule(".nws-bar-up",
		prop("fill", "var(--accent)"),
	)
	rule(".nws-bar-down",
		prop("fill", "var(--text)"),
		prop("opacity", "0.42"),
	)
	// Start and end are STATES, not movements — hollow so the eye reads the
	// coloured legs as the story between them.
	rule(".nws-bar-anchor",
		prop("fill", "var(--text)"),
		prop("opacity", "0.14"),
	)
	// The residual is deliberately drawn in a different language (outlined, not
	// filled): it is the part the named legs could not explain, and it should
	// never be mistaken for one of them.
	rule(".nws-bar-residual",
		prop("fill", "none"),
		prop("stroke", "var(--text-dim)"),
		prop("stroke-width", "1.5"),
		prop("stroke-dasharray", "4 3"),
	)
	rule(".nws-bridge-connect",
		prop("stroke", "var(--border)"),
		prop("stroke-width", "1"),
		prop("stroke-dasharray", "3 3"),
	)
	rule(".nws-bridge-base",
		prop("stroke", "var(--border)"),
		prop("stroke-width", "1"),
	)
	// The label grid: one column per bar, same order, same count.
	rule(".nws-bridge-labels",
		prop("display", "grid"),
		prop("gap", "0"),
	)
	rule(".nws-bridge-label",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "center"),
		prop("gap", "0.1rem"),
		prop("text-align", "center"),
		prop("padding", "0 0.15rem"),
		prop("min-width", "0"),
	)
	rule(".nws-bridge-name",
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("line-height", "1.25"),
	)
	rule(".nws-bridge-amount",
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("letter-spacing", "-0.01em"),
		prop("white-space", "nowrap"),
	)
	rule(".nws-bridge-amount.is-up",
		prop("color", "var(--accent)"),
	)
	rule(".nws-bridge-amount.is-anchor",
		prop("color", "var(--text)"),
	)
	rule(".nws-bridge-amount.is-residual",
		prop("color", "var(--text-dim)"),
	)
	// The stacked fallback for narrow panes: same legs, same figures, read
	// downward instead of across. Hidden until the breakpoint swaps them.
	rule(".nws-bridge-stack",
		prop("display", "none"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
	)
	rule(".nws-bridge-srow",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) auto"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("padding", "0.45rem 0.6rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg)"),
	)
	rule(".nws-bridge-srow.is-anchor",
		prop("background", "transparent"),
		prop("font-weight", "600"),
	)
	rule(".nws-bridge-sbar",
		prop("height", "0.35rem"),
		prop("border-radius", "999px"),
		prop("margin-top", "0.3rem"),
	)
	rule(".nws-bridge-sbar.is-up",
		prop("background", "var(--accent)"),
	)
	rule(".nws-bridge-sbar.is-down",
		prop("background", "var(--text)"),
		prop("opacity", "0.42"),
	)
	rule(".nws-bridge-sbar.is-residual",
		prop("background", "var(--text-dim)"),
		prop("opacity", "0.5"),
	)
}

// registerNwsSides emits signature graphic #2: the mirrored composition chart.
func registerNwsSides() {
	rule(".nws-sides",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
		prop("min-width", "0"),
	)
	rule(".nws-sides-svg",
		prop("display", "block"),
		prop("width", "100%"),
		prop("height", "280px"),
		prop("overflow", "visible"),
	)
	// Asset bands: one hue (the theme accent) stepped by alpha, most liquid the
	// most solid — so composition reads as ONE side, not four unrelated series.
	rule(".nws-band-a0", prop("fill", "var(--accent)"), prop("opacity", "0.92"))
	rule(".nws-band-a1", prop("fill", "var(--accent)"), prop("opacity", "0.66"))
	rule(".nws-band-a2", prop("fill", "var(--accent)"), prop("opacity", "0.42"))
	rule(".nws-band-a3", prop("fill", "var(--accent)"), prop("opacity", "0.24"))
	// Liability bands: neutral, stepped the same way. Structural, not alarming.
	rule(".nws-band-l0", prop("fill", "var(--text)"), prop("opacity", "0.42"))
	rule(".nws-band-l1", prop("fill", "var(--text)"), prop("opacity", "0.28"))
	rule(".nws-band-l2", prop("fill", "var(--text)"), prop("opacity", "0.16"))
	rule(".nws-sides-net",
		prop("fill", "none"),
		prop("stroke", "var(--text)"),
		prop("stroke-width", "2"),
		prop("stroke-linejoin", "round"),
		prop("stroke-linecap", "round"),
	)
	rule(".nws-sides-zero",
		prop("stroke", "var(--border)"),
		prop("stroke-width", "1"),
	)
	rule(".nws-sides-legend",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.35rem 1rem"),
		prop("font-size", "var(--type-13)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".nws-legend-item",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
	)
	rule(".nws-legend-dot",
		prop("width", "0.6rem"),
		prop("height", "0.6rem"),
		prop("border-radius", "3px"),
		prop("flex", "0 0 auto"),
	)
	// Legend swatches mirror the band fills exactly, one modifier per stacking
	// position — same hue, same alpha step, so the key cannot drift from the
	// chart it explains.
	rule(".nws-legend-dot.is-a0", prop("background", "var(--accent)"), prop("opacity", "0.92"))
	rule(".nws-legend-dot.is-a1", prop("background", "var(--accent)"), prop("opacity", "0.66"))
	rule(".nws-legend-dot.is-a2", prop("background", "var(--accent)"), prop("opacity", "0.42"))
	rule(".nws-legend-dot.is-a3", prop("background", "var(--accent)"), prop("opacity", "0.24"))
	rule(".nws-legend-dot.is-l0", prop("background", "var(--text)"), prop("opacity", "0.42"))
	rule(".nws-legend-dot.is-l1", prop("background", "var(--text)"), prop("opacity", "0.28"))
	rule(".nws-legend-dot.is-l2", prop("background", "var(--text)"), prop("opacity", "0.16"))
	rule(".nws-sides-axis",
		prop("display", "flex"),
		prop("justify-content", "space-between"),
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("color", "var(--text-dim)"),
	)
}

// registerNwsDetail emits the Detail view: the numbered section chips and the
// balance-sheet tables.
func registerNwsDetail() {
	rule(".nws-index",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.35rem"),
		prop("position", "sticky"),
		prop("top", "0.25rem"),
		prop("z-index", "3"),
		prop("padding", "0.35rem"),
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
	)
	rule(".nws-idx",
		prop("appearance", "none"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("background", "transparent"),
		prop("border", "0"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
		prop("padding", "0.3rem 0.75rem"),
		prop("border-radius", "999px"),
		prop("cursor", "pointer"),
	)
	rule(".nws-idx:hover",
		prop("color", "var(--text)"),
		prop("background", "var(--bg)"),
	)
	rule(".nws-idx.is-current",
		prop("color", "var(--text)"),
		prop("background", "var(--bg)"),
	)
	rule(".nws-idx-num",
		prop("font-weight", "600"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("opacity", "0.7"),
	)
	// A numbered document section: the number sits beside the title the way the
	// Reports full-report sections do.
	rule(".nws-dsec-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.6rem"),
		prop("margin-bottom", "0.15rem"),
	)
	rule(".nws-dsec-num",
		prop("font-size", "1.15rem"),
		prop("font-weight", "600"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("letter-spacing", "-0.01em"),
	)

	// Balance-sheet tables: everyday rows, one nested step down from the section.
	rule(".nws-table",
		prop("width", "100%"),
		prop("border-collapse", "collapse"),
		prop("font-size", "var(--type-13)"),
	)
	rule(".nws-table th",
		prop("text-align", "left"),
		prop("font-weight", "600"),
		prop("color", "var(--text-dim)"),
		prop("padding", "0.4rem 0.5rem"),
		prop("border-bottom", "1px solid var(--border)"),
		prop("white-space", "nowrap"),
	)
	rule(".nws-table td",
		prop("padding", "0.45rem 0.5rem"),
		prop("border-bottom", "1px solid var(--border)"),
		prop("vertical-align", "middle"),
	)
	rule(".nws-table tr:last-child td",
		prop("border-bottom", "0"),
	)
	rule(".nws-num",
		prop("text-align", "right"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("white-space", "nowrap"),
	)
	rule(".nws-total td",
		prop("font-weight", "600"),
		prop("border-top", "1px solid var(--border)"),
	)
	// A wide table scrolls inside its own box; the page never scrolls sideways.
	rule(".nws-scroll",
		prop("overflow-x", "auto"),
		prop("max-width", "100%"),
	)
	// The share bar inside a composition row, scaled WITHIN its own side so a
	// $304k condo cannot flatten every other bar to a stub.
	rule(".nws-share",
		prop("height", "0.4rem"),
		prop("border-radius", "999px"),
		prop("background", "var(--border)"),
		prop("overflow", "hidden"),
		prop("min-width", "3rem"),
	)
	rule(".nws-share-fill",
		prop("height", "100%"),
		prop("border-radius", "999px"),
		prop("background", "var(--accent)"),
	)
	rule(".nws-share-fill.is-liability",
		prop("background", "var(--text)"),
		prop("opacity", "0.42"),
	)
}

// registerNwsBreakpoints degrades both signature graphics by CHANGING FORM
// rather than by clipping: below the single-column content width the waterfall
// becomes a stacked list of the same legs with the same figures, and the
// mirrored chart loses height rather than legibility.
func registerNwsBreakpoints() {
	ruleContentMax(contentGrid4, ".nws-hero",
		prop("grid-template-columns", "1fr"),
		prop("align-items", "start"),
	)
	ruleContentMax(contentGrid4, ".nws-hero-sides",
		prop("padding-top", "0.9rem"),
		prop("border-top", "1px solid var(--border)"),
		prop("width", "100%"),
	)
	// The waterfall needs roughly 90px per column to label itself honestly. Below
	// the single-column threshold it swaps to the stacked list instead of
	// squeezing eight labels into a phone-width pane.
	ruleContentMax(contentGrid1, ".nws-bridge-svg",
		prop("display", "none"),
	)
	ruleContentMax(contentGrid1, ".nws-bridge-labels",
		prop("display", "none"),
	)
	ruleContentMax(contentGrid1, ".nws-bridge-stack",
		prop("display", "flex"),
	)
	ruleContentMax(contentGrid1, ".nws-sides-svg",
		prop("height", "210px"),
	)
	ruleContentMax(contentGrid1, ".nws-window",
		prop("margin-left", "0"),
	)
}
