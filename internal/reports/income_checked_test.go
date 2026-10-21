// SPDX-License-Identifier: MIT

package reports

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestIncomeByCategoryCheckedSplitsRemainderAndMatchesLegacyRows(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	txns := []domain.Transaction{
		{ID: "pay", Date: start, CategoryID: "salary", Amount: money.New(10000, "USD"), Splits: []domain.CategorySplit{
			{CategoryID: "salary", Amount: money.New(4000, "USD")},
			{CategoryID: "bonus", Amount: money.New(3000, "USD")},
		}},
		{ID: "outside", Date: end, CategoryID: "ignored", Amount: money.New(5000, "USD")},
		{ID: "transfer", Date: start, CategoryID: "ignored", TransferAccountID: "other", Amount: money.New(7000, "USD")},
	}
	rates := currency.Rates{Base: "USD"}
	got, err := IncomeByCategoryChecked(txns, start, end, rates)
	if err != nil {
		t.Fatal(err)
	}
	want := []CategorySpend{{CategoryID: "salary", Amount: 7000}, {CategoryID: "bonus", Amount: 3000}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("checked income rows = %+v, want %+v", got, want)
	}
	legacy, err := IncomeByCategory(txns, start, end, rates)
	if err != nil || !reflect.DeepEqual(legacy, got) {
		t.Fatalf("IncomeByCategory = %+v, %v; want checked rows %+v", legacy, err, got)
	}
}

func TestIncomeByCategoryCheckedRejectsUnknownFXAndOverflow(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	for _, tc := range []struct {
		name   string
		txns   []domain.Transaction
		isOver bool
	}{
		{name: "missing FX", txns: []domain.Transaction{{Date: start, Amount: money.New(100, "EUR")}}},
		{name: "split magnitude", txns: []domain.Transaction{{Date: start, Amount: money.New(1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "s", Amount: money.New(math.MinInt64, "USD")}}}}, isOver: true},
		{name: "category sum", txns: []domain.Transaction{{Date: start, CategoryID: "s", Amount: money.New(math.MaxInt64, "USD")}, {Date: start, CategoryID: "s", Amount: money.New(1, "USD")}}, isOver: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := IncomeByCategoryChecked(tc.txns, start, end, currency.Rates{Base: "USD"})
			if err == nil || got != nil {
				t.Fatalf("invalid income became rows=%+v err=%v", got, err)
			}
			if tc.isOver && !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("overflow error = %v, want %v", err, money.ErrOverflow)
			}
		})
	}
}
