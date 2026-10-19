// SPDX-License-Identifier: MIT

package styles

// registerBC6N7 keeps the editable bucket choice compact inside each template row.
func registerBC6N7() {
	rule(".budget-tmpl-row .budget-tmpl-bucket",
		width("auto"),
		minWidth("7.5rem"),
		flex("0 0 auto"),
	)
}
