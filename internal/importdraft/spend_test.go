// SPDX-License-Identifier: MIT

package importdraft

import (
	"errors"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/spendsummary"
)

func TestMonthlySpendCheckedAndCompatible(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []extract.Row
		err  error
	}{
		{"empty", nil, nil},
		{"dates and signed amounts", []extract.Row{{Date: "2026-01-31", Amount: "$1,234.50"}, {Date: "Jan 31, 2026", Amount: "-2.00"}, {Date: "invalid", Amount: "bad"}, {Date: "2024/02/29", Amount: "0"}}, nil},
		{"matching currency", []extract.Row{{Currency: "usd", Amount: "-1"}}, nil},
		{"missing FX", []extract.Row{{Currency: "EUR", Amount: "1"}}, money.ErrCurrencyMismatch},
		{"income overflow", []extract.Row{{Amount: "92233720368547758.07"}, {Amount: "1"}}, money.ErrOverflow},
		{"expense overflow", []extract.Row{{Amount: "-92233720368547758.07"}, {Amount: "-1"}}, money.ErrOverflow},
		{"magnitude overflow", []extract.Row{{Amount: "-92233720368547758.08"}}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := MonthlySpend(tc.rows, "USD")
			if !errors.Is(got.Err, tc.err) {
				t.Fatal(got.Err)
			}
			if got.Err == nil && !reflect.DeepEqual(got.Months, spendsummary.Summarize(tc.rows, 2)) {
				t.Fatal(got.Months)
			}
			if got.Err != nil && got.Months != nil {
				t.Fatal("published invalid totals")
			}
		})
	}
	rows := make([]extract.Row, 10000)
	for i := range rows {
		rows[i] = extract.Row{Date: "2026-01-31", Amount: "-0.01"}
	}
	got := MonthlySpend(rows, "USD")
	if got.Err != nil || len(got.Months) != 1 || got.Months[0].Count != len(rows) || got.Months[0].Out != 10000 || got.Months[0].Net() != -10000 {
		t.Fatal(got)
	}
}
