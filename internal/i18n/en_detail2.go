// SPDX-License-Identifier: MIT

package i18n

// detail2Keys holds English strings added by the 2026-07-19 fine-detail polish
// batch (detail-lane 2 — Transactions import chooser + recent-imports honesty +
// retained-search chip). Merged via init so the shared en.go and other lanes'
// catalogs are never touched by this concurrent lane.
var detail2Keys = Catalog{
	// Recent-imports honesty (finding #2): a real per-import roll-back exists, but
	// only while the run's safety snapshot is still in the capped ring — so the hint
	// no longer promises it unconditionally, and names the always-available fallback.
	"documents.historyHintHonest": "Each run's full result. Roll back restores your data to just before an import while its safety snapshot is still saved — otherwise undo right after importing with Ctrl+Z.",
	"documents.historyActivityLink": "Review changes in Activity",
}

func init() {
	for k, v := range detail2Keys {
		english[k] = v
	}
}
