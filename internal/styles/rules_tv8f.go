// SPDX-License-Identifier: MIT

package styles

// registerTV8F brings the Transactions dialog surfaces onto one type scale and
// gives Quick-Add the same desktop field width as the edit panel.
func registerTV8F() {
	rule(".cf-overlay__surface--modal, .cf-overlay__surface--sheet, .flip-back, .rvs",
		fontFamily("Inter, sans-serif"),
	)
	rule(".cf-overlay__surface--modal, .cf-overlay__surface--sheet, .flip-back",
		fontSize("var(--fs-body)"),
		fontWeight("400"),
	)
	rule(".cf-overlay__title, .flip-back .set-h h3, .rvs .rvs-intro-title, .cf-dialog-title",
		fontFamily("Inter, sans-serif"),
		fontSize("var(--fs-figure)"),
		fontWeight("600"),
		textAlign("left"),
	)
	rule(".flip-back .set-h",
		justifyContent("space-between"),
	)
	rule(".flip-back .flip-title-balance",
		display("none"),
	)
	rule(".cf-overlay__body",
		padding("24px"),
	)
	rule(".flip-back .set-body:not(.set-body-flush)",
		padding("24px"),
	)
	rule(".cf-overlay__footer",
		padding("16px 24px"),
	)
	rule(".cf-overlay__footer .btn-del, .flip-back .set-foot .btn-del",
		marginRight("auto"),
	)
	rule(".filter-inline-title",
		fontFamily("Inter, sans-serif"),
		fontSize("var(--fs-figure)"),
		fontWeight("600"),
		textAlign("left"),
	)
	rule(".filter-inline-panel.filter-sheet .filter-inline-body",
		padding("24px"),
	)
	rule(".filter-inline-panel.filter-sheet .filter-sheet-clear",
		marginRight("auto"),
	)
	rule(".cf-overlay__footer button, .flip-back .set-btn",
		fontSize("var(--fs-body)"),
		fontWeight("500"),
	)
	rule(".rvs [class*='rvs-'], .rvs [class*='rvw-']",
		fontSize("var(--fs-body)"),
		fontWeight("400"),
		fontStyle("normal"),
		textTransform("none"),
	)
	rule(".rvs .rvs-big",
		fontSize("var(--fs-title)"),
		fontWeight("600"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rvs .rvs-tier-count, .rvs .rvs-foot-sub, .rvs .rvs-note, .rvs .rvs-kbd, .rvs .rvw-rawpayee",
		fontSize("var(--fs-meta)"),
		fontWeight("400"),
		fontStyle("normal"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".rvs .rvs-kbd",
		textTransform("none"),
	)
	rule(".rvs .rvs-intro-title",
		fontFamily("Inter, sans-serif"),
		fontSize("var(--fs-figure)"),
		fontWeight("600"),
	)
	rule(".cf-overlay__surface--modal:has(.txn-add-layout)",
		width("min(760px, 90vw)"),
		maxHeight("min(720px, 90vh)"),
	)
	rule(".txn-form-add .txt-picker:has(.txt-empty)",
		display("none"),
	)
}
