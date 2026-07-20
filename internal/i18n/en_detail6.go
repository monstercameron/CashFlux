// SPDX-License-Identifier: MIT

package i18n

// detail6Keys holds English strings added by the 2026-07-19 detail-lane-6 polish
// pass (fine-detail review). Merged via init so the shared en.go is never touched
// by this concurrent lane.
var detail6Keys = Catalog{
	// Credit proxy-score label — one shared phrasing used at first mention on every
	// reports/health render site (the Annual Review trend chart and the embedded
	// credit-health panel) so the figure never reads as a bureau score before its
	// later disclaimer.
	"detail6.creditScoreLabel": "Credit health score (CashFlux estimate — not a bureau score)",
}

func init() {
	for k, v := range detail6Keys {
		english[k] = v
	}
}
