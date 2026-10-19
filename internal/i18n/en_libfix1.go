// SPDX-License-Identifier: MIT

package i18n

// libfix1Keys contains the picker labels introduced by LIBFIX1.
var libfix1Keys = Catalog{
	"picker.search":     "Search %s",
	"picker.createAria": "Create %s",
	"picker.create":     "Create '%s'",
}

func init() {
	for key, value := range libfix1Keys {
		english[key] = value
	}
}
