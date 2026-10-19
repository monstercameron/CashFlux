// SPDX-License-Identifier: MIT

package i18n

func init() {
	// CAT1 replaces the inline text editor with CategoryPicker. Keep the compact
	// action label; remove strings that belonged only to the deleted editor.
	delete(english, "transactions.newCategoryName")
	delete(english, "transactions.newCategoryAdd")
	delete(english, "transactions.newCategoryCancel")
}
