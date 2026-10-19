// SPDX-License-Identifier: MIT

package styles

// registerBR3Row gives the budget envelope bar a dedicated grid track and keeps
// supporting copy quiet beneath the name at desktop and phone widths.
func registerBR3Row() {
	rule(".budget-crow",
		prop("grid-template-areas", `"name figures bar status actions" "sub sub sub sub sub"`),
		prop("align-items", "center"),
	)
	rule(".budget-crow-head", prop("grid-area", "name"), minWidth("0"))
	rule(".budget-crow-name", minWidth("0"), overflow("hidden"), textOverflow("ellipsis"), whiteSpace("nowrap"))
	rule(".budget-crow-chip", prop("grid-area", "status"), justifySelf("start"), textAlign("left"))
	rule(".budget-crow-status-text", prop("grid-area", "status"), minWidth("0"), fontSize("var(--type-13)"), color("var(--text-dim)"), whiteSpace("nowrap"))
	rule(".budget-crow-actions", prop("grid-area", "actions"), justifySelf("end"), minWidth("72px"))
	rule(".budget-crow-figures",
		prop("grid-area", "figures"), display("flex"), justifyContent("flex-end"),
		minWidth("0"), whiteSpace("nowrap"), fontVariantNumeric("tabular-nums"),
	)
	rule(".budget-crow-amt", whiteSpace("nowrap"), fontVariantNumeric("tabular-nums"))
	rule(".budget-crow-bar",
		prop("grid-area", "bar"), position("relative"), display("block"),
		width("100%"), minWidth("180px"), height("6px"), borderRadius("3px"),
		background("var(--bg-elev)"), overflow("visible"), alignSelf("center"),
	)
	rule(".budget-crow-bar .bar-fill", height("6px"), borderRadius("3px"), maxWidth("100%"))
	rule(".budget-crow-bar .bar-pace-tick", zIndex("1"), height("12px"), top("-3px"), width("2px"), background("var(--text-1, var(--text))"))
	rule(".budget-crow-sub", prop("grid-area", "sub"), minWidth("0"), fontSize("var(--fs-meta)"), lineHeight("1.35"), color("var(--text-dim)"))
	rule(".budget-crow-sub:empty", display("none"))
	rule(".budget-crow-rollover", prop("grid-column", "1 / -1"), minWidth("0"))
	rule(".budget-rollover-detail[open]", prop("grid-column", "1 / -1"), width("100%"))
	rule(".budget-rollover-detail table", width("100%"), fontVariantNumeric("tabular-nums"))
	rule(".budget-rollover-detail th, .budget-rollover-detail td", whiteSpace("nowrap"), textAlign("right"))
	rule(".budget-rollover-detail th:first-child, .budget-rollover-detail td:first-child", textAlign("left"))
	rule(".budget-limit-editor", prop("grid-column", "1 / -1"), width("100%"), minWidth("0"))
	rule(".budget-limit-editor-cap", fontVariantNumeric("tabular-nums"))
	rule(".budget-crow-chip", minHeight("24px"))
	rule(".budget-crow-actions button, .budget-crow-actions a", minWidth("24px"), minHeight("24px"))
	rule(".budget-owner-initial", display("inline-flex"), alignItems("center"), justifyContent("center"), flex("0 0 24px"), width("24px"), height("24px"), borderRadius("50%"), background("var(--bg-elev)"), color("var(--text-dim)"), fontSize("var(--fs-meta)"), fontWeight("600"))
	rule(".budget-cadence", color("var(--text-faint)"), fontSize("12px"), marginLeft("0.2rem"), whiteSpace("nowrap"))
	rule(".budget-crow:has(.budget-rollover-detail[open]) .budget-rollover-carry", display("none"))

	ruleMedia("(min-width: 601px) and (max-width: 1100px)", ".budget-crow",
		gridTemplateColumns("minmax(220px, 1fr) 240px minmax(180px, 1fr) 110px 72px"),
	)
	ruleMedia("(min-width: 601px) and (max-width: 1100px)", ".budget-crow-figures", minWidth("240px"), whiteSpace("nowrap"), textAlign("right"))
	ruleMedia("(min-width: 601px) and (max-width: 1100px)", ".budget-crow-bar", height("6px"), minWidth("180px"))
	ruleMedia("(max-width: 600px)", ".budget-crow",
		gridTemplateColumns("minmax(0, 1fr) auto 72px"),
		prop("grid-template-areas", `"name name status" "figures figures actions" "bar bar bar" "sub sub sub"`),
		gap("0.25rem 0.5rem"), minWidth("0"),
	)
	ruleMedia("(max-width: 600px)", ".budget-crow-head", minWidth("0"), overflow("hidden"))
	ruleMedia("(max-width: 600px)", ".budget-crow-name", minWidth("0"), maxWidth("100%"), overflow("hidden"), textOverflow("ellipsis"), whiteSpace("nowrap"))
	ruleMedia("(max-width: 600px)", ".budget-crow-figures", gridColumn("1 / 3"), minWidth("0"), justifyContent("flex-start"), whiteSpace("nowrap"), fontSize("var(--type-12)"))
	ruleMedia("(max-width: 600px)", ".budget-crow-actions", gridColumn("3"), minWidth("72px"), justifySelf("end"))
	ruleMedia("(max-width: 600px)", ".budget-crow-bar", width("100%"), minWidth("0"), height("6px"))
	ruleMedia("(max-width: 600px)", ".budget-crow-rollover", gridColumn("1 / -1"))
}
