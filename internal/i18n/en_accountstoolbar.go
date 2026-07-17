// SPDX-License-Identifier: MIT

package i18n

// accountsToolbarKeys are the strings for the /accounts consolidated "Manage"
// menu (2026-07-17 visual audit — too many equal-weight management buttons
// before the account list). Kept separate from en.go (concurrent WIP) like the
// other feature key files.
var accountsToolbarKeys = Catalog{
	"accounts.manageMenu":      "Manage",
	"accounts.manageMenuTitle": "Groups, institutions, sweep rules, and exchange rates",
}

func init() {
	for k, v := range accountsToolbarKeys {
		english[k] = v
	}
}
