// SPDX-License-Identifier: MIT

package goaltrajectory

import (
	"testing"
	"time"
)

func mustDate(y int, m time.Month) time.Time {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		in   CompareInput
		want Comparison
	}{
		{
			name: "A finishes four months before B, but costs more",
			in: CompareInput{
				AProjected: mustDate(2027, time.March), AReachable: true,
				BProjected: mustDate(2027, time.July), BReachable: true,
				AMonthlyMinor: 50000, BMonthlyMinor: 36200, MonthlyKnown: true,
			},
			want: Comparison{Sooner: SideA, MonthsApart: 4, Costlier: SideA, MonthlyGapMinor: 13800},
		},
		{
			name: "B finishes sooner and is cheaper",
			in: CompareInput{
				AProjected: mustDate(2028, time.January), AReachable: true,
				BProjected: mustDate(2027, time.October), BReachable: true,
				AMonthlyMinor: 40000, BMonthlyMinor: 30000, MonthlyKnown: true,
			},
			want: Comparison{Sooner: SideB, MonthsApart: 3, Costlier: SideA, MonthlyGapMinor: 10000},
		},
		{
			name: "same landing month is a timing tie",
			in: CompareInput{
				AProjected: mustDate(2027, time.June), AReachable: true,
				BProjected: mustDate(2027, time.June), BReachable: true,
				AMonthlyMinor: 20000, BMonthlyMinor: 20000, MonthlyKnown: true,
			},
			want: Comparison{Sooner: SideNone, MonthsApart: 0, SameTiming: true, Costlier: SideNone},
		},
		{
			name: "timing spans a year boundary",
			in: CompareInput{
				AProjected: mustDate(2027, time.November), AReachable: true,
				BProjected: mustDate(2028, time.February), BReachable: true,
			},
			want: Comparison{Sooner: SideA, MonthsApart: 3},
		},
		{
			name: "one goal unreachable → no timing verdict, monthly still compares",
			in: CompareInput{
				AProjected: mustDate(2027, time.March), AReachable: true,
				BReachable:    false,
				AMonthlyMinor: 25000, BMonthlyMinor: 40000, MonthlyKnown: true,
			},
			want: Comparison{Sooner: SideNone, Costlier: SideB, MonthlyGapMinor: 15000},
		},
		{
			name: "monthly unknown (mixed currency) → no monthly verdict",
			in: CompareInput{
				AProjected: mustDate(2027, time.March), AReachable: true,
				BProjected: mustDate(2027, time.May), BReachable: true,
				MonthlyKnown: false,
			},
			want: Comparison{Sooner: SideA, MonthsApart: 2},
		},
		{
			name: "nothing comparable",
			in:   CompareInput{},
			want: Comparison{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.in)
			if got != tc.want {
				t.Errorf("Compare() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestComparisonMeaningful(t *testing.T) {
	if (Comparison{}).Meaningful() {
		t.Error("empty comparison should not be meaningful")
	}
	if !(Comparison{Sooner: SideA, MonthsApart: 2}).Meaningful() {
		t.Error("timing difference should be meaningful")
	}
	if !(Comparison{SameTiming: true}).Meaningful() {
		t.Error("timing tie should be meaningful")
	}
	if !(Comparison{Costlier: SideB, MonthlyGapMinor: 100}).Meaningful() {
		t.Error("monthly difference should be meaningful")
	}
}
