// SPDX-License-Identifier: MIT

package styles

// registerCategoriesSurface emits the /categories taxonomy-ledger design: the
// auto-row bento host, the per-category figure column (this-period amount +
// category-tinted share bar), and the quiet deductible tag. Hero/section/
// takeaway chrome reuses the shared rpt-*/debt-* rules so the page reads as a
// sibling of the Understand surfaces. Registered from Register().
func registerCategoriesSurface() {
	rule(".bento.bento-cats",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-cats > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// A row with an OPEN ⋯ menu must paint above its later siblings (tiles are
	// transformed stacking contexts; the shared rule handles tile-vs-tile).
	rule(".bento.bento-cats > .w:has(.add-menu:not(.hidden-menu))",
		prop("z-index", "30"),
	)
	// The category's this-period figure column, right-aligned before the actions.
	rule(".cat-figure",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-end"),
		prop("gap", "0.1rem"),
		prop("min-width", "7.5rem"),
		prop("flex-shrink", "0"),
	)
	rule(".cat-figure .amount",
		prop("font-size", "0.95rem"),
	)
	rule(".cat-figure-sub",
		prop("font-size", "0.7rem"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("white-space", "nowrap"),
	)
	// The share bar rides inside .row-main under the name, like the reports rows;
	// cap its reach so a full bar doesn't crowd the figure column.
	rule(".bento-cats .row-main .share-bar",
		prop("max-width", "26rem"),
	)
	// Deductible tag: neutral metadata chrome (mirrors .rpt-new-tag).
	rule(".cat-tag",
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "999px"),
		prop("padding", "0 0.45rem"),
		prop("font-size", "0.68rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
		prop("color", "var(--text-dim)"),
		prop("white-space", "nowrap"),
	)
	// The map keeps its chip grammar but breathes a little more under the serif head.
	rule(".bento-cats .cat-map",
		prop("margin-top", "0.35rem"),
	)
}
