// SPDX-License-Identifier: MIT

package i18n

// imv9Keys contains copy for the verifiable Amazon order-match review.
var imv9Keys = Catalog{
	"orderimport.emptyPaste":       "Paste your orders first.",
	"orderimport.noOrderItems":     "No item details were included in this order.",
	"orderimport.chargeLabel":      "Matched charge",
	"orderimport.chargesLabel":     "Matched charges",
	"orderimport.itemsLabel":       "Order items",
	"orderimport.chargeMissing":    "A matched transaction is no longer in the ledger. Run Find orders again.",
	"orderimport.amountDifference": "Amounts differ by %s: gift card, promo or a partial charge?",
	"orderimport.itemLine":         "%d × %s — %s",
}

func init() {
	for key, value := range imv9Keys {
		english[key] = value
	}
}
