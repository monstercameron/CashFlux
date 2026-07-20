// SPDX-License-Identifier: MIT

package i18n

// detail1DashKeys are the copy for the 2026-07-19 detail-polish lane 1 dashboard
// items: the standing edit-layout hint. Kept in its own file (same init-merge
// pattern as the other en_*.go extensions) so concurrent agent WIP doesn't collide.
var detail1DashKeys = Catalog{
	// A single quiet standing hint shown at the top of the dashboard while
	// edit-layout mode is on, so the many per-tile resize/drag controls have one
	// plain-English explanation instead of none.
	"dashboard.editLayoutHint": "Drag a tile to reorder · hover a tile for resize handles · Done saves",
}

func init() {
	for k, v := range detail1DashKeys {
		english[k] = v
	}
}
