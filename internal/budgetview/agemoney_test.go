// SPDX-License-Identifier: MIT

package budgetview

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAgeOfMoneyUsesOrderedConvertedCashHistory(t *testing.T) {
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "reported exclusion", Amount: money.New(-100, "USD"), Date: first.AddDate(0, 0, 20), ExcludeFromReports: true},
		{ID: "foreign", Amount: money.New(-100, "EUR"), Date: first.AddDate(0, 0, 10)},
		{ID: "income", Amount: money.New(1000, "USD"), Date: first},
		{ID: "missing FX", Amount: money.New(-9000, "JPY"), Date: first.AddDate(0, 0, 5)},
		{ID: "transfer", Amount: money.New(-9000, "USD"), TransferAccountID: "savings", Date: first.AddDate(0, 0, 5)},
		{ID: "zero", Amount: money.Zero("USD"), Date: first},
	}
	before := append([]domain.Transaction(nil), txns...)
	got := AgeOfMoney(txns, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if !got.Ready || got.Days != 13 || got.WindowCount != 2 || got.TotalAgedMinor != 300 || !got.WindowStart.Equal(first.AddDate(0, 0, 10)) {
		t.Fatalf("cash history adapter lost ordering, reporting parity or FX: %+v", got)
	}
	if !reflect.DeepEqual(txns, before) {
		t.Fatal("sorting cash history mutated the shared ledger")
	}
}

func TestAgeOfMoneyKeepsFullRangeCashflowAndUnavailableReason(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := AgeOfMoney([]domain.Transaction{
		{Amount: money.New(math.MaxInt64, "USD"), Date: date},
		{Amount: money.New(1, "USD"), Date: date},
		{Amount: money.New(math.MinInt64, "USD"), Date: date.AddDate(0, 0, 1)},
	}, "USD", currency.Rates{Base: "USD"})
	if got.Ready || !got.Overflow || got.WindowCount != 1 || got.TotalAgedMinor != 0 {
		t.Fatalf("large real cashflow was omitted or shown as valid: %+v", got)
	}
}

func TestAgeOfMoneyPreservesSameDayOrderAndIncompleteHistory(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	income := domain.Transaction{Amount: money.New(100, "USD"), Date: date}
	expense := domain.Transaction{Amount: money.New(-100, "USD"), Date: date}
	if got := AgeOfMoney([]domain.Transaction{income, expense}, "USD", currency.Rates{Base: "USD"}); !got.Ready || got.Days != 0 || got.TotalAgedMinor != 100 {
		t.Fatalf("same-day income before expense = %+v", got)
	}
	if got := AgeOfMoney([]domain.Transaction{expense, income}, "USD", currency.Rates{Base: "USD"}); got.Ready {
		t.Fatalf("incomplete income history incorrectly became ready: %+v", got)
	}
}
