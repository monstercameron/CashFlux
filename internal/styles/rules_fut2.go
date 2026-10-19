// SPDX-License-Identifier: MIT

package styles

func registerFUT2() {
	rule(".txn-future-empty-line",
		prop("margin", "0.5rem 0"),
		prop("font-style", "normal"),
		prop("color", "var(--text)"),
	)
	rule(".txn-scheduled-ghost",
		prop("display", "grid"),
		prop("grid-template-columns", "7rem minmax(12rem,1fr) 10rem 10rem 7rem 7rem"),
		prop("align-items", "center"),
		prop("gap", ".5rem"),
		prop("padding", ".45rem .6rem"),
		prop("border-bottom", "1px solid var(--border)"),
		prop("color", "var(--text)"),
		prop("font-style", "normal"),
		prop("opacity", "1"),
	)
	rule(".txn-scheduled-ghost .td-acct",
		prop("min-width", "0"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".txn-scheduled-name",
		prop("min-width", "0"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".txn-scheduled-pill",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("padding", "0.1rem 0.4rem"),
		prop("border", "1px dashed var(--border)"),
		prop("border-radius", "999px"),
		prop("color", "var(--text)"),
		prop("font-size", "var(--type-11)"),
	)
	rule(".txn-scheduled-menu",
		prop("position", "relative"),
		prop("display", "flex"),
		prop("justify-content", "center"),
	)
	rule(".txn-scheduled-menu-item",
		prop("position", "absolute"),
		prop("right", "0"),
		prop("top", "100%"),
		prop("z-index", "5"),
		prop("min-width", "13rem"),
		prop("padding", ".5rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-md)"),
		prop("background", "var(--bg-card)"),
	)
}
