// SPDX-License-Identifier: MIT

package i18n

// orderImportKeys holds the English strings for the Amazon order-history import
// card (TX4). Defined in their own file and merged via init so this does not
// touch the shared en.go.
var orderImportKeys = Catalog{
	"orderimport.cardTitle": "Amazon order history",
	"orderimport.cardDesc": "Match your Amazon orders to the card charges that paid for them, then " +
		"apply the itemized breakdown. Everything is parsed on your device — nothing is sent anywhere.",
	"orderimport.chooseFile":  "Choose order-history CSV",
	"orderimport.fileOrPaste": "Use your Amazon privacy-export CSV, or paste from the orders page below.",
	"orderimport.findOrders":  "Find orders",
	"orderimport.pastePlaceholder": "Paste from Your Orders:\nORDER PLACED July 1, 2026\nTOTAL $23.98\n" +
		"Order # 111-5556667\nUSB-C Charging Cable 6ft",
	"orderimport.localNote":     "All parsing happens locally. No network calls, no scraping.",
	"orderimport.parsed":        "Found %d order(s). Matched to your card charges below.",
	"orderimport.noneParsed":    "No orders could be read from that input.",
	"orderimport.matchedSingle": "Matched to one charge",
	"orderimport.matchedMulti":  "Matched to %d shipment charges",
	"orderimport.unmatched":     "No matching charge found",
	"orderimport.drift":         "%s covered by gift card or promo",
	"orderimport.apply":         "Apply",
	"orderimport.grouped":       "Grouped %d charges into this order.",
	"orderimport.groupErr":      "Couldn't group the charges: %s",
	"orderimport.noMatch":       "This order has no matching charge to apply.",
	"orderimport.noItems":       "This order has no line items to split.",
	"orderimport.txnGone":       "That transaction is no longer in the ledger.",
}

func init() {
	for k, v := range orderImportKeys {
		english[k] = v
	}
}
