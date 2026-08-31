// SPDX-License-Identifier: MIT

package styles

// registerIncomeAllocation styles the /budgets income-allocation read that sits
// directly beneath the hero band: how much of your income you have budgeted.
//
// It deliberately borrows the existing .zbb-alloc bar, marker and legend rather
// than introducing a second visual language for the same idea — the only new
// chrome is the caption line above it, which is what turns two numbers into a
// stated relationship. Registered after the generated sheet so equal-specificity
// refinements win.
func registerIncomeAllocation() {
	// The block is separated from the band by a hairline rather than a gap: they
	// are two readings of one plan, not two unrelated cards.
	// The block is inset to the band's own gutter so the bar starts and ends
	// where the figures above it do — a full-bleed bar under a padded band reads
	// as a different container's content.
	rule(".budget-alloc",
		marginTop("-0.35rem"),
		marginBottom("1rem"),
		padding("0.85rem 1.5rem 0"),
		borderTop("1px solid var(--border)"),
	)
	// Money past the income tick is money that does not exist. It wears the same
	// stripe the spend band uses for an overage, so one visual language means
	// "past the line" everywhere on this page.
	rule(".zbb-alloc-seg.is-overflow",
		prop("background", "repeating-linear-gradient(45deg,"+
			" color-mix(in srgb, var(--danger) 55%, transparent) 0,"+
			" color-mix(in srgb, var(--danger) 55%, transparent) 6px,"+
			" color-mix(in srgb, var(--danger) 22%, transparent) 6px,"+
			" color-mix(in srgb, var(--danger) 22%, transparent) 12px)"),
	)
	// --- Zero-based: the spend rail, and the hero that replaced the loader band ---
	//
	// The rail lives INSIDE .zbb-alloc-wrap, so it inherits the bar's x-axis exactly.
	// That is the whole point: the income marker is absolutely positioned against the
	// same wrapper and already stretches to its full height, so adding the rail here
	// makes one tick cut through BOTH readings — where the plan crosses income, and
	// where spending has got to — with no second marker and no extra chrome.
	rule(".budget-zbb-rail",
		position("relative"),
		height("4px"),
		marginTop("0.4rem"),
		borderRadius("var(--radius-pill)"),
		overflow("hidden"),
		background("color-mix(in srgb, var(--text-faint) 26%, transparent)"),
	)
	// Weight, not hue. The segments above carry the meaning-bearing colours (accent,
	// savings, danger stripes); a third colour here would compete with them for a
	// quantity that is only ever "how far along". A dim neutral reads as progress.
	rule(".budget-zbb-rail-fill",
		height("100%"),
		borderRadius("var(--radius-pill)"),
		background("var(--text-dim)"),
		transition("width var(--wonder-dur) var(--wonder-ease-out)"),
	)
	rule(".budget-zbb-note",
		marginTop("0.4rem"),
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
	)
	// The hero that stands in for the loader band on zero-based. One number, named
	// by its state, with the label as an eyebrow above it — the band's three figures
	// became the bar and its caption, so what is left here is the answer alone.
	rule(".budget-zbb-hero",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		padding("0.1rem 1.5rem 0.9rem"),
	)
	rule(".budget-zbb-hero-label",
		display("inline-flex"),
		alignItems("center"),
		gap("0.35rem"),
		fontSize("var(--type-12)"),
		letterSpacing("0.06em"),
		textTransform("uppercase"),
		color("var(--text-dim)"),
	)
	rule(".budget-zbb-hero-value",
		fontSize("2rem"),
		lineHeight("1.05"),
		fontWeight("650"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	// The hero and the striped overflow zone are the same fact stated twice — once
	// as a number, once as a region. Sharing the danger tone is what lets the eye
	// connect them; independently-red is a coincidence the reader cannot rely on.
	rule(".budget-zbb-hero-value.neg", color("var(--danger)"))
	rule(".budget-zbb-hero-value.pos", color("var(--money-positive)"))
	// A surplus is unfinished work in this method, never success.
	rule(".budget-zbb-hero-value.warn", color("var(--warn)"))
	// Caption row: the eyebrow on the left, the change-income control on the right.
	rule(".budget-alloc-cap",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.2rem 0.5rem"),
	)
	// The reading line. Percentage, its denominator and the relationship sit
	// ADJACENT, in that order, so the number is never separated from what it is a
	// percentage OF.
	rule(".budget-alloc-line",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.15rem 0.45rem"),
		marginTop("0.15rem"),
	)
	rule(".budget-alloc-denom",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
	)
	// The separator is attached to the relation rather than being its own element:
	// a standalone middot orphans onto its own line when the caption wraps at
	// narrow widths, which reads as a rendering fault.
	rule(".budget-alloc-relation::before",
		prop("content", `"· "`),
		color("var(--text-faint)"),
	)
	rule(".budget-alloc-cap-label",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	// The percentage is the figure the eye should land on, so it carries the
	// weight the caption's other parts deliberately do not.
	rule(".budget-alloc-pct",
		fontSize("1.05rem"),
		fontWeight("800"),
		lineHeight("1"),
		color("var(--text)"),
	)
	rule(".budget-alloc-relation",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
	)
	// Over income is the one state worth a tone. It is a statement of fact, not
	// an alarm — the bar's marker already carries the visual signal.
	rule(".budget-alloc-relation.is-over",
		color("var(--money-negative)"),
		fontWeight("600"),
	)
	// The allocation bar is deliberately SLIMMER than the spend band above it.
	// They are both horizontal bars but they have different denominators — spent-
	// of-budget versus budgeted-of-income — and identical styling invited the
	// reader to apply one reading to the other.
	rule(".budget-alloc-bar",
		height("10px"),
	)
	// The caveat that travels with the hero figure when the plan runs past income.
	rule(".budget-loader-caveat",
		marginTop("0.15rem"),
		fontSize("var(--type-12)"),
		fontWeight("600"),
		color("var(--money-negative)"),
		textAlign("right"),
	)
	// A quiet text control, not a button: changing the basis is a settings act,
	// and it must not compete with the figures beside it.
	rule(".budget-alloc-change",
		fontSize("var(--type-12)"),
		fontWeight("600"),
		color("var(--accent)"),
		background("none"),
		border("0"),
		padding("0"),
		cursor("pointer"),
		prop("text-decoration", "underline"),
		prop("text-underline-offset", "2px"),
	)
	rule(".budget-alloc-change:hover", color("var(--text)"))
	// At narrow widths the caption stacks; the income figure and its control keep
	// their own line so the percentage and the relationship stay adjacent.
	ruleMedia("(max-width: 640px)", ".budget-alloc-cap > span:last-child",
		prop("margin-left", "0"),
		prop("width", "100%"),
	)
}

// registerBudgetCatOptIn styles the add-a-budget category row: the picker for
// what the budget watches, and the opt-in that replaces it with a new category.
func registerBudgetCatOptIn() {
	rule(".budget-cat-row",
		display("grid"),
		gap("0.5rem"),
	)
	// The opt-in reads as a quiet alternative under the picker, not as a second
	// field of equal weight.
	rule(".budget-cat-optin",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
		cursor("pointer"),
	)
	rule(".budget-cat-optin input[disabled]",
		cursor("not-allowed"),
		opacity("0.6"),
	)
	rule(".budget-cat-optin:hover", color("var(--text)"))
}

// registerStickyFormError pins a form's validation message to the foot of its
// scrolling body.
//
// The transaction edit form rendered its error at the very bottom of a scrolling
// modal, below the fold, so pressing Save appeared to do nothing at all — the
// message existed but the person who needed it never saw it. Sticky positioning
// keeps it adjacent to the action that produced it at any scroll position, and
// needs no effect to do it (UseEffect does not run for a component passed as a
// FlipPanel prop, so a scrollIntoView would silently never fire).
func registerStickyFormError() {
	rule(".form-err-sticky",
		position("sticky"),
		bottom("0"),
		zIndex("2"),
		marginTop("0.5rem"),
		padding("0.5rem 0.65rem"),
		borderRadius("var(--radius)"),
		border("1px solid color-mix(in srgb, var(--danger) 45%, var(--border))"),
		background("color-mix(in srgb, var(--danger) 14%, var(--bg-card))"),
	)
	// The message inside loses its own margin so the pinned box is the only chrome.
	rule(".form-err-sticky .err",
		margin("0"),
	)
}

// registerSmartCatPolish restyles the Smart categorization modal (C522).
//
// Two things were wrong with it. Its segmented control marked the active tab
// with a thin outline that read as a focus ring rather than a selection, so at a
// glance no tab looked chosen. And its "you need an inference provider" state
// was a flat grey paragraph naming an action — add a key in Settings — with no
// way to take it, which is the dead end the review scan strip already had fixed.
func registerSmartCatPolish() {
	// The active segment is FILLED, not merely outlined. An outline on a dark
	// surface is the same visual language as focus, and the two competed.
	rule(".seg-btn.active",
		background("color-mix(in srgb, var(--accent) 22%, transparent)"),
		borderColor("var(--accent)"),
		color("var(--text)"),
		fontWeight("700"),
	)
	// The no-provider state is a callout with its action inside it, so the thing
	// to do sits with the reason to do it.
	rule(".smartcat-noprovider",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.6rem"),
		marginTop("0.6rem"),
		padding("0.7rem 0.8rem"),
		borderRadius("var(--radius)"),
		border("1px solid color-mix(in srgb, var(--accent) 34%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 8%, var(--bg-elev))"),
	)
	rule(".smartcat-noprovider-text",
		margin("0"),
		flex("1 1 18rem"),
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
	)
}

// registerTrackUntracked styles the "Track untracked spending" sheet: the bulk,
// per-row half of the unbudgeted strip.
//
// The rows are a grid rather than a flex row so the amount inputs and destination
// pickers form true columns down the sheet — a per-row flex layout lets each row
// size its own controls, and a column of controls that do not line up reads as a
// list of unrelated forms rather than one decision repeated.
func registerTrackUntracked() {
	// The strip's bulk entry point: a quiet text control beside the heading, so it
	// does not compete with the per-category chips it supplements.
	rule(".budget-unbudgeted-all",
		marginLeft("auto"),
		background("none"),
		border("0"),
		padding("0"),
		cursor("pointer"),
		font("inherit"),
		fontSize("var(--type-12)"),
		fontWeight("600"),
		color("var(--accent)"),
		prop("text-decoration", "underline"),
		prop("text-underline-offset", "2px"),
	)
	rule(".budget-unbudgeted-all:hover", color("var(--text)"))
	rule(".track-sheet",
		display("flex"),
		flexDirection("column"),
		minHeight("0"),
		height("100%"),
		padding("0.9rem 1.15rem 0"),
		gap("0.6rem"),
	)
	// The rows scroll; the footer does not. The footer carries the consequence of
	// the whole selection, so it must stay visible while the selection is edited.
	rule(".track-rows",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		overflowY("auto"),
		minHeight("0"),
		flex("1 1 auto"),
	)
	rule(".track-row",
		display("grid"),
		gridTemplateColumns("minmax(0, 1fr) auto"),
		gap("0.15rem 0.75rem"),
		alignItems("center"),
		padding("0.55rem 0.7rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("var(--bg-card)"),
	)
	// An excluded row stays legible — it is still information about your spending —
	// but stops reading as part of the plan being applied.
	rule(".track-row.is-off", opacity("0.55"))
	rule(".track-row-pick",
		display("inline-flex"),
		alignItems("center"),
		gap("0.45rem"),
		cursor("pointer"),
		minWidth("0"),
	)
	rule(".track-row-name",
		fontWeight("600"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".track-row-sub",
		gridColumn("1"),
		fontSize("var(--type-12)"),
		color("var(--text-faint)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".track-row-controls",
		gridColumn("2"),
		gridRow("1 / span 2"),
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.4rem"),
		justifyContent("flex-end"),
	)
	rule(".track-row-amt",
		width("7rem"),
		textAlign("right"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".track-row-dest", maxWidth("11rem"))
	rule(".track-raise",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
		cursor("pointer"),
		whiteSpace("nowrap"),
	)
	rule(".track-foot",
		display("flex"),
		alignItems("flex-end"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.5rem 1rem"),
	)
	rule(".track-foot-read",
		display("flex"),
		flexDirection("column"),
		gap("0.15rem"),
		minWidth("0"),
	)
	rule(".track-foot-line",
		margin("0"),
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
	)
	// The overspend warning is the one line here that describes damage, so it is
	// the only one that takes a tone.
	rule(".track-foot-risk",
		margin("0"),
		fontSize("var(--type-13)"),
		fontWeight("600"),
		color("var(--danger)"),
	)
	rule(".track-foot-actions",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		marginLeft("auto"),
	)
	// Below the two-column threshold each row stacks its controls under its name
	// rather than squeezing a text input, a select and a checkbox onto one line.
	ruleMedia("(max-width: 640px)", ".track-row",
		gridTemplateColumns("minmax(0, 1fr)"),
	)
	ruleMedia("(max-width: 640px)", ".track-row-controls",
		gridColumn("1"),
		gridRow("auto"),
		justifyContent("flex-start"),
	)
}

// registerGoalGlyph styles the compact row's goal-funded marker: the pill while
// its words fit, a glyph with a hover-intent spinner once they do not.
func registerGoalGlyph() {
	// Wide rows keep the words. The glyph is mounted but not shown, so switching
	// between them on a resize costs no re-render.
	//
	// Scoped to the row rather than written as a bare `.budget-goalglyph-wrap`:
	// the wrapper also carries `.add-wrap` and a tw.Fold utility class, both of
	// which set display at the SAME single-class specificity, and the utility won
	// on order — so a bare rule left the glyph visible at every width, beside the
	// pill it was meant to replace (Cam, 2026-08-31). The descendant selector puts
	// it a level above all of them.
	rule(".budget-crow-head .budget-marker-wrap .budget-goalglyph", display("none"))
	// …except where the glyph IS the marker. Hiding it above the threshold left
	// goal-funded rows with no marker at all on a wide screen (Cam, 2026-08-31).
	rule(".budget-crow-head .budget-marker-wrap.is-glyphonly .budget-goalglyph", display("inline-flex"))
	// Below the row's two-column threshold — the same width at which it drops the
	// "left" column — the pill's 148px cannot be afforded and the symbol takes over.
	// One switch for BOTH markers now that pill and glyph live in one component:
	// below the threshold the words go and the symbol arrives, together.
	ruleContentMax(900, ".budget-crow-head .budget-marker-wrap .budget-goalglyph", display("inline-flex"))
	ruleContentMax(900, ".budget-crow-head .budget-marker-wrap .budget-marker-pill", display("none"))
	// SPACING. The head's 0.4rem gap was tuned for worded pills, which carry their
	// own padding and read as separated blocks. Glyphs are bare 22px squares, and
	// at that gap a name sat almost against its markers while two markers on one
	// row drifted apart from each other (Cam, 2026-08-31). Tighten the gap so the
	// glyphs group, and give the name a little air on its right instead — the
	// separation the reader needs is name-from-markers, not marker-from-marker.
	// Below the glyph threshold the note marker is an icon too, so it sizes to the
	// icon rather than flexing into space that no longer exists. Its prose goes at
	// the SAME width the pills do, so the whole head switches to symbols at once
	// instead of in two stages.
	ruleContentMax(900, ".budget-crow-notes", flex("0 0 auto"))
	ruleContentMax(900, ".budget-crow-notes-text", display("none"))
	ruleContentMax(900, ".budget-crow-head", gap("0.15rem"))
	ruleContentMax(900, ".budget-crow-head .budget-crow-name", prop("margin-right", "0.3rem"))
	// The badge wrapper adds its own box around the pill; with the pill gone it
	// would still reserve space, leaving a gap where a marker used to be.
	ruleContentMax(900, ".budget-crow-head .budget-rollover-badge",
		display("inline-flex"), alignItems("center"), padding("0"), border("0"), background("none"))
	rule(".budget-goalglyph",
		position("relative"),
		display("inline-flex"),
		alignItems("center"),
		justifyContent("center"),
		width("1.35rem"),
		height("1.35rem"),
		padding("0"),
		border("0"),
		borderRadius("var(--radius-pill)"),
		background("none"),
		color("var(--text-faint)"),
		cursor("pointer"),
		transition("color var(--motion-fast) var(--ease-standard), background var(--motion-fast) var(--ease-standard)"),
	)
	rule(".budget-goalglyph:hover, .budget-goalglyph.is-open",
		color("var(--accent)"),
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".budget-goalglyph:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	// The wait made visible. It rides ON the glyph rather than replacing it, so the
	// icon never jumps and the ring reads as "this is counting", not "this reloaded".
	rule(".budget-goalglyph-spin",
		position("absolute"),
		prop("inset", "-2px"),
		borderRadius("var(--radius-pill)"),
		border("1.5px solid transparent"),
		borderTopColor("var(--accent)"),
		prop("animation", "budget-goalglyph-spin 0.62s linear infinite"),
		pointerEvents("none"),
	)
	rawBlock(`@keyframes budget-goalglyph-spin{to{transform:rotate(360deg)}}`)
	// A spinner is motion for its own sake to anyone who has asked for less of it;
	// the delay still runs, so the popover behaves identically — it just stops
	// advertising the wait.
	rawBlock(`@media (prefers-reduced-motion: reduce){.budget-goalglyph-spin{animation:none;border-top-color:transparent;background:color-mix(in srgb, var(--accent) 18%, transparent)}}`)
}

// registerNotesMarkerWrap styles the note marker's new wrapper. It became a
// component (to share the row's one hover system) and so gained a Span around the
// button; without this the wrapper, not the button, would be the flex child the
// head sizes — and the careful 0-basis/auto-basis split would apply to the wrong
// element.
func registerNotesMarkerWrap() {
	rule(".budget-crow-notes-wrap",
		display("inline-flex"),
		alignItems("center"),
		flex("1 1 0"),
		minWidth("0"),
		overflow("hidden"),
	)
	ruleContentMax(900, ".budget-crow-notes-wrap", flex("0 0 auto"))
	// The button fills whatever the wrapper was given, so the text ellipsizes
	// against the wrapper's width rather than the button's intrinsic one.
	rule(".budget-crow-notes-wrap .budget-crow-notes",
		flex("1 1 auto"),
		minWidth("0"),
	)
	// The spinner needs a positioned ancestor; the note button is not round like
	// the glyph, so its ring follows the button's own shape.
	rule(".budget-crow-notes", position("relative"), borderRadius("var(--radius-md)"))
}

// registerMarkerWrap styles the shared marker wrapper. The pill and the glyph are
// siblings inside it and exactly one is displayed at a given width, so the wrapper
// must not add a box of its own — it exists only to give the popover a single
// anchor for both shapes.
func registerMarkerWrap() {
	rule(".budget-marker-wrap",
		display("inline-flex"),
		alignItems("center"),
		minWidth("0"),
		// A marker that cannot fit gives ground rather than spilling; the glyph it
		// switches to below the threshold is the real answer at narrow widths.
		flexShrink("1"),
		overflow("hidden"),
	)
	// The pill keeps whatever tone class its caller passed (the rollover badge's
	// on/off/capped colours, the goal chip's dim pill); this only makes it a button
	// rather than the span it used to be.
	rule(".budget-marker-pill",
		font("inherit"),
		cursor("pointer"),
		border("0"),
	)
	rule(".budget-marker-pill:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
}

// registerCoverMarker tones the coverage marker apart from its neighbours. Goal
// funding and rollover are properties the budget has on its own; coverage means
// ANOTHER budget is paying part of this one, which is a different kind of fact and
// the one most likely to explain a row that looks healthier than it is.
func registerCoverMarker() {
	rule(".budget-crow-chip.is-cover",
		background("color-mix(in srgb, var(--accent-savings, #8b7cf6) 16%, transparent)"),
		color("var(--accent-savings, #8b7cf6)"),
		borderColor("color-mix(in srgb, var(--accent-savings, #8b7cf6) 40%, var(--border))"),
	)
	rule(".budget-marker-wrap [data-testid^='budget-glyph-cover-']",
		color("var(--accent-savings, #8b7cf6)"),
	)
}
