// SPDX-License-Identifier: MIT

package afford

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCanAffordCheckedPreservesOrdinaryProjection(t *testing.T) {
	for _, tc := range []struct {
		amount, start, net, reserve int64
		months                      int
	}{
		{500, 1000, 100, 100, 0}, {2000, 1000, 100, 100, 3}, {2000, 1000, 0, 0, 4}, {2000, 1000, -100, 0, 4}, {1, 10, 2, 0, -2},
	} {
		got, err := CanAffordChecked(tc.amount, tc.start, tc.net, tc.months, tc.reserve)
		if err != nil {
			t.Fatal(err)
		}
		want := CanAfford(tc.amount, tc.start, tc.net, tc.months, tc.reserve)
		if got != want {
			t.Fatalf("checked=%+v ordinary=%+v", got, want)
		}
	}
}

func TestCanAffordCheckedRejectsWrappedMoneyAndNarrowedMonthCount(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		amount, start, net, reserve int64
		months                      int
	}{
		{"flow product", 1, 0, math.MaxInt64, 0, 2},
		{"projected sum", 1, math.MaxInt64, 1, 0, 1},
		{"reserve subtraction", 1, math.MinInt64, 0, 1, 0},
		{"shortfall", math.MaxInt64, -1, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CanAffordChecked(tc.amount, tc.start, tc.net, tc.months, tc.reserve)
			if !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	got, err := CanAffordChecked(math.MaxInt64, math.MaxInt64, 0, 0, 1)
	if err != nil || got.Affordable || got.Available != math.MaxInt64-1 || got.Shortfall != 1 {
		t.Fatalf("finite boundary=%+v %v", got, err)
	}
	// This assertion runs on both native and WASM: a wide positive month count
	// either remains exact or explicitly fails, never wraps into a negative int.
	got, err = CanAffordChecked(math.MaxInt64, 0, 1, 0, 0)
	if int64(^uint(0)>>1) < math.MaxInt64 {
		if !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("narrow target accepted months: %+v %v", got, err)
		}
	} else if err != nil || int64(got.MonthsNeeded) != math.MaxInt64 {
		t.Fatalf("wide target=%+v %v", got, err)
	}
}
