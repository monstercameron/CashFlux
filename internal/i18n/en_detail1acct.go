// SPDX-License-Identifier: MIT

package i18n

// detail1AcctKeys are the copy for the 2026-07-19 detail-polish lane 1 accounts
// items: the stale-summary collapse line and the individually-owned-account owner
// chip. Kept in its own file (same init-merge pattern as the other en_*.go
// extensions) so concurrent agent WIP doesn't collide.
var detail1AcctKeys = Catalog{
	// When more than half the visible accounts are stale, the per-row STALE badges
	// collapse to a subdued dot and this one summary line leads the list (its action
	// reuses the toolbar's existing "Mark all updated").
	"accounts.staleSummary": "Most accounts need a balance update",
	// aria-label for the subdued per-row stale dot shown in the collapsed state.
	"accounts.staleDotAria": "Balance is out of date",

	// The owner chip on an individually-owned account row (the Shared chip already
	// marks group/shared accounts; this names the person for the rest). The chip
	// renders the member name directly; this is its hover/aria title.
	"accounts.ownerBadgeTitle": "Owned by %s",
}

func init() {
	for k, v := range detail1AcctKeys {
		english[k] = v
	}
}
