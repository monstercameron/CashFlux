// SPDX-License-Identifier: MIT

package styles

// registerNotesMarkerWrap styles the budget note control and its wrapper.
func registerNotesMarkerWrap() {
	rule(".budget-crow-notes-wrap",
		display("inline-flex"),
		alignItems("center"),
		flex("1 1 0"),
		minWidth("0"),
		overflow("hidden"),
	)
	ruleContentMax(900, ".budget-crow-notes-wrap", flex("0 0 auto"))
	rule(".budget-crow-notes-wrap .budget-crow-notes", flex("1 1 auto"), minWidth("0"))
	rule(".budget-crow-notes", position("relative"), borderRadius("var(--radius-md)"))

	rule(".budget-crow-notes",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		flex("1 1 0"),
		overflow("hidden"),
		minWidth("0"),
		padding("0"),
		border("0"),
		background("none"),
		lineHeight("1"),
		cursor("pointer"),
		color("var(--muted)"),
	)
	rule(".budget-crow-notes .budget-crow-notes-text",
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		fontSize("var(--fs-meta)"),
	)
	rule(".budget-crow-notes:hover, .budget-crow-notes:focus-visible", color("var(--text)"))
	ruleContentMax(860-railCollapsedPx, ".budget-crow-notes-text", display("none"))
	ruleContentMax(900, ".budget-crow-notes", flex("0 0 auto"))
	ruleContentMax(900, ".budget-crow-notes-text", display("none"))
}

// registerBV6A repairs budget-row sizing and typography at desktop, laptop, and phone widths.
func registerBV6A() {
	// Desktop keeps the amount intact while allowing the bar to take the remaining card width.
	rule(".budget-crow",
		gridTemplateColumns("minmax(180px, 1fr) max-content minmax(160px, 1.2fr) 96px 72px"),
		prop("grid-template-areas", `"name figures bar status actions" "sub sub sub sub sub"`),
		minWidth("0"),
	)
	rule(".budget-crow-head", minWidth("0"), overflow("hidden"))
	rule(".budget-crow-name", fontSize("var(--fs-body)"), fontWeight("600"))
	rule(".budget-crow-figures",
		minWidth("0"),
		maxWidth("100%"),
		justifyContent("flex-end"),
		flexWrap("wrap"),
		prop("column-gap", "0.2rem"),
		whiteSpace("normal"),
		fontSize("var(--fs-body)"),
	)
	rule(".budget-crow-amt", fontSize("var(--fs-body)"), fontWeight("500"), fontVariantNumeric("tabular-nums"))
	rule(".budget-crow-left", fontSize("var(--fs-body)"), fontWeight("400"), fontVariantNumeric("tabular-nums"))
	rule(".budget-crow-fig-separator", display("none"))
	rule(".budget-crow-status-text", fontSize("var(--fs-meta)"), fontWeight("400"), minWidth("0"), overflow("hidden"), textOverflow("ellipsis"))
	rule(".budget-crow-actions button, .budget-crow-actions a", minWidth("32px"), minHeight("32px"))
	rule(".budget-crow button, .budget-crow a", minHeight("32px"))
	rule(".budget-crow-notes", minWidth("32px"), minHeight("32px"))
	rule(".budget-crow-bar", minWidth("0"), height("6px"), background("color-mix(in srgb, var(--text) 22%, var(--bg-card))"))
	rule(".budget-crow-bar .bar-pace-tick", height("12px"), top("-3px"), width("2px"))
	rule(".budget-crow-sub", fontSize("var(--fs-meta)"), fontWeight("400"))
	rule(".budget-cadence", fontSize("var(--fs-meta)"), fontWeight("400"))

	// Keep the rollover math near its row instead of spreading columns across the card.
	rule(".budget-rollover-detail", maxWidth("560px"), minWidth("0"))
	rule(".budget-rollover-detail table",
		width("auto"),
		maxWidth("100%"),
		marginLeft("0"),
		fontSize("var(--fs-meta)"),
	)

	// A single aligned line keeps the title, effective cap, input label, and controls together.
	rule(".budget-limit-editor",
		display("grid"),
		gridTemplateColumns("minmax(0, 1fr) auto minmax(180px, 240px) auto auto"),
		alignItems("center"),
		gap("0.5rem 0.75rem"),
	)
	rule(".budget-limit-editor-title", minWidth("0"), fontSize("var(--fs-body)"), fontWeight("600"))
	rule(".budget-limit-editor-cap", fontSize("var(--fs-meta)"), fontWeight("400"), fontVariantNumeric("tabular-nums"))
	rule(".budget-limit-editor-base", display("flex"), alignItems("center"), gap("0.5rem"), minWidth("0"))
	rule(".budget-limit-editor-base label", fontSize("var(--fs-meta)"), fontWeight("400"), whiteSpace("nowrap"))
	rule(".budget-limit-editor button", minWidth("32px"), minHeight("32px"))
	rule(".budget-limit-editor > [role=alert]", gridColumn("1 / -1"))

	ruleMedia("(min-width: 601px) and (max-width: 1200px)", ".budget-crow",
		gridTemplateColumns("minmax(0, 1fr) minmax(0, max-content) 72px"),
		prop("grid-template-areas", `"name figures actions" "bar bar status" "sub sub sub"`),
		gap("0.35rem 0.7rem"),
	)
	ruleMedia("(min-width: 601px) and (max-width: 1200px)", ".budget-crow-figures",
		justifyContent("flex-start"),
		maxWidth("100%"),
	)
	ruleMedia("(min-width: 601px) and (max-width: 1200px)", ".budget-crow-bar",
		gridColumn("1 / 3"),
		width("100%"),
	)
	ruleMedia("(min-width: 601px) and (max-width: 1200px)", ".budget-crow-status-text",
		gridColumn("3"),
		gridRow("2"),
		justifySelf("end"),
		maxWidth("72px"),
	)
	// The amount and remaining balance may wrap as groups; the separator never becomes an orphan.
	ruleMedia("(min-width: 601px) and (max-width: 1200px)", ".budget-crow-figures .budget-crow-amt", whiteSpace("nowrap"))
	ruleMedia("(min-width: 601px) and (max-width: 1200px)", ".budget-crow-figures .budget-crow-left", whiteSpace("normal"))
	ruleMedia("(min-width: 601px) and (max-width: 1200px)", ".budget-crow-fig-separator", display("none"))

	// Phone rows use three short tracks and keep the figures and bar inside the card.
	ruleMedia("(max-width: 600px)", ".budget-crow",
		gridTemplateColumns("minmax(0, 1fr) max-content 72px"),
		prop("grid-template-areas", `"name status actions" "figures figures figures" "bar bar bar" "sub sub sub"`),
		gap("0.25rem 0.5rem"),
		minWidth("0"),
	)
	ruleMedia("(max-width: 600px)", ".budget-crow-head", minWidth("0"), overflow("hidden"))
	ruleMedia("(max-width: 600px)", ".budget-crow-figures",
		gridColumn("1 / -1"),
		minWidth("0"),
		maxWidth("100%"),
		justifyContent("flex-start"),
		flexWrap("wrap"),
	)
	ruleMedia("(max-width: 600px)", ".budget-crow-bar", gridColumn("1 / -1"), width("100%"), minWidth("0"))
	ruleMedia("(max-width: 600px)", ".budget-crow-actions", gridColumn("3"), minWidth("72px"), justifySelf("end"))
	ruleMedia("(max-width: 600px)", ".budget-crow-chip, .budget-crow-status-text", maxWidth("100%"), minWidth("0"), overflow("hidden"), textOverflow("ellipsis"))
	ruleMedia("(max-width: 600px)", ".budget-limit-editor",
		gridTemplateColumns("minmax(0, 1fr) auto"),
	)
	ruleMedia("(max-width: 600px)", ".budget-limit-editor-title", gridColumn("1 / -1"))
	ruleMedia("(max-width: 600px)", ".budget-limit-editor-cap", gridColumn("1 / -1"))
	ruleMedia("(max-width: 600px)", ".budget-limit-editor-base", gridColumn("1 / -1"), flexWrap("wrap"))
	ruleMedia("(max-width: 600px)", ".budget-limit-editor button", minHeight("32px"))
}
