// SPDX-License-Identifier: MIT

package styles

// registerBC5B9 holds the final budget rules whose declarations supersede
// older rules while the budget styles are migrated to single owners.
func registerBC5B9() {
	rule(".budget-crow", gridTemplateColumns("minmax(180px, 1fr) max-content minmax(160px, 1.2fr) 96px 72px"))
}
