// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/period"
)

func TestExportFilename(t *testing.T) {
	tests := []struct {
		name string
		base string
		res  period.Resolution
		from time.Time
		want string
	}{
		{
			name: "year resolution",
			base: "spending-by-category",
			res:  period.Year,
			from: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			want: "spending-by-category-2025.csv",
		},
		{
			name: "month resolution",
			base: "spending-by-category",
			res:  period.Month,
			from: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			want: "spending-by-category-2026-06.csv",
		},
		{
			name: "month resolution january",
			base: "income-by-source",
			res:  period.Month,
			from: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			want: "income-by-source-2026-01.csv",
		},
		{
			name: "quarter Q1",
			base: "spending-by-category",
			res:  period.Quarter,
			from: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			want: "spending-by-category-2026-Q1.csv",
		},
		{
			name: "quarter Q2",
			base: "top-payees",
			res:  period.Quarter,
			from: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			want: "top-payees-2026-Q2.csv",
		},
		{
			name: "quarter Q3",
			base: "spending-by-category",
			res:  period.Quarter,
			from: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
			want: "spending-by-category-2026-Q3.csv",
		},
		{
			name: "quarter Q4",
			base: "spending-by-category",
			res:  period.Quarter,
			from: time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
			want: "spending-by-category-2026-Q4.csv",
		},
		{
			name: "week resolution single-digit week",
			base: "spending-by-category",
			res:  period.Week,
			from: time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC), // ISO week 2
			want: "spending-by-category-2026-w02.csv",
		},
		{
			name: "week resolution double-digit week",
			base: "largest-expenses",
			res:  period.Week,
			from: time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC), // ISO week 25
			want: "largest-expenses-2026-w25.csv",
		},
		{
			name: "tax summary year",
			base: "tax-summary",
			res:  period.Year,
			from: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
			want: "tax-summary-2024.csv",
		},
		{
			name: "custom field base with hyphen",
			base: "spending-by-deductible",
			res:  period.Month,
			from: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
			want: "spending-by-deductible-2026-03.csv",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExportFilename(tc.base, tc.res, tc.from)
			if got != tc.want {
				t.Errorf("ExportFilename(%q, %q, %v) = %q, want %q",
					tc.base, tc.res, tc.from.Format("2006-01-02"), got, tc.want)
			}
		})
	}
}
