// SPDX-License-Identifier: MIT

package i18n

// bc5b4Keys names the selected dashboard window on the flex-budgeting surface.
var bc5b4Keys = Catalog{
	"flex.windowLabel": "Viewing %s",
}

func init() {
	for key, value := range bc5b4Keys {
		english[key] = value
	}
}
