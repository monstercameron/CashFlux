// SPDX-License-Identifier: MIT

package assistantview

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSpendTrendUsesSixCompleteCalendarMonths(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		questionTxn("too old", "", "USD", -900000, time.Date(2025, 8, 31, 0, 0, 0, 0, time.UTC)),
		questionTxn("oldest", "", "USD", -600, time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)),
		questionTxn("latest", "", "USD", -1200, time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)),
		questionTxn("current", "", "USD", -900000, now),
	}
	got, err := SpendTrend(txns, currency.Rates{Base: "USD"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Values, []float64{600, 0, 0, 0, 0, 1200}) || !reflect.DeepEqual(got.Labels, []string{"Sep", "Oct", "Nov", "Dec", "Jan", "Feb"}) || got.Average != 300 || got.Delta != 900 || got.LastMonth != "February" {
		t.Fatalf("incorrect trend: %+v", got)
	}
}

func TestSpendTrendAverageDoesNotOverflowAcrossMonths(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	var txns []domain.Transaction
	for i := -6; i < 0; i++ {
		txns = append(txns, questionTxn("large", "", "USD", -math.MaxInt64, dateutil.AddMonths(dateutil.MonthStart(now), i)))
	}
	got, err := SpendTrend(txns, currency.Rates{Base: "USD"}, now)
	if err != nil || got.Average != math.MaxInt64 || got.Delta != 0 {
		t.Fatalf("average overflow: avg=%d delta=%d err=%v", got.Average, got.Delta, err)
	}
}

func TestSpendTrendMissingFXReturnsError(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	_, err := SpendTrend([]domain.Transaction{questionTxn("foreign", "", "EUR", -100, dateutil.AddMonths(now, -1))}, currency.Rates{Base: "USD"}, now)
	if err == nil {
		t.Fatal("missing rate must not generate a base-currency trend")
	}
}
