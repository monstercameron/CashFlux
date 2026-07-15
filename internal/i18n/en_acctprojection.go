// SPDX-License-Identifier: MIT

package i18n

// acctProjectionKeys holds the English strings for the per-account projected-balance
// row line (AC13 — "$2,340 today → ~$1,150 low on the 28th" with an expandable list
// of the drivers behind the low). Kept in its own file and merged via init so this
// does not touch the shared en.go.
var acctProjectionKeys = Catalog{
	// %s = formatted projected low, second %s = the date it occurs (e.g. "the 28th").
	"accounts.projectedLow": "→ ~%s low on %s",
	// Accessible name for the projected-low line. %s = low amount, %s = date.
	"accounts.projectedLowAria": "Projected low of %s on %s over the next 30 days",
	"accounts.projectedShow":    "Why?",
	"accounts.projectedHide":    "Hide",
	// Header above the expanded driver list.
	"accounts.projectedDrivers": "What moves this balance",
	// One driver line: %s = label, %s = signed amount, %s = date. e.g. "Rent −$1,400 on the 1st".
	"accounts.projectedDriver": "%s %s on %s",
	"accounts.projectedTitle":  "A 30-day projection from your recurring money on this account. A due date is when the money is expected to move.",
}

func init() {
	for k, v := range acctProjectionKeys {
		english[k] = v
	}
}
