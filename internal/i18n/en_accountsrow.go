// SPDX-License-Identifier: MIT

package i18n

// accountsRowKeys holds the English strings for the accounts list's row labels,
// group editor and institution editor messages. Registered at init time like the
// other supplemental catalogs.
var accountsRowKeys = Catalog{
	"accounts.viewTxnsAria":            "Transactions — %s",
	"accounts.moreActionsAria":         "More actions — %s",
	"accounts.groupSubtotalNote":       " · %s",
	"account.percentSuffix":            "%",
	"accounts.groupDeleteConfirm":      "Delete the \"%s\" group? Its accounts move back to Ungrouped — nothing else changes.",
	"accounts.institutionColorInvalid": "Pick the colour again — it has to be a hex value such as #7c83ff.",
}

func init() {
	for k, v := range accountsRowKeys {
		english[k] = v
	}
}
