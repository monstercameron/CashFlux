// SPDX-License-Identifier: MIT

package styles

// registerR7Z keeps the transaction bulk bar's viewport layer visually empty
// and lets an open row menu paint above its neighbouring rows.
func registerR7Z() {
	// The widget remains a full-viewport pointer-events layer, but its .w surface
	// styling must not turn that layer into an opaque tile. The hover selector
	// also defeats the shared tile lift and shadow rules, which use !important.
	rule(`html .w[data-widget="txn-bulkbar"], html .w[data-widget="txn-bulkbar"]:hover`,
		prop("background", "transparent !important"),
		prop("border", "0 !important"),
		prop("border-radius", "0 !important"),
		prop("box-shadow", "none !important"),
		prop("outline", "none !important"),
		prop("padding", "0 !important"),
		prop("backdrop-filter", "none !important"),
		prop("transform", "none !important"),
		prop("will-change", "auto !important"),
		prop("transition", "none !important"),
		prop("animation", "none !important"),
		prop("overflow", "visible !important"),
	)
	rule(`html .w[data-widget="txn-bulkbar"]::before, html .w[data-widget="txn-bulkbar"]::after`,
		prop("content", "none !important"),
		display("none !important"),
	)

	// The editable-description popover rule raises its cell to z-index 51. Put the
	// active action row above that cell and its siblings whether this menu opens
	// up or down. The action cell must also escape the ledger's cell clipping.
	rule(".txn-table tbody tr.row:has(.overflow-menu-open)",
		position("relative"),
		zIndex("var(--z-overlay)"),
	)
	rule(".txn-table tbody tr.row:has(.overflow-menu-open) > td.td-actions",
		position("relative"),
		zIndex("var(--z-row-active)"),
		overflow("visible"),
	)
}
