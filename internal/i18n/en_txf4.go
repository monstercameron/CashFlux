// SPDX-License-Identifier: MIT

package i18n

// txf4Keys names the unassigned-owner choice in the member perspective picker.
var txf4Keys = Catalog{
	"member.unassigned": "Unassigned",
}

func init() {
	for k, v := range txf4Keys {
		english[k] = v
	}
}
