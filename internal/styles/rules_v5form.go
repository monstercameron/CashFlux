// SPDX-License-Identifier: MIT

package styles

func registerV5Form() {
	rule(".split-row, .split-column-labels",
		prop("grid-template-columns", "minmax(180px, 1fr) 120px 32px"),
	)
	rule(".split-row.has-owner, .split-column-labels.has-owner",
		prop("grid-template-columns", "minmax(180px, 1fr) 160px 120px 32px"),
	)
	rule(".split-category-control",
		minWidth("0"),
	)
	rule(".split-category-control .field",
		width("100%"),
		minWidth("180px"),
	)
	rule(".split-amount-control",
		display("flex"),
		alignItems("center"),
		gap("0.35rem"),
		minWidth("0"),
	)
	rule(".split-amount-control .field",
		width("100%"),
		minWidth("0"),
	)
	rule(".hero-stat-sub.split-remainder-neutral",
		color("var(--muted)"),
	)
}
