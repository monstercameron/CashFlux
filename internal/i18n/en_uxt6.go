// SPDX-License-Identifier: MIT

package i18n

var uxt6Keys = Catalog{
	"reviewInbox.categoriesTitle": "Review categories",
	"reviewInbox.countLine":       "%d charges from %d merchants",
	// Keep the source filter label aligned with the other sentence-case nouns.
	"transactions.filterSource": "Source",
}

func init() {
	for key, value := range uxt6Keys {
		english[key] = value
	}
}
