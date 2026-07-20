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
		prop("gap", "0.7rem"),
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
		prop("padding", "0.75rem 1.1rem"),
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
		prop("margin-bottom", "0.35rem"),
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
		prop("margin", "0 0 0.5rem"),
	)

	// ── The Glance grid: the interpretation sits BESIDE the evidence, so a
	// reader meets "what it means" at the same moment as the chart it explains
	// rather than a scroll later. Sections opt into a full-width row themselves.
	rule(".nws-glance",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1.62fr) minmax(19rem, 1fr)"),
		prop("gap", "0.85rem"),
		// Stretch, NOT start. With start-aligned items the shorter column ended
		// where its content ended and left a ~215px hole above the next
		// full-width row — the page read as though a block had failed to load.
		// Stretching makes the row one deliberate band, and the extra height
		// goes to the waterfall, which is the page's strongest element and only
		// gets better with room.
		prop("align-items", "stretch"),
	)
	// A stretched section lays its body out as a column so the graphic inside
	// can take the slack, rather than the section growing around a fixed-height
	// chart and reintroducing the hole one level down.
	rule(".nws-glance > .nws-section",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".nws-glance > .nws-section > .nws-bridge",
		prop("flex", "1 1 auto"),
	)
	// The bars grow into whatever the row gives them; the labels beneath keep
	// their own size, because they are the graphic's authority and a stretched
	// label is not more readable.
	rule(".nws-glance .nws-bridge-svg",
		prop("flex", "1 1 auto"),
		prop("min-height", "170px"),
	)
	rule(".nws-glance > .nws-wide",
		prop("grid-column", "1 / -1"),
	)
	rule(".nws-read .nws-ratios",
		prop("grid-template-columns", "1fr"),
		prop("gap", "0.5rem"),
	)
	rule(".nws-read .nws-ratio",
		prop("padding", "0.6rem 0.75rem"),
	)
	rule(".nws-read .nws-ratio-value",
		prop("font-size", "1.25rem"),
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
	// The data-quality trigger reads as part of the as-of line, not as a badge
	// competing with it — quiet by default, and only tinted when something
	// actually needs the reader's attention.
	rule(".nws-dq",
		prop("display", "inline-flex"),
		prop("position", "relative"),
	)
	rule(".nws-dq-btn",
		prop("appearance", "none"),
		prop("background", "transparent"),
		prop("border", "0"),
		prop("border-bottom", "1px dotted var(--border)"),
		prop("padding", "0"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "inherit"),
		prop("font-family", "inherit"),
		prop("cursor", "pointer"),
	)
	rule(".nws-dq-btn:hover",
		prop("color", "var(--text)"),
		prop("border-bottom-color", "var(--text-dim)"),
	)
	rule(".nws-dq-btn.is-attention",
		prop("color", "var(--warn, #cfa14e)"),
		prop("border-bottom-color", "var(--warn, #cfa14e)"),
	)
	rule(".nws-hero-value",
		prop("font-size", "clamp(2.1rem, 4.4vw, 3rem)"),
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
		prop("margin-top", "0.3rem"),
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
		// 190px is the floor that keeps a small leg visible as a bar without
		// crowding the label grid beneath it; the labels are the graphic's
		// authority and are never compressed to buy height.
		prop("height", "170px"),
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
		// Two lines' worth of space whether the name needs it or not, so a
		// wrapping label ("Debt paid down") does not push its own figure out of
		// line with the figures either side of it.
		prop("min-height", "2.5em"),
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

// registerNwsSides emits signature graphic #2: the gap between the two
// boundaries, plus the composition strips that carry "what shape" now that the
// chart carries only "how it moved".
func registerNwsSides() {
	// Chart and strips are two panels of one section: "how it moved" beside
	// "what shape it is". Side by side they cost one panel's height instead of
	// two, which is what keeps the whole Glance view inside a single screen.
	// The plot gets the WHOLE width and the strips sit beneath it, side by side.
	// They used to share a row, which left the chart about 40% of its section —
	// roughly 420px at a 1440px viewport. Everything measured along time was
	// paying for that: eight date labels had 50px each, and the pace rail's
	// rungs collided outright. A graphic whose subject is a five-year shape
	// needs the width; two 100%-bars do not.
	rule(".nws-sides",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1.1rem"),
		prop("min-width", "0"),
	)
	// The plot is a positioned frame: the SVG draws the shapes, and every WORD on
	// the graphic is real HTML laid over it at an exact computed percentage. That
	// keeps type crisp under the stretched viewBox, keeps it themeable and
	// selectable, and puts the axis and region names in the accessibility tree —
	// the same reasoning THE BRIDGE's label grid follows.
	rule(".nws-plot",
		prop("position", "relative"),
		prop("padding-left", "3.4rem"),
	)
	rule(".nws-yaxis",
		prop("position", "absolute"),
		prop("inset", "0 auto 0 0"),
		prop("width", "3.1rem"),
	)
	rule(".nws-ytick",
		prop("position", "absolute"),
		prop("right", "0"),
		prop("transform", "translateY(-50%)"),
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("white-space", "nowrap"),
	)
	// The floor tick is the scale's starting point rather than a gridline, so it
	// is toned back — present and readable, but not competing with the round
	// values above it.
	rule(".nws-ytick.is-floor",
		prop("opacity", "0.7"),
		prop("font-style", "italic"),
	)
	rule(".nws-grid",
		prop("stroke", "var(--border)"),
		prop("stroke-width", "1"),
		prop("stroke-dasharray", "3 4"),
	)
	// The two halves NAMED where they sit, so the reader never decodes which is
	// which from a caption.
	rule(".nws-annos",
		prop("position", "absolute"),
		prop("inset", "0 0 0 3.4rem"),
		prop("pointer-events", "none"),
	)
	rule(".nws-anno",
		prop("position", "absolute"),
		prop("right", "0.4rem"),
		prop("transform", "translateY(-50%)"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-end"),
		prop("gap", "0.02rem"),
		prop("padding", "0.15rem 0.4rem"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "color-mix(in srgb, var(--bg-card) 86%, transparent)"),
		prop("line-height", "1.2"),
		prop("white-space", "nowrap"),
	)
	rule(".nws-anno-label",
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".nws-anno-value",
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".nws-anno.is-assets .nws-anno-value",
		prop("color", "var(--accent)"),
	)
	rule(".nws-anno.is-gap",
		prop("border", "1px solid var(--border)"),
	)
	// Ticks are PLACED at their point's position in the plot rather than laid
	// out as equal boxes. Equal boxes were what turned an all-time axis into a
	// run of first letters: 37 points sharing a 900px gutter gives each 24px,
	// and every caption ellipsised to "J". A label that has shrunk to an
	// initial costs the space of a label and gives none of the information, so
	// the density is now planned (balancesheet.TimeAxisTicks) and each surviving
	// label keeps its whole word.
	rule(".nws-xaxis",
		prop("position", "relative"),
		prop("height", "1.15rem"),
		prop("margin-left", "3.4rem"),
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".nws-xtick",
		prop("position", "absolute"),
		prop("top", "0"),
		prop("white-space", "nowrap"),
		prop("transform", "translateX(-50%)"),
	)
	// The end labels sit inside the plot rather than hanging off it.
	rule(".nws-xtick.is-first",
		prop("transform", "none"),
	)
	rule(".nws-xtick.is-last",
		prop("transform", "translateX(-100%)"),
	)
	// As the pane narrows, the plan's minor ticks go and its major ones stay —
	// the axis thins instead of colliding.
	ruleContentMax(contentTwoCol, ".nws-xtick.is-minor",
		prop("display", "none"),
	)

	// ── THE PACE RAIL. It shares the plot's x scale, so the distance between
	// two rungs IS the time between them and the legs visibly shorten as the
	// climb speeds up. Everything here stays quiet so that geometry is the one
	// thing the eye reads.
	rule(".nws-pace",
		prop("margin", "0.55rem 0 0 3.4rem"),
	)
	rule(".nws-pace-track",
		prop("position", "relative"),
		prop("height", "2.35rem"),
	)
	// A leg is the time it took, drawn as the space it occupied.
	rule(".nws-pace-leg",
		prop("position", "absolute"),
		prop("top", "0.32rem"),
		prop("height", "2px"),
		prop("background", "var(--accent)"),
		prop("opacity", "0.35"),
		prop("border-radius", "999px"),
	)
	rule(".nws-pace-legtime",
		prop("position", "absolute"),
		prop("left", "50%"),
		prop("top", "-0.62rem"),
		prop("transform", "translateX(-50%)"),
		prop("padding", "0 0.3rem"),
		prop("background", "var(--bg-card)"),
		prop("font-size", "var(--type-11, 0.6875rem)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("color", "var(--text-dim)"),
		prop("white-space", "nowrap"),
	)
	rule(".nws-pace-rung",
		prop("position", "absolute"),
		prop("top", "0"),
		prop("transform", "translateX(-50%)"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "center"),
		prop("gap", "0.1rem"),
		prop("white-space", "nowrap"),
	)
	rule(".nws-pace-rung:first-of-type",
		prop("transform", "none"),
		prop("align-items", "flex-start"),
	)
	rule(".nws-pace-rung:last-of-type",
		prop("transform", "translateX(-100%)"),
		prop("align-items", "flex-end"),
	)
	rule(".nws-pace-dot",
		prop("width", "7px"),
		prop("height", "7px"),
		prop("border-radius", "999px"),
		prop("background", "var(--accent)"),
	)
	rule(".nws-pace-value",
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("font-weight", "600"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("color", "var(--text)"),
	)
	rule(".nws-pace-when",
		prop("font-size", "var(--type-11, 0.6875rem)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".nws-pace-rung.is-tight .nws-pace-when",
		prop("display", "none"),
	)
	rule(".nws-pace-foot",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "0.4rem"),
		prop("margin-top", "0.15rem"),
	)
	rule(".nws-pace-read",
		prop("margin", "0"),
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("color", "var(--text-dim)"),
	)
	// The projection sits OFF the track and wears a dashed edge, because it is
	// an extrapolation and not part of the record.
	rule(".nws-pace-next",
		prop("padding", "0.12rem 0.55rem"),
		prop("font-size", "var(--type-11, 0.6875rem)"),
		prop("font-weight", "600"),
		prop("color", "var(--text-dim)"),
		prop("border", "1px dashed var(--border)"),
		prop("border-radius", "999px"),
		prop("white-space", "nowrap"),
	)
	ruleContentMax(contentGrid1, ".nws-pace",
		prop("margin-left", "2.8rem"),
	)

	// ── The crossings, marked on the plot where they happened.
	rule(".nws-mark",
		prop("stroke", "var(--accent)"),
		prop("stroke-width", "1"),
		prop("opacity", "0.5"),
	)
	rule(".nws-mark-cap",
		prop("fill", "var(--accent)"),
		prop("stroke", "none"),
	)
	// A setback is recorded in the same place, in a quieter hand — the record
	// stays truthful without painting a fall as an achievement.
	rule(".nws-mark.is-down",
		prop("stroke", "var(--text-dim)"),
		prop("stroke-dasharray", "3 3"),
		prop("opacity", "0.55"),
	)

	// ── The "?" explainer, matching the number-provenance affordance the Annual
	// Review masthead figures already use.
	rule(".nws-explain",
		prop("display", "inline-flex"),
		prop("position", "relative"),
	)
	rule(".nws-explain-btn",
		prop("appearance", "none"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("width", "1.3rem"),
		prop("height", "1.3rem"),
		prop("padding", "0"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
		prop("background", "var(--bg)"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("font-weight", "600"),
		prop("line-height", "1"),
		prop("cursor", "pointer"),
	)
	rule(".nws-explain-btn:hover",
		prop("color", "var(--text)"),
		prop("border-color", "var(--text-dim)"),
	)
	rule(".nws-explain-pop",
		prop("width", "min(23rem, 78vw)"),
		prop("padding", "0.75rem 0.85rem"),
		prop("white-space", "normal"),
	)
	rule(".nws-explain-title",
		prop("font-weight", "600"),
		prop("margin-bottom", "0.35rem"),
	)
	rule(".nws-explain-line",
		prop("margin", "0 0 0.45rem"),
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.5"),
		prop("color", "var(--text-dim)"),
	)
	rule(".nws-explain-line:last-child",
		prop("margin-bottom", "0"),
	)
	rule(".nws-sides-plot",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("min-width", "0"),
	)
	rule(".nws-sides-svg",
		prop("display", "block"),
		prop("width", "100%"),
		prop("height", "158px"),
		prop("overflow", "visible"),
	)
	// The gap IS the net worth, so it is the only filled region on the chart —
	// and it is toned by MEANING: the accent while you own more than you owe,
	// the danger colour only if the two boundaries cross.
	rule(".nws-gap",
		prop("fill", "var(--accent)"),
		prop("opacity", "0.21"),
	)
	rule(".nws-gap.is-underwater",
		prop("fill", "var(--danger)"),
		prop("opacity", "0.22"),
	)
	rule(".nws-line-assets",
		prop("stroke", "var(--accent)"),
		prop("stroke-width", "2.5"),
		prop("stroke-linejoin", "round"),
		prop("stroke-linecap", "round"),
	)
	rule(".nws-line-liab",
		prop("stroke", "var(--text)"),
		prop("stroke-width", "2"),
		prop("opacity", "0.68"),
		prop("stroke-linejoin", "round"),
		prop("stroke-linecap", "round"),
	)
	// The gap MEASURED at both ends: even where the wedge is subtle, the story
	// still arrives as two figures the reader can subtract.
	rule(".nws-sides-ends",
		prop("display", "flex"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
		prop("padding-left", "3.4rem"),
	)
	rule(".nws-gap-value",
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("letter-spacing", "-0.01em"),
	)

	// ── Composition strips: what each side is made of, now.
	rule(".nws-strips",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(2, minmax(0, 1fr))"),
		prop("gap", "0.85rem 1.5rem"),
		prop("align-items", "start"),
		prop("min-width", "0"),
	)
	rule(".nws-strip",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.35rem"),
		prop("min-width", "0"),
	)
	rule(".nws-strip-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("font-size", "var(--type-13)"),
	)
	rule(".nws-strip-swatch",
		prop("width", "0.55rem"),
		prop("height", "0.55rem"),
		prop("border-radius", "2px"),
		prop("flex", "0 0 auto"),
	)
	rule(".nws-strip-title",
		prop("font-weight", "600"),
	)
	rule(".nws-strip-total",
		prop("margin-left", "auto"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	// One bar, segments sized by their share of THEIR OWN side.
	rule(".nws-strip-bar",
		prop("display", "flex"),
		prop("height", "0.6rem"),
		prop("border-radius", "999px"),
		prop("overflow", "hidden"),
		prop("background", "var(--border)"),
	)
	rule(".nws-strip-seg",
		prop("height", "100%"),
		prop("min-width", "2px"),
	)
	rule(".nws-strip-key",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.15rem 0.7rem"),
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".nws-legend-item",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
	)
	rule(".nws-legend-dot",
		prop("width", "0.55rem"),
		prop("height", "0.55rem"),
		prop("border-radius", "2px"),
		prop("flex", "0 0 auto"),
	)
	// One hue per side, stepped by alpha, so a side reads as ONE thing made of
	// parts rather than as several unrelated series. Shared by the strip
	// segments, their key dots and the strip swatches.
	registerNwsTones(".nws-strip-seg", ".nws-legend-dot", ".nws-strip-swatch")
}

// registerNwsTones emits the shared side/position swatch backgrounds for every
// element that carries them, so the key, the bar and the heading can never
// drift apart.
func registerNwsTones(selectors ...string) {
	steps := []struct {
		mod   string
		color string
		alpha string
	}{
		{"is-a0", "var(--accent)", "1"},
		{"is-a1", "var(--accent)", "0.6"},
		{"is-a2", "var(--accent)", "0.32"},
		{"is-a3", "var(--accent)", "0.18"},
		{"is-l0", "var(--text)", "0.62"},
		{"is-l1", "var(--text)", "0.36"},
		{"is-l2", "var(--text)", "0.18"},
	}
	for _, sel := range selectors {
		for _, st := range steps {
			rule(sel+"."+st.mod, prop("background", st.color), prop("opacity", st.alpha))
		}
	}
}

// registerNwsDetail emits the Detail view: the numbered section chips and the
// balance-sheet tables.
func registerNwsDetail() {
	// A jumped-to section lands clear of the app header AND the sticky index
	// above it. Without this the reader arrives mid-chart with no section title
	// on screen, which is exactly the moment they most need one.
	rule(".nws-section[id]",
		prop("scroll-margin-top", "5.5rem"),
	)
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
	rule(".nws-idx.is-current",
		prop("color", "var(--text)"),
		prop("background", "var(--bg)"),
		prop("font-weight", "600"),
	)
	rule(".nws-idx-sep",
		prop("width", "1px"),
		prop("align-self", "stretch"),
		prop("margin", "0.15rem 0.25rem"),
		prop("background", "var(--border)"),
	)
	rule(".nws-idx-back",
		prop("color", "var(--text-dim)"),
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
	// ── Rows that open in place. The toggle is the row's own first cell, so the
	// whole name is the target rather than a separate icon the reader must aim at.
	rule(".nws-drill-toggle",
		prop("appearance", "none"),
		prop("background", "transparent"),
		prop("border", "0"),
		prop("padding", "0"),
		prop("margin", "0"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("color", "inherit"),
		prop("font", "inherit"),
		prop("text-align", "left"),
		prop("cursor", "pointer"),
	)
	rule(".nws-drill-toggle:hover",
		prop("color", "var(--accent)"),
	)
	rule(".nws-drill-caret",
		prop("display", "inline-block"),
		prop("color", "var(--text-dim)"),
		prop("transition", "transform var(--motion-fast, 120ms) var(--ease-standard, ease)"),
	)
	rule(".nws-drill-toggle.is-open .nws-drill-caret",
		prop("transform", "rotate(90deg)"),
	)
	rule(".nws-drill-panel-row td",
		prop("background", "var(--bg)"),
		prop("padding", "0"),
	)
	rule(".nws-drill-panel",
		prop("padding", "0.7rem 0.85rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
	)
	rule(".nws-drill-note",
		prop("margin", "0"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
	)
	rule(".nws-facts",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(auto-fit, minmax(13rem, 1fr))"),
		prop("gap", "0.3rem 1.2rem"),
	)
	rule(".nws-fact",
		prop("display", "flex"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
		prop("padding", "0.2rem 0"),
		prop("border-bottom", "1px solid var(--border)"),
		prop("font-size", "var(--type-13)"),
	)
	rule(".nws-fact-k",
		prop("color", "var(--text-dim)"),
	)
	rule(".nws-fact-v",
		prop("font-variant-numeric", "tabular-nums"),
		prop("white-space", "nowrap"),
	)
	rule(".nws-drill-actions",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
	)

	rule(".nws-milestone-list",
		prop("list-style", "none"),
		prop("margin", "0"),
		prop("padding", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.3rem"),
	)
	rule(".nws-milestone",
		prop("font-size", "var(--type-13)"),
		prop("padding-left", "0.75rem"),
		prop("border-left", "2px solid var(--accent)"),
	)
	// A milestone lost is stated in the same voice, only untinted — the record
	// is not a trophy cabinet.
	rule(".nws-milestone.is-down",
		prop("border-left-color", "var(--border)"),
		prop("color", "var(--text-dim)"),
	)
	// The expander states the honest total on its face, so the collapsed list is
	// a disclosure rather than a cap.
	rule(".nws-milestones-more",
		prop("margin-top", "0.5rem"),
		prop("padding", "0.25rem 0.7rem"),
		prop("font-size", "var(--type-12, 0.75rem)"),
		prop("font-weight", "600"),
		prop("color", "var(--text-dim)"),
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
		prop("cursor", "pointer"),
		prop("transition", "color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard)"),
	)
	rule(".nws-milestones-more:hover",
		prop("color", "var(--text)"),
		prop("border-color", "var(--accent)"),
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
// nwsGlanceStack is the content width below which the Glance grid stops being
// two columns. Chosen from the waterfall's needs, not from the shared bento
// steps: below this the left column cannot fit the bridge's labels.
const nwsGlanceStack = 860

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
	// The Glance grid holds its two columns further down than the bento
	// breakpoints do, deliberately: at a 1202px viewport the content pane is
	// ~960px, and collapsing there would push the interpretation back below the
	// fold on exactly the common desktop size this layout exists to serve. It
	// stacks at 860px, the width below which the left column can no longer label
	// the waterfall honestly.
	ruleContentMax(nwsGlanceStack, ".nws-glance",
		prop("grid-template-columns", "1fr"),
	)
	ruleContentMax(contentGrid4, ".nws-strips",
		prop("grid-template-columns", "1fr"),
	)
	ruleContentMax(contentGrid1, ".nws-sides-svg",
		prop("height", "150px"),
	)
	ruleContentMax(contentGrid1, ".nws-plot",
		prop("padding-left", "2.8rem"),
	)
	ruleContentMax(contentGrid1, ".nws-annos",
		prop("inset", "0 0 0 2.8rem"),
	)
	ruleContentMax(contentGrid1, ".nws-sides-ends",
		prop("padding-left", "2.8rem"),
	)
	ruleContentMax(contentGrid1, ".nws-xaxis",
		prop("margin-left", "2.8rem"),
	)
	ruleContentMax(contentGrid1, ".nws-window",
		prop("margin-left", "0"),
	)
}
