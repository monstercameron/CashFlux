// SPDX-License-Identifier: MIT

package txnview_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txnview"
)

func TestTC3_5SummaryAndCalendarParityForSamePeriodIncludingExcludedRows(t *testing.T) {
	date := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	rows := []domain.Transaction{
		{ID: "spent", Date: date, Amount: money.New(-4_000, "USD")},
		{ID: "income", Date: date, Amount: money.New(9_000, "USD")},
		{ID: "excluded", Date: date, Amount: money.New(-20_000, "USD"), ExcludeFromReports: true},
		{ID: "move-out", Date: date, AccountID: "checking", TransferAccountID: "savings", Amount: money.New(-12_500, "USD")},
		{ID: "move-in", Date: date, AccountID: "savings", TransferAccountID: "checking", Amount: money.New(10_000, "EUR")},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.25}}
	summary, err := txnview.Summarize(rows, rates, "USD")
	if err != nil {
		t.Fatal(err)
	}
	month, err := txncalendar.MonthCashflow(date, time.Sunday, rows, nil, rates)
	if err != nil {
		t.Fatal(err)
	}
	var spent, income, moved int64
	var count int
	for _, week := range month.Weeks {
		for _, day := range week.Days {
			if !day.InMonth {
				continue
			}
			spent += day.Cashflow.Spent
			income += day.Cashflow.Income
			moved += day.Cashflow.Moved
			count += day.Cashflow.Count
		}
	}
	if summary.Spent != spent || summary.Income != income || summary.Transferred != moved || summary.Count != count {
		t.Fatalf("summary=%+v calendar spent=%d income=%d moved=%d count=%d", summary, spent, income, moved, count)
	}
}
