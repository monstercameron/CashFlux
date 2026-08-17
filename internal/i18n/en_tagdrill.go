// SPDX-License-Identifier: MIT

package i18n

// tagDrillKeys is what the ledger says when a row's "Filter to #coffee" control
// is used (C651).
//
// The filter was being applied correctly the whole time. What made the control
// read as dead is that on the reported view — a search already narrowing the
// ledger to the one row carrying the tag — nothing it could change was visible.
// A control that alters state invisibly and says nothing is a dead button from
// where the user sits, so the fix is the sentence it was missing, including the
// way back out of it.
//
// It is posted ONLY when the visible result set did not move. On an ordinary
// drill the chip appears and the count changes, which is how every other filter
// change in this app announces itself; adding a toast to that case would be noise
// for the common path in order to fix the uncommon one.
var tagDrillKeys = Catalog{
	"transactions.tagFilterApplied": "Filtered to #%s. Remove the tag chip to go back.",
}

func init() {
	for k, v := range tagDrillKeys {
		english[k] = v
	}
}
