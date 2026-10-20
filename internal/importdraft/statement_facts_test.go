// SPDX-License-Identifier: MIT

package importdraft

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
)

func TestImportAddedCount(t *testing.T) {
	for _, tc := range []struct {
		document domain.Document
		want     int
	}{
		{domain.Document{}, 0},
		{domain.Document{RowCount: 2, Extracted: []domain.DocumentRow{{}}}, 2},
		{domain.Document{Extracted: []domain.DocumentRow{{}, {}, {}}}, 3},
	} {
		if got := AddedCount(tc.document); got != tc.want {
			t.Errorf("AddedCount = %d, want %d", got, tc.want)
		}
	}
}

func TestStatementDateRange(t *testing.T) {
	for _, tc := range []struct {
		name        string
		rows        []extract.Row
		first, last string
	}{
		{name: "empty"},
		{name: "invalid", rows: []extract.Row{{Date: "wrong"}, {Date: "2026-02-30"}}},
		{name: "one", rows: []extract.Row{{Date: "2026-03-01"}}, first: "2026-03-01", last: "2026-03-01"},
		{name: "unordered with invalid", rows: []extract.Row{{Date: "2026-03-03"}, {Date: "bad"}, {Date: "2026-03-01"}, {Date: "2026-03-02"}}, first: "2026-03-01", last: "2026-03-03"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, last := DateRange(tc.rows)
			if tc.first == "" {
				if !first.IsZero() || !last.IsZero() {
					t.Fatal(first, last)
				}
				return
			}
			if first.Format("2006-01-02") != tc.first || last.Format("2006-01-02") != tc.last {
				t.Fatal(first, last)
			}
		})
	}
}
