// SPDX-License-Identifier: MIT

package reportview

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

func TestBuildCustomFieldReviewPreservesDominanceRoundingAndOrder(t *testing.T) {
	rows := []reports.CustomFieldSpend{
		{Value: "", Amount: 80},
		{Value: "valued-a", Amount: 14},
		{Value: "valued-b", Amount: 6},
	}
	got, err := BuildCustomFieldReview(rows)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalMinor != 100 || got.NoValueMinor != 80 || got.NoValuePercent != 80 || !got.DominantUnvalued || got.AllUnvalued {
		t.Fatalf("custom field summary = %+v", got)
	}
	if len(got.Rows) != 3 || len(got.DisplayRows) != 2 || got.DisplayRows[0].Spend.Value != "valued-a" || got.DisplayRows[0].WidthPercent != 100 || got.DisplayRows[1].WidthPercent != 42 {
		t.Fatalf("rows/order/widths = %+v", got)
	}
	if rows[0].Value != "" || len(rows) != 3 {
		t.Fatal("source rows were mutated")
	}
}

func TestBuildCustomFieldReviewRejectsTotalsAndBarInputsThatOverflow(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []reports.CustomFieldSpend
	}{
		{name: "grand total", rows: []reports.CustomFieldSpend{{Value: "a", Amount: math.MaxInt64}, {Value: "b", Amount: 1}}},
		{name: "negative amount", rows: []reports.CustomFieldSpend{{Value: "a", Amount: -1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildCustomFieldReview(tc.rows)
			if err == nil || len(got.DisplayRows) != 0 || got.TotalMinor != 0 {
				t.Fatalf("invalid rows returned partial facts: %+v, %v", got, err)
			}
		})
	}
}

func TestBuildDeductibleReviewChecksTotalAndPreservesWidthOrder(t *testing.T) {
	rows := []reports.DeductibleRow{{CategoryID: "large", Amount: 200}, {CategoryID: "small", Amount: 50}}
	got, err := BuildDeductibleReview(reports.DeductibleSummary{Rows: rows, Total: 250})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 2 || got.Rows[0].Row.CategoryID != "large" || got.Rows[0].WidthPercent != 100 || got.Rows[1].WidthPercent != 25 {
		t.Fatalf("deductible rows = %+v", got.Rows)
	}
	if _, err := BuildDeductibleReview(reports.DeductibleSummary{Rows: rows, Total: 249}); err == nil {
		t.Fatal("mismatched summary total was accepted")
	}
}

func TestBuildDeductibleReviewOwnsSummaryRows(t *testing.T) {
	source := []reports.DeductibleRow{{CategoryID: "medical", Amount: 250}}
	got, err := BuildDeductibleReview(reports.DeductibleSummary{Rows: source, Total: 250})
	if err != nil {
		t.Fatal(err)
	}
	source[0].Amount = 1
	source[0].CategoryID = "changed"
	if got.Summary.Rows[0].Amount != 250 || got.Summary.Rows[0].CategoryID != "medical" {
		t.Fatalf("summary rows alias caller input: %+v", got.Summary.Rows)
	}
	got.Summary.Rows[0].Amount = 2
	if got.Rows[0].Row.Amount != 250 || got.Rows[0].Row.CategoryID != "medical" {
		t.Fatalf("prepared display row aliases summary rows: %+v", got.Rows[0])
	}
}

func TestReportRatioUsesWideProductButChecksFinalQuotient(t *testing.T) {
	got, err := reportRatio(math.MaxInt64/2, math.MaxInt64)
	if err != nil || got != 49 {
		t.Fatalf("wide intermediate ratio = %d, %v; want 49", got, err)
	}
	if _, err := reportRatio(math.MaxInt64, 1); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("unrepresentable final ratio error = %v; want overflow", err)
	}
}

func TestBuildInvestmentReviewCheckedTotalsAndReturnRounding(t *testing.T) {
	got, err := BuildInvestmentReview([]reports.AccountPerformance{
		{AccountID: "one", Invested: 8, Current: 9, Gain: 1},
		{AccountID: "two", Invested: 2, Current: 3, Gain: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalInvested != 10 || got.TotalCurrent != 12 || got.TotalGain != 2 || got.TotalReturnTenthsPct != 200 || !got.HasReturn {
		t.Fatalf("investment summary = %+v", got)
	}
	rounded, err := BuildInvestmentReview([]reports.AccountPerformance{{Invested: 8, Current: 9, Gain: 1}})
	if err != nil || rounded.TotalReturnTenthsPct != 125 {
		t.Fatalf("one-decimal return rounding = %+v, %v; want 12.5%%", rounded, err)
	}

	bad, err := BuildInvestmentReview([]reports.AccountPerformance{{Invested: math.MaxInt64}, {Invested: 1}})
	if !errors.Is(err, money.ErrOverflow) || len(bad.Rows) != 0 {
		t.Fatalf("overflowing investment total = %+v, %v", bad, err)
	}
	bad, err = BuildInvestmentReview([]reports.AccountPerformance{{AccountID: "minimum", Invested: math.MinInt64}})
	if !errors.Is(err, money.ErrOverflow) || len(bad.Rows) != 0 {
		t.Fatalf("unrepresentable negative basis magnitude = %+v, %v", bad, err)
	}
}

func TestBuildInvestmentReviewRejectsCurrentGainAndRatioOverflow(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []reports.AccountPerformance
	}{
		{name: "current total", rows: []reports.AccountPerformance{{Current: math.MaxInt64}, {Current: 1}}},
		{name: "gain total", rows: []reports.AccountPerformance{{Gain: math.MaxInt64}, {Gain: 1}}},
		{name: "return ratio", rows: []reports.AccountPerformance{{Invested: 1, Gain: math.MaxInt64}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildInvestmentReview(tc.rows)
			if !errors.Is(err, money.ErrOverflow) || len(got.Rows) != 0 {
				t.Fatalf("overflow returned partial investment facts: %+v, %v", got, err)
			}
		})
	}
}

func TestBuildInvestmentReviewPreservesHalfEvenDisplayTies(t *testing.T) {
	for _, tc := range []struct {
		gain int64
		want int64
	}{
		{gain: 1, want: 62},  // 6.25% prints as 6.2%.
		{gain: 3, want: 188}, // 18.75% prints as 18.8%.
		{gain: -1, want: -62},
		{gain: -3, want: -188},
	} {
		got, err := BuildInvestmentReview([]reports.AccountPerformance{{Invested: 16, Gain: tc.gain}})
		if err != nil || got.TotalReturnTenthsPct != tc.want {
			t.Fatalf("gain %d: return tenths = %d, %v; want %d", tc.gain, got.TotalReturnTenthsPct, err, tc.want)
		}
	}
}
