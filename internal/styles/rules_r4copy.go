// SPDX-License-Identifier: MIT

package styles

// registerR4Copy keeps explanatory prose on the Transactions surfaces easy to scan.
func registerR4Copy() {
	rule(".rvs-note, .filter-inline-panel.filter-sheet p",
		maxWidth("68ch"),
	)
}
