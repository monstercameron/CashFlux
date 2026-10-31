// SPDX-License-Identifier: MIT

package periodview

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTotalsDistinguishKnownZeroFromUnknownAndConvertExactly(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	if (Totals{}).Known || (Totals{}).SavingsKnown() {
		t.Fatal("empty result claimed valid zero")
	}
	zero := BuildTotals(nil, start, end, rates)
	if !zero.Known || zero.Error != nil || !zero.SavingsKnown() || zero.Income != money.Zero("USD") || zero.Net != money.Zero("USD") {
		t.Fatalf("real empty period=%+v", zero)
	}
	rows := []domain.Transaction{{ID: "income", Date: start, Amount: money.New(10000, "USD")}, {ID: "expense", Date: start, Amount: money.New(-2000, "EUR")}}
	got := BuildTotals(rows, start, end, rates)
	if !got.Known || got.Error != nil || got.Income.Amount != 10000 || got.Expense.Amount != 4000 || got.Net.Amount != 6000 || got.SavingsPercent != 60 {
		t.Fatalf("converted totals=%+v", got)
	}
	// Unknown totals retain neither a successful prefix nor invented base-currency
	// zeroes. Restoring the rate recovers a known result without altering rows.
	rates.Rates = nil
	missing := BuildTotals(rows, start, end, rates)
	if missing.Known || !errors.Is(missing.Error, currency.ErrUnknownRate) || missing.SavingsKnown() || missing.Income != (money.Money{}) || missing.Expense != (money.Money{}) || missing.Net != (money.Money{}) {
		t.Fatalf("missing FX looked usable: %+v", missing)
	}
	rates.Rates = map[string]float64{"EUR": 2}
	if recovered := BuildTotals(rows, start, end, rates); !recovered.Known || recovered.Net != got.Net {
		t.Fatalf("FX recovery=%+v", recovered)
	}
}

func TestTotalsPropagateOverflowAndKeepValidMoneyWhenOnlyRateOverflows(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rates := currency.Rates{Base: "USD"}
	for _, rows := range [][]domain.Transaction{
		{{Date: start, Amount: money.New(math.MaxInt64, "USD")}, {Date: start, Amount: money.New(1, "USD")}},
		{{Date: start, Amount: money.New(math.MinInt64, "USD")}},
	} {
		got := BuildTotals(rows, start, end, rates)
		if got.Known || !errors.Is(got.Error, money.ErrOverflow) {
			t.Fatalf("overflow totals=%+v", got)
		}
	}
	large := BuildTotals([]domain.Transaction{{Date: start, Amount: money.New(math.MaxInt64, "USD")}, {Date: start, Amount: money.New(-1, "USD")}}, start, end, rates)
	if !large.Known || !large.SavingsKnown() || large.SavingsPercent != 99 {
		t.Fatalf("representable rate overflowed intermediate: %+v", large)
	}
	extreme := BuildTotals([]domain.Transaction{{Date: start, Amount: money.New(1, "USD")}, {Date: start, Amount: money.New(-math.MaxInt64, "USD")}}, start, end, rates)
	if !extreme.Known || extreme.Net.Amount != 1-math.MaxInt64 || extreme.SavingsKnown() || !errors.Is(extreme.SavingsError, money.ErrOverflow) {
		t.Fatalf("rate-only overflow=%+v", extreme)
	}
}

func TestTotalsSavingsPercentageRetainsWideSignedValue(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	got := BuildTotals([]domain.Transaction{{Date: start, Amount: money.New(1, "USD")}, {Date: start, Amount: money.New(-30000000, "USD")}}, start, start.AddDate(0, 1, 0), currency.Rates{Base: "USD"})
	if !got.SavingsKnown() || got.SavingsPercent != -2999999900 {
		t.Fatalf("wide savings rate narrowed: %+v", got)
	}
}
