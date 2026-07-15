// SPDX-License-Identifier: MIT

package i18n

// receiptSplitKeys holds English strings for XC11 — proposing a transaction's
// category split from a receipt image. Kept in its own file (not en.go) so the
// change stays additive and off the concurrent-WIP file. Registered at init.
var receiptSplitKeys = Catalog{
	// The ⋯ row-menu entry that starts the receipt-to-split flow.
	"receiptsplit.menuAction": "Split from receipt…",

	// Shown when the action is used without an OpenAI key (BYO-key AI tier).
	"receiptsplit.needsKey": "Reading a receipt needs your OpenAI key. Add it in Settings, then try again — you can still split by hand without a key.",

	// Progress notice while the vision model reads the receipt.
	"receiptsplit.reading": "Reading the receipt…",

	// The model returned no line items.
	"receiptsplit.noneFound": "No line items could be read from that image. Try a clearer photo, or split by hand.",

	// Line items were read but none matched a category, so a proposal wouldn't help.
	"receiptsplit.noProposal": "The receipt's items couldn't be matched to categories, so there's nothing to propose — split by hand instead.",
}

func init() {
	for k, v := range receiptSplitKeys {
		english[k] = v
	}
}
