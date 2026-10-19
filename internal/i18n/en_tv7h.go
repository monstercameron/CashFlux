// SPDX-License-Identifier: MIT

package i18n

var tv7hKeys = Catalog{
	"tv7h.themeLabel": "Theme: %s",
}

func init() {
	for key, value := range tv7hKeys {
		english[key] = value
	}
}
