// SPDX-License-Identifier: MIT

package ledger

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCategorySpendSelectedMatchesFullFoldForSelectedLines(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bounds := []time.Time{start, start.AddDate(0, 1, 0), start.AddDate(0, 2, 0)}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	rows := []domain.Transaction{
		{ID: "split", Date: start, Amount: money.New(-1000, "USD"), CategoryID: "other", Splits: []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-600, "USD")}, {CategoryID: "other", Amount: money.New(-500, "USD")}, {CategoryID: "ded", Amount: money.New(100, "USD")}}},
		{ID: "later", Date: bounds[1], Amount: money.New(-100, "EUR"), CategoryID: "ded"},
		{ID: "uncategorized", Date: start, Amount: money.New(-50, "USD")},
		{ID: "excluded", Date: start, Amount: money.New(math.MinInt64, "NOFX"), ExcludeFromReports: true},
		{ID: "end", Date: bounds[2], Amount: money.New(math.MinInt64, "NOFX")},
		{ID: "transfer", Date: start, Amount: money.New(math.MinInt64, "NOFX"), TransferAccountID: "other"},
	}
	wanted := map[string]bool{"ded": true, "": true, "other": false}
	before, _ := json.Marshal(rows)
	full, err := CategorySpendSeries(rows, bounds, rates)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CategorySpendSeriesSelected(rows, bounds, rates, wanted)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string][]int64{"ded": {600, 200}, "": {50, 0}}) {
		t.Fatalf("selected sums: %+v", got)
	}
	for key, values := range got {
		if !reflect.DeepEqual(values, full[key]) {
			t.Fatalf("full-fold parity for %q: %v / %v", key, values, full[key])
		}
	}
	if len(wanted) != 3 || wanted["other"] {
		t.Fatal("selection map mutated")
	}
	after, _ := json.Marshal(rows)
	if string(before) != string(after) {
		t.Fatal("input rows or split lines mutated")
	}
	if got, err := CategorySpendSeriesSelected(rows, bounds, rates, nil); err != nil || len(got) != 0 {
		t.Fatalf("nil selection must select none: %+v / %v", got, err)
	}
}

func TestCategorySpendSelectedPreservesUnusedSourceValidation(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bounds := []time.Time{start, start.AddDate(0, 1, 0)}
	rates := currency.Rates{Base: "USD"}
	for _, tc := range []struct {
		name string
		rows []domain.Transaction
		want error
	}{
		{"parent FX", []domain.Transaction{{Amount: money.New(-1, "EUR")}}, currency.ErrUnknownRate},
		{"parent magnitude", []domain.Transaction{{Amount: money.New(math.MinInt64, "USD")}}, money.ErrOverflow},
		{"split FX", []domain.Transaction{{Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "unused", Amount: money.New(-1, "EUR")}}}}, currency.ErrUnknownRate},
		{"split magnitude", []domain.Transaction{{Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "unused", Amount: money.New(math.MinInt64, "USD")}}}}, money.ErrOverflow},
		{"same-record unused category sum", []domain.Transaction{{Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "unused", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "credit", Amount: money.New(math.MaxInt64, "USD")}, {CategoryID: "unused", Amount: money.New(-1, "USD")}}}}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := range tc.rows {
				tc.rows[i].Date = start
			}
			got, err := CategorySpendSeriesSelected(tc.rows, bounds, rates, map[string]bool{"ded": true})
			if !errors.Is(err, tc.want) || got != nil {
				t.Fatalf("unused validation omitted: %+v / %v", got, err)
			}
		})
	}
}

func TestCategorySpendSelectedDoesNotAccumulateUnusedWindowTotals(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bounds := []time.Time{start, start.AddDate(0, 1, 0)}
	rates := currency.Rates{Base: "USD"}
	rows := []domain.Transaction{
		{ID: "a", Date: start, Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "unused", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "credit", Amount: money.New(math.MaxInt64-1, "USD")}}},
		{ID: "b", Date: start, Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "unused", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "credit", Amount: money.New(math.MaxInt64-1, "USD")}}},
		{ID: "ded", Date: start, Amount: money.New(-100, "USD"), CategoryID: "ded"},
	}
	if _, err := CategorySpendSeries(rows, bounds, rates); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("fixture must overflow unused whole-window total: %v", err)
	}
	got, err := CategorySpendSeriesSelected(rows, bounds, rates, map[string]bool{"ded": true})
	if err != nil || !reflect.DeepEqual(got, map[string][]int64{"ded": {100}}) {
		t.Fatalf("unused window total blocked selection: %+v / %v", got, err)
	}
	if _, err := CategorySpendSeriesSelected(rows, bounds, rates, map[string]bool{"unused": true}); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("selected overflow not retained: %v", err)
	}
}
