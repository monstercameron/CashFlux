// SPDX-License-Identifier: MIT

package goalfunded

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBC6N8GoalFundedArithmeticReportsOverflow(t *testing.T) {
	t.Run("absolute minimum amount", func(t *testing.T) {
		tx := domain.Transaction{ID: "min", GoalID: "g", Date: d(2026, time.August, 3), Amount: money.New(math.MinInt64, "USD")}
		from, to := month()
		if _, err := SummarizeChecked([]domain.Transaction{tx}, nil, from, to, rates); err == nil {
			t.Fatal("expected absolute-value overflow")
		}
	})

	t.Run("sum overflow", func(t *testing.T) {
		from, to := month()
		txns := []domain.Transaction{
			spend("c", "g", d(2026, time.August, 3), math.MaxInt64),
			spend("c", "g", d(2026, time.August, 4), 1),
		}
		if _, err := SummarizeChecked(txns, nil, from, to, rates); err == nil {
			t.Fatal("expected split total overflow")
		}
		if _, err := ByGoalChecked(txns, from, to, rates); err == nil {
			t.Fatal("expected goal drawdown overflow")
		}
	})

	t.Run("remaining underflow", func(t *testing.T) {
		if _, err := RemainingMinorChecked(math.MinInt64, Drawdown{SpentMinor: 1}); err == nil {
			t.Fatal("expected remaining-balance overflow")
		}
	})
}
