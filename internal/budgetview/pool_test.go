// SPDX-License-Identifier: MIT

package budgetview

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPreparePoolKnownResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  PoolInput
		amount int64
		has    bool
	}{
		{name: "zero-based includes rollover and fixed assignments", input: PoolInput{Method: budgeting.MethodZeroBased, Expected: 100, Rollover: 20, Budgeted: 80, Fixed: 10, PlanKnown: true}, amount: 30, has: true},
		{name: "simple excludes rollover and fixed assignments", input: PoolInput{Method: budgeting.MethodSimple, Expected: 100, Rollover: 50, Budgeted: 125, Fixed: 25, PlanKnown: true}, amount: -25, has: true},
		{name: "zero is a known pool", input: PoolInput{Method: budgeting.MethodZeroBased, Expected: 100, Budgeted: 100, PlanKnown: true}, amount: 0, has: true},
		{name: "envelope has no shared pool", input: PoolInput{Method: budgeting.MethodEnvelope}, amount: 0, has: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PreparePool(tc.input)
			if !got.Known || got.Error != nil || got.HasPool != tc.has || got.Amount != tc.amount {
				t.Fatalf("PreparePool = %+v, want known amount=%d hasPool=%t", got, tc.amount, tc.has)
			}
		})
	}
}

func TestPreparePoolPreservesUnavailableAndOverflow(t *testing.T) {
	for _, tc := range []struct {
		name         string
		input        PoolInput
		wantOverflow bool
	}{
		{name: "zero-based plan unavailable", input: PoolInput{Method: budgeting.MethodZeroBased, Expected: 100}},
		{name: "zero-based income and rollover overflow", input: PoolInput{Method: budgeting.MethodZeroBased, Expected: math.MaxInt64, Rollover: 1, Budgeted: 0, PlanKnown: true}, wantOverflow: true},
		{name: "zero-based assignment overflow", input: PoolInput{Method: budgeting.MethodZeroBased, Expected: 10, Budgeted: math.MaxInt64, Fixed: 1, PlanKnown: true}, wantOverflow: true},
		{name: "zero-based subtraction overflow", input: PoolInput{Method: budgeting.MethodZeroBased, Expected: math.MaxInt64, Budgeted: math.MinInt64, PlanKnown: true}, wantOverflow: true},
		{name: "simple subtraction overflow", input: PoolInput{Method: budgeting.MethodSimple, Expected: math.MaxInt64, Budgeted: -1, PlanKnown: true}, wantOverflow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PreparePool(tc.input)
			if got.Known || got.HasPool || got.Error == nil {
				t.Fatalf("PreparePool = %+v, want explicit unavailable result", got)
			}
			if errors.Is(got.Error, money.ErrOverflow) != tc.wantOverflow {
				t.Fatalf("PreparePool error = %v, overflow=%t want %t", got.Error, errors.Is(got.Error, money.ErrOverflow), tc.wantOverflow)
			}
		})
	}
}
