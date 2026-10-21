// SPDX-License-Identifier: MIT

package goalcompare

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPrepareMonthlyPool(t *testing.T) {
	tests := []struct {
		name      string
		a, b      money.Money
		known     bool
		available bool
		amount    int64
		currency  string
		err       error
	}{
		{name: "pooled plans", a: money.New(400, "USD"), b: money.New(600, "USD"), known: true, available: true, amount: 1000, currency: "USD"},
		{name: "known zero", a: money.Zero("USD"), b: money.Zero("USD"), known: true, available: true, currency: "USD"},
		{name: "missing plan stays unavailable", a: money.New(400, "USD"), b: money.Zero("USD"), err: ErrMonthlyPlansUnavailable},
		{name: "currency mismatch", a: money.New(400, "USD"), b: money.New(600, "EUR"), known: true, err: ErrMonthlyCurrencyMismatch},
		{name: "positive overflow", a: money.New(math.MaxInt64, "USD"), b: money.New(1, "USD"), known: true, err: money.ErrOverflow},
		{name: "negative overflow", a: money.New(math.MinInt64, "USD"), b: money.New(-1, "USD"), known: true, err: money.ErrOverflow},
		{name: "representable negative endpoint", a: money.New(math.MinInt64, "USD"), b: money.Zero("USD"), known: true, available: true, amount: math.MinInt64, currency: "USD"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := PrepareMonthlyPool(test.a, test.b, test.known)
			if got.Available != test.available || got.Amount.Amount != test.amount || got.Amount.Currency != test.currency {
				t.Fatalf("pool = %+v", got)
			}
			if !errors.Is(got.Err, test.err) {
				t.Fatalf("error = %v, want %v", got.Err, test.err)
			}
		})
	}
}
