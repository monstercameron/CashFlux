// SPDX-License-Identifier: MIT

package i18n

// receiptSplitKeys holds English strings for XC11 — proposing a transaction's
// category split from a receipt image. Kept in its own file (not en.go) so the
// change stays additive and off the concurrent-WIP file. Registered at init.
var receiptSplitKeys = Catalog{
	// The ⋯ row-menu entry that starts the receipt-to-split flow.
	"receiptsplit.menuAction": "Split using receipt",

	// Shown when the action is used without an OpenAI key (BYO-key AI tier).
	"receiptsplit.needsKey": "Add your AI key in Settings to split from a receipt. You can still split by hand.",

	// Progress notice while the vision model reads the receipt.
	"receiptsplit.reading": "Reading the receipt…",

	// The model returned no line items.
	"receiptsplit.noneFound": "Couldn’t read any items from that image. Try a clearer photo, or split by hand.",

	// Line items were read but none matched a category, so a proposal wouldn't help.
	"receiptsplit.noProposal": "Couldn’t match the receipt items to categories, so there is nothing to suggest. Split by hand instead.",
}

func init() {
	for k, v := range receiptSplitKeys {
		english[k] = v
	}
}
