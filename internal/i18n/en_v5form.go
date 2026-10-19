// SPDX-License-Identifier: MIT

package i18n

var v5formKeys = Catalog{
	"common.saving":           "Saving…",
	"v5form.splitNewCategory": "New category…",
}

func init() {
	for key, value := range v5formKeys {
		english[key] = value
	}
}
