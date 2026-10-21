// SPDX-License-Identifier: MIT
package reports

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"math"
	"testing"
	"time"
)

func statsCheckedRow(id string, amount int64, code string, on time.Time) domain.Transaction {
	return domain.Transaction{ID: id, Desc: "Shop", Date: on, Amount: money.New(amount, code)}
}
func TestSpendingStatsCheckedErrorsHaveNoPartialResult(t *testing.T) {
	start, end := dt(2026, 6, 1), dt(2026, 7, 1)
	for _, tc := range []struct {
		name  string
		rows  []domain.Transaction
		rates currency.Rates
		want  error
	}{
		{"minimum magnitude", []domain.Transaction{statsCheckedRow("minimum", math.MinInt64, "USD", start)}, usdRates(), money.ErrOverflow},
		{"sum overflow", []domain.Transaction{statsCheckedRow("maximum", -math.MaxInt64, "USD", start), statsCheckedRow("one", -1, "USD", start)}, usdRates(), money.ErrOverflow},
		{"unknown FX", []domain.Transaction{statsCheckedRow("foreign", -1, "EUR", start)}, usdRates(), currency.ErrUnknownRate},
		{"FX overflow", []domain.Transaction{statsCheckedRow("foreign", -math.MaxInt64, "EUR", start)}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, money.ErrOverflow},
		{"invalid FX", []domain.Transaction{statsCheckedRow("foreign", -1, "EUR", start)}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": math.NaN()}}, currency.ErrInvalidRate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SpendingStats(tc.rows, start, end, tc.rates)
			if got != (SpendStats{}) || !errors.Is(err, tc.want) {
				t.Fatalf("got %+v error%v want%v", got, err, tc.want)
			}
		})
	}
}
func TestSpendingStatsCheckedBoundariesAndExclusions(t *testing.T) {
	start, end := dt(2026, 6, 1), dt(2026, 7, 1)
	got, err := SpendingStats([]domain.Transaction{statsCheckedRow("maximum", -math.MaxInt64, "USD", start)}, start, end, usdRates())
	if err != nil || got != (SpendStats{Count: 1, Total: math.MaxInt64, Average: math.MaxInt64, Median: math.MaxInt64}) {
		t.Fatalf("valid maximum %+v %v", got, err)
	}
	a, b := int64(math.MaxInt64/2), int64(math.MaxInt64/2-1)
	got, err = SpendingStats([]domain.Transaction{statsCheckedRow("a", -a, "USD", start), statsCheckedRow("b", -b, "USD", start)}, start, end, usdRates())
	if err != nil || got.Total != a+b || got.Average != (a+b)/2 || got.Median != (a+b)/2 {
		t.Fatalf("large even median %+v %v", got, err)
	}
	valid := statsCheckedRow("valid", -101, "USD", start)
	rows := []domain.Transaction{valid, statsCheckedRow("before", math.MinInt64, "USD", start.Add(-time.Nanosecond)), statsCheckedRow("end", math.MinInt64, "USD", end)}
	excluded := statsCheckedRow("excluded", math.MinInt64, "USD", start)
	excluded.ExcludeFromReports = true
	rows = append(rows, excluded)
	transfer := statsCheckedRow("transfer", math.MinInt64, "USD", start)
	transfer.TransferAccountID = "other"
	rows = append(rows, transfer)
	got, err = SpendingStats(rows, start, end, usdRates())
	if err != nil || got != (SpendStats{Count: 1, Total: 101, Average: 101, Median: 101}) {
		t.Fatalf("exclusions changed %+v %v", got, err)
	}
}
func TestSpendingStatsCheckedMixedFXHalfOpenAndRounding(t *testing.T) {
	start, end := dt(2026, 6, 1), dt(2026, 7, 1)
	rows := []domain.Transaction{statsCheckedRow("start", -101, "USD", start), statsCheckedRow("euro", -101, "EUR", start.Add(time.Hour)), statsCheckedRow("end", -99999, "USD", end)}
	got, err := SpendingStats(rows, start, end, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}})
	if err != nil || got != (SpendStats{Count: 2, Total: 253, Average: 126, Median: 126}) {
		t.Fatalf("mixed FX %+v %v", got, err)
	}
}
