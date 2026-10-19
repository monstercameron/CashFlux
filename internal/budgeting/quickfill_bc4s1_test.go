// SPDX-License-Identifier: MIT

package budgeting

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBC4QuickFillPriorLimitUsesHistoryAndBoost(t *testing.T) {
	for _, tc := range []struct {
		name    string
		history map[string]int64
		boost   int64
		want    int64
		present bool
	}{
		{"history", map[string]int64{"2026-09-01": 30000, "2026-10-01": 40000}, 5000, 35000, true},
		{"legacy", nil, 5000, 45000, true},
		{"overflow", nil, math.MaxInt64, 0, false},
		{"invalid history", map[string]int64{"bad": 30000}, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := domain.Budget{Period: domain.PeriodMonthly, Limit: money.New(40000, "USD"), LimitHistory: tc.history, PeriodBoosts: map[string]int64{"2026-09-01": tc.boost}}
			chips, _ := QuickFills(b, nil, QuickFillInput{Now: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), Rates: currency.Rates{Base: "USD"}})
			found := false
			for _, c := range chips {
				if c.Key == QuickFillPriorLimit {
					found = true
					if c.Amount.Amount != tc.want {
						t.Fatalf("prior=%v want %d", c.Amount, tc.want)
					}
				}
			}
			if found != tc.present {
				t.Fatalf("chip present=%v want %v", found, tc.present)
			}
		})
	}
}
