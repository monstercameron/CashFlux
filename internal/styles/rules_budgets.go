// SPDX-License-Identifier: MIT

package styles

// registerBudgetsSurface emits the /budgets zero-based "by source" income-basis
// ledger: a grouped, tactile list of the household's income categories, each an
// include / hold-aside toggle with its last-month amount, under a live "budgeting
// against $X" total. It reuses the page's existing zbb vocabulary (uppercase micro-
// labels, tabular figures, the accent/positive/faint tones) so the new control reads
// as part of the same surface rather than a bolted-on panel. Registered after the
// generated sheet, so equal-specificity refinements win.
func registerBudgetsSurface() {
	// A named token for the savings allocation tone, so it inherits theme handling like
	// every other surface color instead of a bare literal hex (a soft indigo that stays
	// distinct from the green --accent used for expenses).
	rule(":root", customProp("--accent-savings", "#8b7cf6"))
	// The ledger is a quiet inset panel: it groups the sources visually and gives the
	// toggles a surface to sit on, distinct from the demoted spend bar below.
	rule(".zbb-sources",
		marginTop("0.2rem"),
		padding("0.55rem 0.7rem 0.35rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius)"),
	)
	// Header row: title on the left, bulk include / hold-aside actions on the right.
	rule(".zbb-sources-head",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.6rem"),
		paddingBottom("0.3rem"),
	)
	rule(".zbb-sources-title",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".zbb-sources-actions",
		display("flex"),
		alignItems("baseline"),
		gap("0.4rem"),
		prop("flex-shrink", "0"),
	)
	rule(".zbb-sources-act",
		background("transparent"),
		border("0"),
		padding("0"),
		cursor("pointer"),
		fontSize("0.72rem"),
		fontWeight("600"),
		color("var(--accent)"),
	)
	rule(".zbb-sources-act:hover", textDecoration("underline"))
	rule(".zbb-sources-actsep",
		color("var(--text-faint)"),
		fontSize("0.72rem"),
	)
	// The running total row, under the header: the figure the checked rows sum to, plus
	// how many sources are included.
	rule(".zbb-sources-total",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.15rem 0.4rem"),
		paddingBottom("0.35rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".zbb-sources-total-cap",
		fontSize("0.66rem"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".zbb-sources-total-val",
		fontSize("1.05rem"),
		fontWeight("800"),
		color("var(--accent)"),
	)
	rule(".zbb-sources-count", fontSize("0.72rem"))
	// Cap the height once the list is long, so a household with many income categories
	// scrolls the ledger instead of stretching the modal past the viewport.
	rule(".zbb-sources-rows",
		display("flex"),
		flexDirection("column"),
		maxHeight("264px"),
		overflowY("auto"),
	)
	// One income source. Default (unchecked) is the held-aside state — muted, so the
	// eye lands on what IS funding the budget.
	rule(".zbb-source",
		display("flex"),
		alignItems("center"),
		gap("0.55rem"),
		padding("0.4rem 0.15rem"),
		cursor("pointer"),
		borderTop("1px solid var(--border)"),
		color("var(--text-faint)"),
	)
	rule(".zbb-sources-rows .zbb-source:first-child",
		borderTop("0"),
	)
	rule(".zbb-source:hover",
		background("var(--bg-card)"),
	)
	rule(".zbb-source .cf-check",
		flexShrink("0"),
	)
	rule(".zbb-source-name",
		flex("1"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontWeight("500"),
	)
	rule(".zbb-source-aside",
		flexShrink("0"),
		fontSize("0.62rem"),
		fontWeight("700"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".zbb-source-amt",
		flexShrink("0"),
		fontWeight("600"),
		color("var(--text-faint)"),
	)
	// A category with no income last month: a plain italic note, not a figure — so it
	// reads as "nothing here" rather than a source you forgot to include.
	rule(".zbb-source-none",
		fontSize("0.74rem"),
		fontStyle("italic"),
		fontWeight("400"),
	)
	// Included: the source counts toward the budget — name reads full-strength, its
	// amount in the positive (money-in) tone, and the held-aside tag is gone.
	rule(".zbb-source.is-in",
		color("var(--text)"),
	)
	rule(".zbb-source.is-in .zbb-source-name",
		color("var(--text)"),
	)
	rule(".zbb-source.is-in .zbb-source-amt",
		color("var(--money-positive)"),
		fontWeight("700"),
	)
	rule(".zbb-sources-empty",
		margin("0"),
		padding("0.4rem 0.1rem"),
		fontSize("0.82rem"),
	)

	// --- zero-based hero: header row + allocation bar + legend --------------------
	// The eyebrow label and the income button share the top row so the action reads as
	// part of the hero rather than a stray button.
	rule(".zbb-hero-top",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.6rem"),
	)
	// A quiet caption above the bar naming the income pool it splits.
	rule(".zbb-alloc-cap",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.15rem 0.5rem"),
		marginTop("0.7rem"),
	)
	rule(".zbb-alloc-cap-label",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	rule(".zbb-alloc-cap-val",
		fontSize("0.9rem"),
		fontWeight("700"),
		color("var(--text)"),
	)
	rule(".zbb-alloc-cap-note",
		fontSize("0.72rem"),
		color("var(--text-dim)"),
	)
	// A non-clipping wrapper positions the income marker so its tick can protrude past
	// the bar (which clips its own segments).
	rule(".zbb-alloc-wrap",
		position("relative"),
		marginTop("0.35rem"),
	)
	// The allocation bar: income split into expenses, savings, and the unassigned gap.
	// The track color is the "empty" tone, so any rounding remainder reads as gap.
	rule(".zbb-alloc",
		position("relative"),
		display("flex"),
		height("16px"),
		overflow("hidden"),
		borderRadius("var(--radius)"),
		background("color-mix(in srgb, var(--text-faint) 26%, transparent)"),
	)
	rule(".zbb-alloc-seg",
		height("100%"),
		transition("width var(--wonder-dur) var(--wonder-ease-out)"),
	)
	rule(".zbb-alloc-seg.is-exp", background("var(--accent)"))
	// A hairline (inset shadow) separates savings from the expenses segment beside it.
	rule(".zbb-alloc-seg.is-sav",
		background("var(--accent-savings)"),
		prop("box-shadow", "inset 1px 0 0 var(--bg-card)"),
	)
	// The unassigned segment uses the same tone as its legend dot, so "still open" reads
	// as a real color in both the bar and the legend (not just the empty track).
	rule(".zbb-alloc-seg.is-gap", background("color-mix(in srgb, var(--text-faint) 45%, transparent)"))
	// Income reference marker: a vertical tick showing where actual income runs out, so
	// the fill past it (when over-assigned) reads as the overage.
	rule(".zbb-alloc-marker",
		position("absolute"),
		top("-2px"),
		bottom("-2px"),
		width("2px"),
		marginLeft("-1px"),
		background("var(--text)"),
		prop("box-shadow", "0 0 0 1px var(--bg-card)"),
	)
	// The legend ties each amount to its bar color.
	rule(".zbb-legend",
		display("flex"),
		flexWrap("wrap"),
		gap("0.3rem 1.2rem"),
		marginTop("0.55rem"),
	)
	rule(".zbb-legend-item",
		display("flex"),
		alignItems("baseline"),
		gap("0.4rem"),
	)
	rule(".zbb-legend-dot",
		width("0.6rem"),
		height("0.6rem"),
		borderRadius("var(--radius)"),
		prop("flex-shrink", "0"),
		prop("align-self", "center"),
	)
	rule(".zbb-legend-dot.is-exp", background("var(--accent)"))
	rule(".zbb-legend-dot.is-sav", background("var(--accent-savings)"))
	rule(".zbb-legend-dot.is-gap", background("color-mix(in srgb, var(--text-faint) 45%, transparent)"))
	// The over-assigned swatch is a vertical tick (matching the bar's income marker), not
	// a round dot — it reads as a threshold reading, not an additive fourth slice.
	rule(".zbb-legend-dot.is-over",
		width("2px"),
		height("0.85rem"),
		borderRadius("0"),
		background("var(--text)"),
		prop("box-shadow", "0 0 0 1px var(--bg-card)"),
	)
	rule(".zbb-legend-label",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
	)
	rule(".zbb-legend-val",
		fontSize("0.82rem"),
		fontWeight("700"),
		color("var(--text)"),
	)
	// The over-assigned legend figure reads in the danger tone (matches the headline).
	rule(".zbb-legend-val-over", color("var(--money-negative)"))

	// --- the income button (page) + the basis modal (config lives in the modal) -----
	rule(".zbb-basis-open",
		prop("flex-shrink", "0"),
	)
	// Simple/envelope summary: the income context line + the income button on one row.
	rule(".budget-basis-row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.5rem 0.9rem"),
		marginTop("0.5rem"),
	)
	rule(".zbb-basis-modal",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		padding("0.2rem 0.1rem"),
	)
	rule(".zbb-basis-modal-help",
		margin("0"),
		fontSize("0.85rem"),
		lineHeight("1.45"),
	)

	// --- 2026-07-12 refinements: sort picker, formulas modal, top-up funding, add template ---

	// Toolbar: budgets now uses the shared two-row .filter-toolbar (matching the
	// transactions/accounts toolbar). It has no free-text search, so the primary row
	// carries the two view-shaping pickers (budgeting method + sort). They share the
	// line — each grows equally — and each picker's <select> fills its pill so the two
	// controls read as one deliberate row instead of two small huddled capsules.
	rule(".budgets-tb .filter-toolbar-primary",
		flexWrap("wrap"),
	)
	rule(".budgets-tb .filter-toolbar-primary .fctrl",
		flex("1 1 0"),
		minWidth("13rem"),
	)
	rule(".budgets-tb .filter-toolbar-primary .fctrl .fctrl-select",
		flex("1 1 auto"),
		width("auto"),
	)

	// Formulas modal: copyable variable → value rows.
	rule(".budget-formulas",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		marginTop("0.4rem"),
	)
	rule(".budget-formula-row",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		padding("0.45rem 0.6rem"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("var(--bg-card)"),
	)
	// The variable name is itself a click-to-copy control, monospace + accent so it reads
	// as a code token.
	rule(".budget-formula-name",
		flex("1"),
		minWidth("0"),
		prop("font-family", "ui-monospace, SFMono-Regular, Menlo, monospace"),
		fontSize("0.8rem"),
		color("var(--accent)"),
		background("transparent"),
		border("0"),
		padding("0"),
		prop("text-align", "left"),
		cursor("pointer"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".budget-formula-name:hover", textDecoration("underline"))
	rule(".budget-formula-val",
		prop("font-variant-numeric", "tabular-nums"),
		fontWeight("600"),
		color("var(--text)"),
		whiteSpace("nowrap"),
	)
	rule(".budget-formula-copy",
		flexShrink("0"),
		padding("0.2rem 0.4rem"),
	)

	// Top-up: "fund from other budgets" checklist.
	rule(".budget-topup-cover",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
		marginTop("0.5rem"),
		padding("0.65rem 0.7rem"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		background("var(--hover)"),
	)
	rule(".budget-topup-src",
		display("flex"),
		alignItems("center"),
		gap("0.55rem"),
		padding("0.3rem 0.15rem"),
		cursor("pointer"),
	)
	rule(".budget-topup-src .row-main",
		flexDirection("row"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.5rem"),
		flex("1"),
		minWidth("0"),
	)

	// Add-budget: the 50/30/20 template banner + the "or" divider.
	rule(".budget-add-tmpl",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.9rem"),
		padding("0.75rem 0.9rem"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		background("var(--hover)"),
	)
	rule(".budget-add-tmpl-title",
		display("block"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".budget-add-tmpl .btn", flexShrink("0"))
	// The banner's start-from actions: the 50/30/20 button + the copy-existing select
	// stack as one right-aligned group.
	rule(".budget-add-tmpl-actions",
		display("flex"),
		flexDirection("column"),
		alignItems("stretch"),
		gap("0.4rem"),
		flexShrink("0"),
	)
	// Category-fate + owner-scope hints: quiet one-liners directly under their fields.
	rule(".budget-cat-fate, .budget-owner-hint",
		display("block"),
		fontSize("0.78rem"),
		lineHeight("1.35"),
		marginTop("-0.35rem"),
	)
	// 50/30/20 review list: one checkbox row per proposal, amount right-aligned.
	rule(".budget-tmpl-rows",
		display("flex"),
		flexDirection("column"),
		margin("0.35rem 0"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		overflow("hidden"),
	)
	rule(".budget-tmpl-row",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		padding("0.55rem 0.8rem"),
		cursor("pointer"),
		borderBottom("1px solid color-mix(in srgb, var(--border) 55%, transparent)"),
	)
	rule(".budget-tmpl-row:last-child", borderBottom("0"))
	rule(".budget-tmpl-row:hover", background("var(--hover)"))
	rule(".budget-tmpl-name",
		flex("1 1 auto"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".budget-tmpl-amt",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
		whiteSpace("nowrap"),
	)
	rule(".budget-tmpl-total",
		margin("0.25rem 0 0"),
		textAlign("right"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".budget-add-or",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		margin("0.15rem 0"),
		color("var(--text-dim)"),
		fontSize("0.7rem"),
		textTransform("uppercase"),
		letterSpacing("0.06em"),
	)
	rule(".budget-add-or::before, .budget-add-or::after",
		content("\"\""),
		flex("1"),
		height("1px"),
		background("var(--border)"),
	)

	// G1: the inline limit editor inside the card's loader bar. The limit figure is a
	// quiet button (dotted underline on hover signals "editable in place"); editing
	// swaps it for a compact number input + save/cancel.
	rule(".budget-limit-btn",
		prop("appearance", "none"),
		background("transparent"),
		border("0"),
		padding("0"),
		margin("0"),
		font("inherit"),
		color("inherit"),
		cursor("pointer"),
		borderRadius("4px"),
		textDecoration("underline"),
		prop("text-decoration-style", "dotted"),
		prop("text-decoration-color", "transparent"),
		prop("text-underline-offset", "3px"),
		transition("text-decoration-color 0.12s ease"),
	)
	rule(".budget-limit-btn:hover",
		prop("text-decoration-color", "var(--text-dim)"),
	)
	rule(".budget-limit-btn:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".budget-limit-editform",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
	)
	rule(".budget-limit-editform .budget-limit-input",
		width("110px"),
		minHeight("28px"),
		padding("0.15rem 0.45rem"),
		fontSize("0.9rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".budget-limit-editform .btn",
		minHeight("28px"),
		padding("0.15rem 0.4rem"),
	)
	// The "base limit" tag: shown while editing when carry/boost make the cap differ
	// from the base, so the button→input number swap reads as intentional.
	rule(".budget-limit-basetag",
		fontSize("0.62rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
		whiteSpace("nowrap"),
	)

	// C1: the edit form's link-out to the dedicated tracked-categories editor.
	rule(".budget-edit-cats-link",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.6rem"),
		padding("0.5rem 0.7rem"),
		border("1px dashed var(--border)"),
		borderRadius("8px"),
	)

	// G8: the "Unbudgeted spending" strip — an invitation to budget the categories
	// that are actually taking money this month. Quiet dashed frame so it reads as an
	// offer, not another data card.
	rule(".budget-unbudgeted",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		margin("0.85rem 0 0.35rem"),
		padding("0.7rem 0.85rem"),
		border("1px dashed var(--border)"),
		borderRadius("10px"),
	)
	rule(".budget-unbudgeted-head",
		display("flex"),
		alignItems("baseline"),
		gap("0.6rem"),
		flexWrap("wrap"),
	)
	rule(".budget-unbudgeted-title",
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".budget-unbudgeted-chips",
		display("flex"),
		flexWrap("wrap"),
		gap("0.45rem"),
	)
	rule(".budget-unbudgeted-chip",
		prop("appearance", "none"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.55rem"),
		padding("0.4rem 0.7rem"),
		border("1px solid var(--border)"),
		borderRadius("999px"),
		background("var(--bg-elev)"),
		color("var(--text)"),
		font("inherit"),
		fontSize("0.85rem"),
		cursor("pointer"),
		transition("border-color 0.12s ease, background 0.12s ease"),
	)
	rule(".budget-unbudgeted-chip:hover",
		borderColor("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 10%, var(--bg-elev))"),
	)
	rule(".budget-unbudgeted-chip:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".budget-unbudgeted-cta",
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		fontWeight("600"),
		color("var(--accent)"),
		whiteSpace("nowrap"),
	)

	// Budget-card notes line reuses the /accounts .acct-notes treatment; a touch of
	// separation from the metadata above.
	rule(".budget-notes", marginTop("0.5rem"))

	// Notes modal: the textarea grows to fill the modal so there's room to write. The
	// scroll region is a flex column; the labeled field and its textarea both flex-grow.
	rule(".budget-notes-scroll > .labeled-field",
		flex("1"),
		display("flex"),
		flexDirection("column"),
		minHeight("0"),
	)
	rule(".budget-notes-scroll textarea",
		flex("1"),
		width("100%"),
		minHeight("12rem"),
		prop("resize", "none"),
	)

	// --- Sweep-leftovers config modal ------------------------------------------------
	// The "Budgets to sweep" participation list is a bordered inset box (like the top-up
	// funding checklist), scrolling once a household has many budgets so it never balloons
	// the modal. Each row is an aligned checkbox + name with a quiet hover.
	rule(".sweep-budgets",
		display("flex"),
		flexDirection("column"),
		maxHeight("15rem"),
		overflowY("auto"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		background("var(--bg-elev)"),
		padding("0.2rem 0.35rem"),
	)
	rule(".sweep-check-row",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		padding("0.4rem 0.35rem"),
		borderRadius("7px"),
		cursor("pointer"),
		color("var(--text)"),
		transition("background .12s ease"),
	)
	rule(".sweep-check-row:hover",
		background("var(--bg-card)"),
	)
	rule(".sweep-check-row .t-body",
		flex("1"),
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)

	// --- Age of Money stat (budgets summary) -----------------------------------------
	// YNAB's signature buffer metric, surfaced as a calm insight card just under the
	// spend bar. An accent spine on the left marks it as a distinct read without
	// shouting; the day count borrows the page's serif hero-figure signature so it
	// speaks the same typographic language as the spent/left figures above it.
	rule(".budget-agemoney",
		// A subordinate INSIGHT strip, not a fourth hero stat — sits under the
		// SPENT/BUDGETED/LEFT hero with a quiet accent stripe and a compact figure.
		marginTop("0.55rem"),
		padding("0.5rem 0.75rem"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		prop("border-left", "2px solid var(--accent)"),
		borderRadius("var(--radius)"),
	)
	rule(".budget-agemoney-head",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.6rem"),
	)
	rule(".budget-agemoney-label",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	// The "Why?" affordance: a quiet, keyboard-reachable dotted-underline note whose
	// title tooltip explains the FIFO matching. Accent-toned but understated.
	rule(".budget-agemoney-why",
		fontSize("0.72rem"),
		fontWeight("600"),
		color("var(--accent)"),
		cursor("help"),
		prop("border-bottom", "1px dotted var(--accent)"),
		prop("flex-shrink", "0"),
	)
	rule(".budget-agemoney-why:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
		borderRadius("3px"),
	)
	// The figure: a large serif numeral (the app's clearest typographic signature) with
	// a small unit word riding the baseline beside it.
	rule(".budget-agemoney-fig",
		display("flex"),
		alignItems("baseline"),
		gap("0.3rem"),
		marginTop("0.35rem"),
	)
	rule(".budget-agemoney-num",
		// Serif (consistent with the budget hero figures) but sized as an insight, not a
		// hero — clearly smaller than the 2rem SPENT/BUDGETED/LEFT numbers above it.
		prop("font-family", "var(--font-display), Fraunces, Georgia, serif"),
		fontSize("1.25rem"),
		fontWeight("700"),
		letterSpacing("-0.01em"),
		prop("line-height", "1"),
		prop("font-variant-numeric", "tabular-nums"),
		color("var(--text)"),
	)
	// Tone the count by buffer health: a healthy buffer reads positive-green; a tight
	// one takes the on-brand accent — calm, never the danger red of an error.
	rule(".budget-agemoney-fig.is-healthy .budget-agemoney-num", color("var(--money-positive)"))
	rule(".budget-agemoney-fig.is-tight .budget-agemoney-num", color("var(--accent)"))
	rule(".budget-agemoney-unit",
		fontSize("0.9rem"),
		fontWeight("600"),
		color("var(--text-dim)"),
	)
	rule(".budget-agemoney-explain",
		marginTop("0.3rem"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		prop("line-height", "1.4"),
	)

	// Sinking-fund shortfall alert: the sibling of the red .budget-over-banner, in the
	// caution amber — nothing is overspent yet, the PLAN doesn't add up. One icon, a bold
	// shortfall headline over a plain-English body, and the review action pinned right.
	rule(".budget-fundshort",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		padding("0.6rem 0.85rem"),
		margin("0.6rem 0"),
		borderRadius("8px"),
		borderLeft("4px solid var(--warn)"),
		background("color-mix(in srgb, var(--warn) 10%, var(--bg-elev))"),
	)
	rule(".budget-fundshort-icon", color("var(--warn)"))
	rule(".budget-fundshort-main",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
		prop("flex", "1 1 auto"),
		minWidth("0"),
	)
	rule(".budget-fundshort-title",
		fontWeight("700"),
		fontSize("0.9rem"),
		color("var(--text)"),
	)
	rule(".budget-fundshort-body",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		prop("line-height", "1.4"),
	)
	rule(".budget-fundshort-btn", prop("flex", "0 0 auto"), whiteSpace("nowrap"))

	// Historical (last-month overlay) fills are NEUTRAL: green is reserved for healthy
	// LIVE progress and amber for live warnings, so last month's spend renders as a
	// quiet theme-agnostic gray band. Overruns keep danger red — a fact is a fact,
	// whenever it happened. (Design critique: green was doing too many jobs.)
	rule(".bar-fill.is-hist",
		background("color-mix(in srgb, var(--text) 22%, transparent)"),
		boxShadow("none"),
	)
	rule(".budget-loader-fill.is-hist",
		background("linear-gradient(90deg, color-mix(in srgb, var(--text) 18%, transparent), color-mix(in srgb, var(--text) 9%, transparent))"),
		borderRight("2px solid color-mix(in srgb, var(--text) 40%, transparent)"),
	)
	// The LAST MONTH tag follows: a neutral chip, not the accent used for live/healthy.
	rule(".bento-budgets .budget-lastmonth-tag",
		color("var(--text-dim)"),
		background("color-mix(in srgb, var(--text) 10%, transparent)"),
	)

	// Compact density: the budget list as a LEDGER — one hairline row per budget with a
	// left state stripe, a mini meter, and tabular figures. The card layout's job is
	// depth; this one's is span (fifteen categories without scrolling).
	rule(".budget-clist",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
	)
	rule(".budget-crow",
		display("grid"),
		gridTemplateColumns("minmax(8rem, 1.2fr) minmax(7rem, 1.6fr) max-content max-content max-content max-content"),
		alignItems("center"),
		gap("0.8rem"),
		padding("0.4rem 0.6rem 0.4rem 0.75rem"),
		border("1px solid var(--border)"),
		borderLeft("3px solid var(--border)"),
		borderRadius("8px"),
		background("var(--bg-card)"),
	)
	rule(".budget-crow.is-over", borderLeftColor("var(--danger)"))
	rule(".budget-crow.is-near", borderLeftColor("var(--warn)"))
	rule(".budget-crow.is-risk", borderLeftColor("var(--warn)"))
	rule(".budget-crow.is-ontrack", borderLeftColor("color-mix(in srgb, var(--accent) 55%, var(--border))"))
	rule(".budget-crow-name",
		fontWeight("600"),
		fontSize("0.88rem"),
		color("var(--text)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		textAlign("left"),
	)
	rule("button.budget-crow-name",
		background("transparent"),
		border("0"),
		padding("0"),
		margin("0"),
		font("inherit"),
		cursor("pointer"),
	)
	rule(".budget-crow-bar",
		position("relative"),
		height("8px"),
		borderRadius("999px"),
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		overflow("hidden"),
	)
	rule(".budget-crow-bar .bar-fill",
		position("absolute"),
		top("0"),
		left("0"),
		bottom("0"),
		height("100%"),
		borderRadius("0"),
		boxShadow("none"),
	)
	rule(".budget-crow-amt",
		fontSize("0.85rem"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".budget-crow-left",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".budget-crow-chip", whiteSpace("nowrap"))
	// In the chip slot the LAST MONTH tag is an inline chip, not an overline above a bar.
	rule(".budget-crow .budget-lastmonth-tag", margin("0"), alignSelf("center"))
	// Narrow columns: drop the meter and the left phrase, keep name/amount/chip/menu.
	ruleContentMax(860-railCollapsedPx, ".budget-crow",
		gridTemplateColumns("minmax(7rem, 1fr) max-content max-content max-content"),
	)
	ruleContentMax(860-railCollapsedPx, ".budget-crow-bar", display("none"))
	ruleContentMax(860-railCollapsedPx, ".budget-crow-left", display("none"))

	// --- 2026-07-17 audit P0: the summary's three-column status strip ---
	// Plan (To-Assign hero / income basis) · Spending (the loader) · Age of money
	// side by side, so the first budget category lands in the initial viewport.
	rule(".budget-status-strip",
		display("grid"),
		gridTemplateColumns("minmax(0, 1.1fr) minmax(0, 1.3fr) minmax(0, 0.8fr)"),
		gap("0.75rem 1.4rem"),
		alignItems("stretch"),
	)
	rule(".budget-strip-cell",
		minWidth("0"),
		display("flex"),
		flexDirection("column"),
		justifyContent("flex-start"),
	)
	rule(".budget-strip-cell.is-spend, .budget-strip-cell.is-age",
		borderLeft("1px solid var(--border)"),
		paddingLeft("1.4rem"),
	)
	rule(".budget-strip-label",
		fontSize("0.7rem"),
		fontWeight("700"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	// Inside the strip the zero-based hero is one column, not the page centrepiece:
	// tighter padding and a page-KPI-sized figure (audit type scale), not a hero.
	rule(".budget-status-strip .zbb-hero",
		padding("0.1rem 0 0.2rem"),
		gap("0.25rem"),
	)
	rule(".budget-status-strip .zbb-figure",
		fontSize("1.9rem"),
	)
	rule(".budget-status-strip .budget-loader",
		minHeight("84px"),
		marginBottom("0.4rem"),
	)
	// The loader now lives in a ~1/3-width column: the full-size serif figures
	// (1.45/2rem, 1.5rem side padding) collided into each other there. Scale the
	// figures to the column, tighten the padding, and let the middle "Budgeted"
	// cell truncate before anything overlaps.
	rule(".budget-status-strip .budget-loader-figs",
		minHeight("84px"),
		padding("0 0.8rem"),
		gap("0.5rem"),
	)
	rule(".budget-status-strip .budget-loader-value",
		fontSize("1.05rem"),
	)
	rule(".budget-status-strip .budget-loader-value.is-hero",
		fontSize("1.35rem"),
	)
	rule(".budget-status-strip .budget-loader-fig",
		overflow("hidden"),
	)
	rule(".budget-status-strip .budget-loader-fig .budget-loader-value",
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	rule(".budget-status-strip .budget-agemoney",
		marginTop("0"),
	)
	ruleContentMax(contentTwoCol, ".budget-status-strip",
		gridTemplateColumns("1fr"),
	)
	ruleContentMax(contentTwoCol, ".budget-strip-cell.is-spend, .budget-strip-cell.is-age",
		borderLeft("0"),
		paddingLeft("0"),
	)

	// --- the collapsed "N issues need attention" rail (replaces stacked banners) ---
	rule(".budget-issues-wrap",
		marginTop("0.6rem"),
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
	)
	rule(".budget-issues-rail",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		width("100%"),
		textAlign("left"),
		padding("0.45rem 0.7rem"),
		background("color-mix(in srgb, #f59e0b 8%, transparent)"),
		border("1px solid color-mix(in srgb, #f59e0b 45%, var(--border))"),
		borderRadius("10px"),
		color("var(--text)"),
		cursor("pointer"),
		font("inherit"),
		fontSize("0.85rem"),
		fontWeight("600"),
	)
	rule(".budget-issues-icon", color("#f59e0b"))
	rule(".budget-issues-title", flex("1"), minWidth("0"))
	// The over-assignment "Resolve $X" figure is the rail's call to action.
	rule(".budget-rail-resolve",
		padding("0.15rem 0.55rem"),
		borderRadius("999px"),
		background("var(--danger)"),
		color("#fff"),
		fontWeight("700"),
		fontSize("0.78rem"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".budget-issues-chev", color("var(--text-faint)"))
	rule(".budget-issues-detail",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
	)
	rule(".budget-issue-row",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		padding("0.5rem 0.7rem"),
		border("1px solid var(--border)"),
		borderRadius("10px"),
		background("var(--bg-elev)"),
	)
	rule(".budget-issue-main",
		flex("1"),
		minWidth("0"),
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
	)
	rule(".budget-issue-title",
		fontWeight("600"),
		fontSize("0.85rem"),
	)
	rule(".budget-issue-body",
		fontSize("0.78rem"),
		color("var(--text-dim)"),
	)
	// W6 rows/styling lane: the per-row rollover policy badge (C395). Chained here
	// rather than in the contended install.go.
	registerBudgetRolloverBadge()
}
