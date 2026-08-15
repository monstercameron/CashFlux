// SPDX-License-Identifier: MIT

package styles

// registerReviewSurface emits the dual-mode review surface (C500–C512): the
// tiered merchant list, the single-charge view with its context band, the
// pinned action bar, and the shared category picker.
//
// Every colour is a token. The surface lives inside a 90vw × 90vh FlipPanel, so
// the body scrolls and the footer pins — nothing here may introduce a second
// scroll container.
func registerReviewSurface() {
	// ---- shell ---------------------------------------------------------------
	rule(".rvs",
		// --rvs-tail is the caret column plus its gap. The member rows reserve the
		// SAME space on their right, so the money column lines up between a
		// merchant and the charges inside it instead of staggering by 35px.
		// The panel is mounted with FlushBody, which strips ITS padding so this
		// body can pin its own footer. The gutter therefore has to live here —
		// without it the rows and the keyboard hint sat flush against the panel
		// edge, which is what read as "crooked".
		customProp("--rvs-gutter", "1.35rem"),
		customProp("--rvs-tail", "2.05rem"),
		// --rvs-indent puts a charge's date under the merchant's name, not 3px off.
		customProp("--rvs-indent", "2.8rem"),
		display("flex"),
		flexDirection("column"),
		height("100%"),
		minHeight("0"),
	)
	rule(".rvs-head .seg button",
		whiteSpace("nowrap"),
	)
	rule(".rvs-head",
		display("flex"),
		alignItems("center"),
		gap("0.9rem"),
		flexWrap("wrap"),
		flex("none"),
		padding("0 var(--rvs-gutter) 0.85rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".rvs-kbd",
		marginLeft("auto"),
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
	)
	rule(".rvs-body",
		flex("1"),
		minHeight("0"),
		overflowY("auto"),
		padding("1.1rem var(--rvs-gutter) 0"),
	)
	rule(".rvs-foot",
		flex("none"),
		display("flex"),
		alignItems("center"),
		gap("0.9rem"),
		flexWrap("wrap"),
		padding("0.8rem var(--rvs-gutter)"),
		marginTop("0.4rem"),
		borderTop("1px solid var(--border)"),
	)
	rule(".rvs-foot-acts",
		marginLeft("auto"),
		display("flex"),
		gap("0.5rem"),
		flexWrap("wrap"),
	)
	rule(".rvs-foot-n", fontWeight("600"))
	rule(".rvs-foot-sub",
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
	)
	rule(".rvs-done",
		alignItems("center"),
		justifyContent("center"),
		textAlign("center"),
		gap("0.5rem"),
	)

	// ---- the signature line: charges collapse into decisions -----------------
	rule(".rvs-collapse",
		display("flex"),
		alignItems("baseline"),
		gap("0.6rem"),
		flexWrap("wrap"),
		marginBottom("0.35rem"),
	)
	rule(".rvs-big",
		fontFamily("var(--font-display), Georgia, serif"),
		fontSize("2rem"),
		fontWeight("600"),
		lineHeight("1"),
		letterSpacing("-0.02em"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rvs-big.is-accent", color("var(--accent)"))
	rule(".rvs-unit",
		color("var(--text-dim)"),
		fontSize("var(--type-13)"),
	)
	rule(".rvs-arrow",
		color("var(--text-faint)"),
		padding("0 0.3rem"),
	)
	rule(".rvs-note",
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
		maxWidth("66ch"),
		margin("0 0 1.1rem"),
	)
	rule(".rvs-notice",
		color("var(--accent)"),
		fontSize("var(--type-13)"),
		margin("0 0 0.8rem"),
	)
	rule(".rvs-warn",
		color("var(--danger)"),
		fontSize("var(--type-12)"),
		margin("0.6rem 0 0"),
	)

	// ---- SMART+ scan strip ---------------------------------------------------
	rule(".rvs-smart",
		display("flex"),
		alignItems("center"),
		gap("0.9rem"),
		flexWrap("wrap"),
		padding("0.8rem 0.9rem"),
		border("1px solid var(--border)"),
		background("var(--bg-elev)"),
		borderRadius("var(--radius-sm)"),
		marginBottom("1rem"),
	)
	rule(".rvs-smart-mark",
		width("26px"),
		height("26px"),
		flex("none"),
		display("grid"),
		prop("place-content", "center"),
		border("1px solid var(--accent)"),
		color("var(--accent)"),
		fontSize("var(--type-12)"),
		fontWeight("600"),
		borderRadius("var(--radius-xs)"),
	)
	rule(".rvs-smart-main", flex("1"), minWidth("240px"))
	rule(".rvs-smart-t", fontSize("var(--type-13)"), fontWeight("600"))
	rule(".rvs-smart-sub",
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
		marginTop("0.15rem"),
	)

	// ---- confidence legend ---------------------------------------------------
	rule(".rvs-legend",
		display("flex"),
		gap("1.1rem"),
		flexWrap("wrap"),
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
		marginBottom("1.1rem"),
	)
	rule(".rvs-legend > span",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
	)
	rule(".rvs-dot",
		width("8px"),
		height("8px"),
		flex("none"),
		borderRadius("var(--radius-pill)"),
		display("inline-block"),
		background("var(--border)"),
	)
	rule(".rvs-dot.is-ready", background("var(--accent)"))
	rule(".rvs-dot.is-look", background("var(--brand)"))
	rule(".rvs-dot.is-none", background("var(--border)"))

	// ---- tiers ---------------------------------------------------------------
	rule(".rvs-tier", marginBottom("1.4rem"))
	rule(".rvs-tier-head",
		display("flex"),
		alignItems("center"),
		gap("0.7rem"),
		flexWrap("wrap"),
		paddingBottom("0.5rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".rvs-tier-mark",
		width("3px"),
		alignSelf("stretch"),
		minHeight("18px"),
		background("var(--border)"),
	)
	rule(".rvs-tier.is-ready .rvs-tier-mark", background("var(--accent)"))
	rule(".rvs-tier.is-look .rvs-tier-mark", background("var(--brand)"))
	rule(".rvs-tier-name", fontWeight("600"), fontSize("var(--type-14)"))
	rule(".rvs-tier-count",
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
	)
	rule(".rvs-tier-act", marginLeft("auto"))

	// ---- merchant group row --------------------------------------------------
	rule(".rvs-grp", borderBottom("1px solid var(--border)"))
	rule(".rvs-grp-row",
		display("grid"),
		gridTemplateColumns("26px 1fr auto"),
		gap("0.8rem"),
		alignItems("center"),
		padding("0.65rem 0.4rem"),
		transition("background .12s ease"),
	)
	rule(".rvs-grp-row:hover", background("var(--bg-elev)"))
	// The keyboard's current row. Distinct from selection: focus is where you
	// are, selection is what you have chosen.
	rule(".rvs-grp.is-focus .rvs-grp-row",
		outline("2px solid var(--brand)"),
		outlineOffset("-2px"),
	)
	rule(".rvs-grp.is-sel .rvs-grp-row",
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
		boxShadow("inset 3px 0 0 var(--accent)"),
	)
	rule(".rvs-grp-main", minWidth("0"))
	rule(".rvs-grp-name",
		display("flex"),
		alignItems("baseline"),
		gap("0.5rem"),
		flexWrap("wrap"),
	)
	rule(".rvs-grp-n",
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
	)
	rule(".rvs-grp-raw",
		marginTop("0.1rem"),
		color("var(--text-faint)"),
		fontSize("var(--type-11)"),
		fontFamily("ui-monospace, SFMono-Regular, Menlo, monospace"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".rvs-grp-right",
		display("flex"),
		alignItems("center"),
		gap("0.7rem"),
		justifyContent("flex-end"),
	)
	rule(".rvs-grp-total",
		fontWeight("500"),
		minWidth("92px"),
		textAlign("right"),
		fontVariantNumeric("tabular-nums"),
	)
	// The category is an editable control, not a label: switching one must not
	// cost a trip through a modal (Cam, 2026-08-15).
	rule(".rvs-cat",
		background("var(--bg-elev)"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-sm)"),
		color("var(--text)"),
		font("inherit"),
		fontSize("var(--type-12)"),
		padding("0.3rem 0.45rem"),
		maxWidth("230px"),
	)
	rule(".rvs-cat:hover", background("var(--hover)"))
	// A category the user set by hand is marked, and auto-fill never overwrites it.
	rule(".rvs-cat[data-manual=\"true\"]", borderColor("var(--accent)"))
	rule(".rvs-caret",
		background("none"),
		border("0"),
		width("1.35rem"),
		flex("none"),
		textAlign("center"),
		color("var(--text-faint)"),
		padding("0.2rem 0.35rem"),
		lineHeight("1"),
		cursor("pointer"),
		transition("transform .12s ease"),
	)
	rule(".rvs-caret:hover", color("var(--text)"))
	rule(".rvs-grp.is-open .rvs-caret", transform("rotate(90deg)"))

	rule(".rvs-members",
		padding("0.1rem 0.4rem 0.7rem"),
		paddingLeft("var(--rvs-indent)"),
		// + 0.4rem for the group row's own right padding, or the money column
		// still lands 6px out.
		paddingRight("calc(var(--rvs-tail) + 0.4rem)"),
		background("var(--bg)"),
	)
	rule(".rvs-mem",
		display("grid"),
		gridTemplateColumns("92px 1fr auto"),
		gap("0.8rem"),
		alignItems("center"),
		padding("0.35rem 0"),
		borderTop("1px solid var(--border)"),
		fontSize("var(--type-13)"),
	)
	rule(".rvs-mem:first-child", borderTop("0"))
	rule(".rvs-mem-date",
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
	)
	rule(".rvs-mem-desc",
		color("var(--text-dim)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".rvs-mem-amt",
		fontWeight("500"),
		textAlign("right"),
		minWidth("92px"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rvs-more",
		color("var(--brand)"),
		fontSize("var(--type-12)"),
		margin("0.4rem 0 0"),
	)

	// ---- single mode: two columns so 90vw is earned, not wasted --------------
	rule(".rvs-single",
		display("grid"),
		gridTemplateColumns("minmax(0, 1fr) minmax(0, 400px)"),
		gap("1.4rem"),
		alignItems("start"),
		maxWidth("1180px"),
		margin("0 auto"),
	)
	rule(".rvs-assign",
		border("1px solid var(--border)"),
		borderTop("0"),
		background("var(--bg-elev)"),
		padding("1rem 1.1rem"),
	)
	rule(".rvs-why",
		color("var(--text-dim)"),
		fontSize("var(--type-12)"),
		marginTop("0.5rem"),
	)

	// ---- context band --------------------------------------------------------
	rule(".rvs-band",
		border("1px solid var(--border)"),
		background("var(--bg)"),
	)
	rule(".rvs-band-head",
		padding("0.6rem 0.9rem"),
		borderBottom("1px solid var(--border)"),
		color("var(--text-dim)"),
		fontSize("var(--type-11)"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
	)
	rule(".rvs-ctx",
		padding("0.8rem 0.9rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".rvs-ctx:last-child", borderBottom("0"))
	rule(".rvs-ctx.is-warn", background("color-mix(in srgb, var(--danger) 8%, transparent)"))
	rule(".rvs-ctx-t", marginBottom("0.45rem"))
	rule(".rvs-ctx-why",
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
		marginLeft("0.4rem"),
	)
	rule(".rvs-link-row",
		display("grid"),
		gridTemplateColumns("92px 88px 1fr"),
		gap("0.6rem"),
		alignItems("baseline"),
		padding("0.2rem 0"),
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
	)
	rule(".rvs-link-amt",
		color("var(--text)"),
		textAlign("right"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rvs-link-desc",
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)

	// ---- shared category picker ---------------------------------------------
	rule(".catpick-back",
		position("fixed"),
		inset("0"),
		background("rgba(0,0,0,.5)"),
		zIndex("var(--z-dialog)"),
		display("grid"),
		placeItems("start center"),
		paddingTop("10vh"),
	)
	rule(".catpick",
		width("min(94vw, 430px)"),
		maxHeight("72vh"),
		display("flex"),
		flexDirection("column"),
		background("var(--bg-card)"),
		border("1px solid var(--border)"),
		boxShadow("0 18px 48px rgba(0,0,0,.45)"),
	)
	rule(".catpick-search",
		padding("0.7rem"),
		borderBottom("1px solid var(--border)"),
	)
	rule(".catpick-list",
		overflowY("auto"),
		padding("0.25rem 0"),
	)
	rule(".catpick-opt",
		display("block"),
		width("100%"),
		background("none"),
		border("0"),
		textAlign("left"),
		padding("0.45rem 0.8rem"),
		fontSize("var(--type-13)"),
		color("var(--text)"),
		cursor("pointer"),
	)
	rule(".catpick-opt:hover", background("var(--hover)"))
	rule(".catpick-opt.is-child", paddingLeft("1.8rem"))
	rule(".catpick-opt.is-sel", color("var(--accent)"), fontWeight("600"))
	rule(".catpick-empty",
		padding("0.9rem 0.8rem"),
		color("var(--text-faint)"),
		fontSize("var(--type-12)"),
		margin("0"),
	)
	// The name is already typed by this point, so the only decision left is
	// WHERE the category goes.
	rule(".catpick-make",
		borderTop("1px solid var(--border)"),
		background("var(--bg-elev)"),
		padding("0.7rem 0.8rem"),
	)
	rule(".catpick-make-t",
		fontSize("var(--type-12)"),
		fontWeight("600"),
		marginBottom("0.5rem"),
	)
	rule(".catpick-make-row",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
	)
	rule(".catpick-make-row select", flex("1"), minWidth("140px"))
	rule(".catpick-foot",
		borderTop("1px solid var(--border)"),
		padding("0.5rem 0.8rem"),
		display("flex"),
		justifyContent("flex-end"),
	)

	// A machine-readable mirror of the queue total for tests and assistive tech.
	rule(".rvs-hidden",
		position("absolute"),
		width("1px"),
		height("1px"),
		overflow("hidden"),
		clip("rect(0 0 0 0)"),
		whiteSpace("nowrap"),
	)

	// ---- responsive ----------------------------------------------------------
	ruleMedia("(max-width: 1100px)", ".rvs-single",
		gridTemplateColumns("minmax(0, 1fr)"),
		maxWidth("680px"),
	)
	ruleMedia("(max-width: 700px)", ".rvs", customProp("--rvs-gutter", "0.85rem"))
	ruleMedia("(max-width: 900px)", ".rvs-grp-row",
		gridTemplateColumns("26px 1fr"),
		rowGap("0.55rem"),
	)
	ruleMedia("(max-width: 900px)", ".rvs-grp-right",
		gridColumn("2"),
		justifyContent("flex-start"),
		flexWrap("wrap"),
	)
	ruleMedia("(max-width: 900px)", ".rvs-grp-total",
		minWidth("0"),
		textAlign("left"),
	)
	ruleMedia("(max-width: 900px)", ".rvs-members", paddingLeft("1rem"), paddingRight("0.4rem"))
	ruleMedia("(max-width: 560px)", ".rvs-big", fontSize("1.6rem"))
	ruleMedia("(max-width: 1024px)", ".rvs-kbd", display("none"))
	ruleMedia("(max-width: 560px)", ".rvs-foot-acts",
		marginLeft("0"),
		width("100%"),
	)
	// Every cell placed EXPLICITLY. Auto-placement put the payee in the right-hand
	// column and pushed the amount onto its own line — the row read as two
	// unrelated fragments.
	ruleMedia("(max-width: 560px)", ".rvs-mem",
		gridTemplateColumns("1fr auto"),
		gridTemplateRows("auto auto"),
		rowGap("0.05rem"),
		prop("column-gap", "0.6rem"),
	)
	ruleMedia("(max-width: 560px)", ".rvs-mem-desc",
		gridColumn("1"),
		gridRow("1"),
		color("var(--text)"),
	)
	ruleMedia("(max-width: 560px)", ".rvs-mem-date",
		gridColumn("1"),
		gridRow("2"),
	)
	ruleMedia("(max-width: 560px)", ".rvs-mem-amt",
		gridColumn("2"),
		gridRow("1 / span 2"),
		alignSelf("center"),
		minWidth("0"),
	)
	// The footer stacked into a right-aligned column, so the summary read as if it
	// belonged to the buttons. Left-align it and let the actions sit on their own row.
	ruleMedia("(max-width: 560px)", ".rvs-foot",
		flexDirection("column"),
		alignItems("stretch"),
		textAlign("left"),
		gap("0.4rem"),
	)
	ruleMedia("(max-width: 560px)", ".rvs-link-row",
		gridTemplateColumns("70px 80px 1fr"),
		gap("0.4rem"),
	)
}
