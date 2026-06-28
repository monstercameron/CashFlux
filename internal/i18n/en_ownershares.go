// SPDX-License-Identifier: MIT

package i18n

// ownerSharesKeys holds the English strings for the fractional account
// ownership feature (C279). Registered at init time using the same
// new-file init-merge pattern as other supplemental catalogs so en.go
// (which may have concurrent WIP) is never touched.
var ownerSharesKeys = Catalog{
	"account.splitOwnership":        "Split ownership",
	"account.splitOwnershipToggle":  "Split this account across multiple members",
	"account.splitOwnershipHint":    "Assign a percentage to each member. Shares must add up to 100.",
	"account.sharePercent":          "% share",
	"account.shareSumError":         "Shares must sum to 100 (currently %d)",
	"account.sharesSaved":           "Ownership shares saved",
	"account.ownerSharesLabel":      "Ownership shares",
}

func init() {
	for k, v := range ownerSharesKeys {
		english[k] = v
	}
}
