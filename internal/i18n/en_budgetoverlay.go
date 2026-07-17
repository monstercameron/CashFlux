// SPDX-License-Identifier: MIT

package i18n

// budgetOverlayKeys holds the "Last month's spend" overlay copy on /budgets: the
// summary's period-honest labels while the overlay is on. Merged via init so this
// file does not touch en.go.
var budgetOverlayKeys = Catalog{
	// The summary's third figure in overlay mode is last month's UNSPENT budget (its
	// own spend against its own budget) — "Left" would wrongly imply money still
	// available now.
	"budgets.lastMonthUnspent": "Unspent",
}

func init() {
	for k, v := range budgetOverlayKeys {
		english[k] = v
	}
}
