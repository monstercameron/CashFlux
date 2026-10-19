// SPDX-License-Identifier: MIT

package ledger

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAdjustmentToTargetRejectsOverflow(t *testing.T) {
	tests := []struct {
		name    string
		current int64
		target  int64
	}{
		{name: "positive delta overflow", current: -1, target: math.MaxInt64},
		{name: "negative delta overflow", current: 1, target: math.MinInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjustment, ok := AdjustmentToTarget(money.New(tt.current, "USD"), tt.target)
			if ok || adjustment != (money.Money{}) {
				t.Fatalf("AdjustmentToTarget(%d, %d) = (%v, %v), want rejected zero adjustment", tt.current, tt.target, adjustment, ok)
			}
		})
	}
}
