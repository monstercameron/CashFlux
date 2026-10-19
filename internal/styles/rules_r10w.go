// SPDX-License-Identifier: MIT

package styles

// registerR10W styles the in-place category affordance and nested row-menu actions.
func registerR10W() {
	rule(".td-cat-uncategorized",
		color("var(--text-dim)"),
	)
	rule(".txn-cat-choose",
		padding("0 .25rem"),
		border("0"),
		background("transparent"),
		color("var(--text-dim)"),
		font("inherit"),
		lineHeight("inherit"),
		textAlign("left"),
		whiteSpace("nowrap"),
		cursor("pointer"),
	)
	rule(".txn-cat-choose:hover, .txn-cat-choose:focus-visible",
		color("var(--accent)"),
		textDecoration("underline"),
	)
	rule(".txn-catsuggest + .txn-cat-choose",
		marginLeft("-.2rem"),
		fontSize("var(--fs-meta)"),
	)
	rule(".add-submenu-wrap",
		position("relative"),
	)
	rule(".add-submenu",
		position("absolute"),
		left("calc(100% - 2px)"),
		top("-.35rem"),
		zIndex("calc(var(--z-dropdown) + 2)"),
		minWidth("220px"),
		maxWidth("min(320px, calc(100vw - 16px))"),
		maxHeight("calc(100vh - 16px)"),
		overflowY("auto"),
		padding(".35rem"),
		border("1px solid var(--border)"),
		borderRadius(".65rem"),
		background("var(--bg-card)"),
		boxShadow("0 8px 24px rgba(0,0,0,.18)"),
	)
	rule(".add-menu.open-left .add-submenu",
		left("auto"),
		right("calc(100% - 2px)"),
	)
	rule(".add-submenu .add-item",
		width("100%"),
		whiteSpace("normal"),
	)
}
