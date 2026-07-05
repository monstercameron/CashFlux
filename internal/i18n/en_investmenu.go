// SPDX-License-Identifier: MIT

package i18n

// investMenuKeys holds the English strings for the holdings-row ⋯ menu (close /
// delete a security). Merged via init so this file does not touch en.go.
var investMenuKeys = Catalog{
	"investments.holdingMenuAria":   "Holding actions",
	"investments.closePosition":     "Close position…",
	"investments.closeConfirm":      "Close %s — mark the position sold and remove it from the account? To reflect the cash, record the sale as a transaction on the account.",
	"investments.closeConfirmBtn":   "Close position",
	"investments.closedNotice":      "Closed %s. Record the sale proceeds as a transaction if you want the cash reflected.",
	"investments.deleteHoldingItem": "Delete record…",
}

func init() {
	for k, v := range investMenuKeys {
		english[k] = v
	}
}
