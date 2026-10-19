// SPDX-License-Identifier: MIT

package i18n

// r5nKeys keeps the search-widen action concise and separate from the count sentence.
var r5nKeys = Catalog{
	"r5n.searchAllDates": "Search all dates (%s) →",
}

func init() {
	for key, value := range r5nKeys {
		english[key] = value
	}
}
