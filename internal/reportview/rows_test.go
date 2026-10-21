// SPDX-License-Identifier: MIT

package reportview

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

func TestCategoryReviewAveragesUseConfiguredWindow(t *testing.T) {
	for _, months := range []int{3, 6, 12, 24} {
		t.Run(fmt.Sprint(months), func(t *testing.T) {
			got, err := BuildCategoryReview([]reports.CategorySpend{{CategoryID: "food", Amount: 7200}}, nil, nil, false, months, 14400)
			if err != nil {
				t.Fatal(err)
			}
			row := got.Rows[0]
			if row.AverageMinor != 7200/int64(months) || row.SharePercent != 50 || row.WidthPercent != 100 || row.HistogramPercent != 100 {
				t.Fatalf("%d-month readings = %+v", months, row)
			}
		})
	}
}

func TestCategoryReviewRollupKeepsSparkAndAmountTogether(t *testing.T) {
	cats := []domain.Category{{ID: "food"}, {ID: "groceries", ParentID: "food"}, {ID: "coffee", ParentID: "groceries"}, {ID: "self", ParentID: "self"}, {ID: "a", ParentID: "b"}, {ID: "b", ParentID: "a"}}
	spending := []reports.CategorySpend{{CategoryID: "groceries", Amount: 200}, {CategoryID: "food", Amount: 100}, {CategoryID: "coffee", Amount: 50}, {CategoryID: "", Amount: 20}, {CategoryID: "unknown", Amount: 10}, {CategoryID: "self", Amount: 5}, {CategoryID: "a", Amount: 2}, {CategoryID: "b", Amount: 3}}
	var trends []reports.CategoryTrend
	for _, spend := range spending {
		trends = append(trends, reports.CategoryTrend{CategoryID: spend.CategoryID, Spend: []int64{0, spend.Amount, 0}, Total: spend.Amount})
	}
	before := append([]reports.CategorySpend(nil), spending...)
	got, err := BuildCategoryReview(spending, trends, cats, true, 3, 390)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range got.Rows {
		var sum int64
		for _, value := range row.Spark {
			sum += value
		}
		if sum != row.Amount {
			t.Fatalf("row %q amount %d and spark %v disagree", row.Spend.CategoryID, row.Amount, row.Spark)
		}
	}
	if got.Rows[0].Spend.CategoryID != "food" || got.Rows[0].Amount != 350 || got.UncategorizedMinor != 20 {
		t.Fatalf("rolled readings = %+v", got)
	}
	if !reflect.DeepEqual(spending, before) || trends[0].Spend[1] != 200 {
		t.Fatal("rollup mutated raw rows or bucket values")
	}
}

func TestCategoryReviewRemainderAndWideRatios(t *testing.T) {
	spending := make([]reports.CategorySpend, 15)
	spending[0] = reports.CategorySpend{CategoryID: "largest", Amount: math.MaxInt64 / 2}
	for i := 1; i < len(spending); i++ {
		spending[i] = reports.CategorySpend{CategoryID: fmt.Sprint(i), Amount: 100}
	}
	got, err := BuildCategoryReview(spending, nil, nil, false, 12, math.MaxInt64/2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rows[0].SharePercent != 100 || len(got.Histogram) != 12 || got.RestCount != 3 || got.RestMinor != 300 || got.RestWidthPercent != 1 {
		t.Fatalf("histogram = %+v", got)
	}
	if _, err := BuildCategoryReview([]reports.CategorySpend{{Amount: math.MinInt64}}, nil, nil, false, 12, 100); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("unrepresentable magnitude = %v", err)
	}
}

func TestTagRowsPreserveComparisonRulesWithoutMultiplicationOverflow(t *testing.T) {
	rows, err := BuildTagRows([]reports.TagSpend{
		{Tag: "new", Amount: 100}, {Tag: "down", Amount: 50, Prior: 100}, {Tag: "same", Amount: 100, Prior: 100},
		{Tag: "wide", Amount: math.MaxInt64, Prior: math.MaxInt64 / 2},
	}, math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if !rows[0].PriorZero || rows[0].HasDelta || rows[1].DeltaPercent != -50 || !rows[2].HasDelta || rows[2].DeltaPercent != 0 || rows[3].DeltaPercent != 100 || rows[3].WidthPercent != 100 {
		t.Fatalf("tag readings = %+v", rows)
	}
	if _, err := BuildTagRows([]reports.TagSpend{{Amount: math.MaxInt64, Prior: 1}}, 100); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("unrepresentable percentage = %v", err)
	}
}

func TestGroupDepositsPreservesResolvedNameAmountAndLatestDate(t *testing.T) {
	items := []reports.ExpenseItem{
		{Desc: "Payroll", Amount: 10000, Date: day(time.June, 2)},
		{CategoryID: "salary", Amount: 10000, Date: day(time.June, 5)},
		{Desc: "Payroll", Amount: 20000, Date: day(time.June, 3)},
		{Desc: "Payroll", Amount: 10000, Date: day(time.June, 1)},
		{CategoryID: "missing", Amount: 10000, Date: day(time.June, 4)},
	}
	got := GroupDeposits(items, func(id string) string {
		if id == "salary" {
			return "Payroll"
		}
		return "Uncategorized"
	})
	if len(got) != 3 || got[0].Description != "Payroll" || got[0].Amount != 10000 || got[0].Count != 3 || !got[0].Last.Equal(day(time.June, 5)) || got[1].Amount != 20000 || got[2].Description != "Uncategorized" {
		t.Fatalf("deposit groups = %+v", got)
	}
	if items[0].Desc != "Payroll" || !items[0].Date.Equal(day(time.June, 2)) {
		t.Fatal("grouping mutated input")
	}
}
