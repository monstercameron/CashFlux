// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/period"
)

// ExportFilename returns a period-stamped download filename for a Reports CSV
// export so that multiple exports from different periods do not overwrite each
// other in the browser's Downloads folder (L45/L58).
//
// The stamp format is chosen by resolution:
//
//   - Year  → base + "-" + year          e.g. "spending-by-category-2025.csv"
//   - Month → base + "-" + year-month    e.g. "spending-by-category-2026-06.csv"
//   - Quarter → base + "-" + quarter     e.g. "spending-by-category-2026-Q2.csv"
//   - Week  → base + "-" + ISO week date e.g. "spending-by-category-2026-w25.csv"
//
// base should be the bare name without extension (e.g. "spending-by-category");
// from is the window's From anchor (the start of the viewed period).
func ExportFilename(base string, res period.Resolution, from time.Time) string {
	var stamp string
	switch res {
	case period.Year:
		stamp = fmt.Sprintf("%d", from.Year())
	case period.Quarter:
		q := (int(from.Month())-1)/3 + 1
		stamp = fmt.Sprintf("%d-Q%d", from.Year(), q)
	case period.Week:
		_, week := from.ISOWeek()
		stamp = fmt.Sprintf("%d-w%02d", from.Year(), week)
	default: // Month
		stamp = from.Format("2006-01")
	}
	return base + "-" + stamp + ".csv"
}
