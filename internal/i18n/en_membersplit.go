// SPDX-License-Identifier: MIT

package i18n

// memberSplitKeys holds English strings for the per-member income-split card on
// the /members screen (C279/C280 delta — #474). Registered at init time using
// the new-file init-merge pattern so en.go (which may have concurrent WIP) is
// never touched.
var memberSplitKeys = Catalog{
	// Section heading for the "Income split this period" card on /members.
	"members.incomeSplitTitle": "Income split this period",
}

func init() {
	for k, v := range memberSplitKeys {
		english[k] = v
	}
}
