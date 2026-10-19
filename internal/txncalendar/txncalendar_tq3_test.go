// SPDX-License-Identifier: MIT

package txncalendar

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestMonthCashflow(t *testing.T) {
	anchor := mustDate("2026-10-15")
	cases := []struct {
		name       string
		txns       []domain.Transaction
		rates      currency.Rates
		day        string
		want       Cashflow
		weekOf     string
		wantWeek   Cashflow
		wantIssues int
		wantErr    error
	}{
		{
			name: "transfer is movement, not spending",
			txns: []domain.Transaction{
				{ID: "sweep", Date: mustDate("2026-10-12"), Amount: money.New(-50000, "USD"), TransferAccountID: "savings"},
				{ID: "groceries", Date: mustDate("2026-10-12"), Amount: money.New(-2500, "USD")},
				{ID: "pay", Date: mustDate("2026-10-13"), Amount: money.New(10000, "USD")},
				{ID: "refund", Date: mustDate("2026-10-13"), Amount: money.New(500, "USD")},
				{ID: "outside", Date: mustDate("2026-11-01"), Amount: money.New(-900, "USD")},
			},
			rates:    currency.Rates{Base: "USD"},
			day:      "2026-10-12",
			want:     Cashflow{Spent: 2500, Count: 2, Moved: 50000, Net: -2500},
			weekOf:   "2026-10-12",
			wantWeek: Cashflow{Spent: 2500, Income: 10500, Count: 4, Moved: 50000, Net: 8000},
		},
		{
			name:  "convert foreign currency into base",
			txns:  []domain.Transaction{{ID: "eur", Date: mustDate("2026-10-05"), Amount: money.New(-1000, "EUR")}},
			rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}},
			day:   "2026-10-05", want: Cashflow{Spent: 1500, Count: 1, Net: -1500},
			weekOf: "2026-10-05", wantWeek: Cashflow{Spent: 1500, Count: 1, Net: -1500},
		},
		{
			name:  "missing rate is reported and transaction count remains",
			txns:  []domain.Transaction{{ID: "unknown", Date: mustDate("2026-10-06"), Amount: money.New(-1200, "CAD")}},
			rates: currency.Rates{Base: "USD"},
			day:   "2026-10-06", want: Cashflow{Count: 1},
			weekOf: "2026-10-05", wantWeek: Cashflow{Count: 1}, wantIssues: 1,
		},
		{
			name: "checked spending sum catches overflow",
			txns: []domain.Transaction{
				{ID: "large-a", Date: mustDate("2026-10-01"), Amount: money.New(math.MinInt64+1, "USD")},
				{ID: "large-b", Date: mustDate("2026-10-01"), Amount: money.New(-10, "USD")},
			},
			rates: currency.Rates{Base: "USD"}, wantErr: money.ErrOverflow,
		},
		{
			name: "checked week total catches overflow across days",
			txns: []domain.Transaction{
				{ID: "day-one", Date: mustDate("2026-10-05"), Amount: money.New(-(math.MaxInt64/2 + 1), "USD")},
				{ID: "day-two", Date: mustDate("2026-10-06"), Amount: money.New(-(math.MaxInt64/2 + 1), "USD")},
			},
			rates: currency.Rates{Base: "USD"}, wantErr: money.ErrOverflow,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MonthCashflow(anchor, time.Monday, tc.txns, nil, tc.rates)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("MonthCashflow error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got.RateIssues) != tc.wantIssues {
				t.Fatalf("rate issues = %d, want %d: %+v", len(got.RateIssues), tc.wantIssues, got.RateIssues)
			}
			var day Cashflow
			foundDay := false
			var week CashflowWeek
			foundWeek := false
			for _, candidate := range got.Weeks {
				if DayKey(candidate.Days[0].Date) == tc.weekOf {
					week, foundWeek = candidate, true
				}
				for _, cell := range candidate.Days {
					if DayKey(cell.Date) == tc.day {
						day, foundDay = cell.Cashflow, true
					}
				}
			}
			if !foundDay || day != tc.want {
				t.Errorf("day %s = %+v (found %v), want %+v", tc.day, day, foundDay, tc.want)
			}
			if !foundWeek || week.Total != tc.wantWeek {
				t.Errorf("week %s = %+v (found %v), want %+v", tc.weekOf, week.Total, foundWeek, tc.wantWeek)
			}
		})
	}
}
