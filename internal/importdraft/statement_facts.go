// SPDX-License-Identifier: MIT

package importdraft

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
)

// AddedCount returns the import's saved row count, including legacy history
// records whose imported rows were saved before RowCount existed.
func AddedCount(document domain.Document) int {
	if document.RowCount == 0 {
		return len(document.Extracted)
	}
	return document.RowCount
}

// DateRange returns the earliest and latest valid source dates. Invalid dates
// remain visible as row errors and never become misleading statement facts.
func DateRange(rows []extract.Row) (first, last time.Time) {
	for _, row := range rows {
		date, err := dateutil.ParseDate(row.Date)
		if err != nil || date.IsZero() {
			continue
		}
		if first.IsZero() || date.Before(first) {
			first = date
		}
		if last.IsZero() || date.After(last) {
			last = date
		}
	}
	return first, last
}
