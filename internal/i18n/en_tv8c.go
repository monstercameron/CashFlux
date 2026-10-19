// SPDX-License-Identifier: MIT

package i18n

var tv8cKeys = Catalog{
	"tv8c.categoryList": "All spending categories",
	"tv8c.closeList":    "Close list",
}

func init() {
	for key, value := range tv8cKeys {
		english[key] = value
	}
}
