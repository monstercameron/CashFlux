// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetWithPeriodBoostChecked(t *testing.T) {
	start := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	key := "2026-10-19"
	tests := []struct {
		name    string
		initial int64
		delta   int64
		want    int64
		present bool
		wantErr bool
	}{
		{name: "adds delta", initial: 100, delta: -35, want: 65, present: true},
		{name: "removes zero result", initial: 100, delta: -100},
		{name: "positive overflow", initial: math.MaxInt64, delta: 1, wantErr: true},
		{name: "negative overflow", initial: math.MinInt64, delta: -1, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			original := Budget{PeriodBoosts: map[string]int64{key: tc.initial}}
			got, err := original.WithPeriodBoostChecked(start, tc.delta)
			if tc.wantErr {
				if !errors.Is(err, money.ErrOverflow) {
					t.Fatalf("error = %v, want money.ErrOverflow", err)
				}
				if got.PeriodBoosts != nil {
					t.Fatalf("failed update returned budget: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("WithPeriodBoostChecked: %v", err)
			}
			value, present := got.PeriodBoosts[key]
			if value != tc.want || present != tc.present {
				t.Fatalf("boost = %d present=%t, want %d present=%t", value, present, tc.want, tc.present)
			}
			if original.PeriodBoosts[key] != tc.initial {
				t.Fatalf("input budget mutated: %+v", original.PeriodBoosts)
			}
		})
	}
}
