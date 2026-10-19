// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestComputeTrackingMetadata(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	txns := []domain.Transaction{
		{ID: "split", Date: start.AddDate(0, 0, 2), CategoryID: "whole", Tags: []string{" Trip ", "trip"}, Amount: money.New(-1000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-400, "USD")}, {CategoryID: "travel", Amount: money.New(-600, "USD")}}},
		{ID: "outside", Date: end, CategoryID: "food", Amount: money.New(-200, "USD")},
		{ID: "income", Date: start, CategoryID: "pay", Amount: money.New(500, "USD")},
		{ID: "excluded", Date: start, CategoryID: "food", ExcludeFromReports: true, Amount: money.New(-100, "USD")},
	}
	got := ComputeTrackingMetadata(txns, start, end, "USD", currency.Rates{Base: "USD"})
	if got.CategoryCount["food"] != 1 || got.CategoryTotal["food"] != 400 {
		t.Errorf("food metadata = (%d, %d), want (1, 400)", got.CategoryCount["food"], got.CategoryTotal["food"])
	}
	if got.CategoryCount["travel"] != 1 || got.CategoryTotal["travel"] != 600 {
		t.Errorf("travel metadata = (%d, %d), want (1, 600)", got.CategoryCount["travel"], got.CategoryTotal["travel"])
	}
	if got.TagCount["trip"] != 2 || got.TagTotal["trip"] != 2000 {
		t.Errorf("tag metadata = (%d, %d), want (2, 2000)", got.TagCount["trip"], got.TagTotal["trip"])
	}
}

func TestComputeTrackingMetadataSkipsUnconvertibleAmounts(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	txn := domain.Transaction{ID: "fx", Date: start, CategoryID: "food", Tags: []string{"trip"}, Amount: money.New(-1000, "EUR")}
	got := ComputeTrackingMetadata([]domain.Transaction{txn}, start, start.AddDate(0, 1, 0), "USD", currency.Rates{Base: "USD"})
	if len(got.CategoryCount) != 0 || len(got.TagCount) != 0 {
		t.Fatalf("unconvertible transaction contributed metadata: %+v", got)
	}
}
