// SPDX-License-Identifier: MIT

package budgeting

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBC6N8CheckedArithmeticRejectsOverflow(t *testing.T) {
	t.Run("auto-budget plan sum", func(t *testing.T) {
		budgets := []domain.Budget{
			monthlyBudget("a", "a", math.MaxInt64),
			monthlyBudget("b", "b", 1),
		}
		if _, err := AutoBudgetImpactOfChecked(budgets, 0, nil); err == nil {
			t.Fatal("expected current-plan overflow")
		}
	})

	t.Run("bill fit sum", func(t *testing.T) {
		if _, err := FitBillChecked(10, math.MaxInt64, 1); err == nil {
			t.Fatal("expected booked spend overflow")
		}
		got := FitBill(math.MaxInt64, math.MaxInt64, 1)
		if got.Fits || got.OverBy != math.MaxInt64 {
			t.Fatalf("overflow fallback = %+v", got)
		}
	})

	t.Run("progress percentage", func(t *testing.T) {
		if _, _, err := CommittedPercentagesChecked(1, math.MaxInt64, 0); err == nil {
			t.Fatal("expected percentage overflow")
		}
	})

	t.Run("checked impact difference", func(t *testing.T) {
		if _, err := (AutoBudgetImpact{BudgetedAfter: math.MaxInt64, BudgetedBefore: -1}).ChangeChecked(); err == nil {
			t.Fatal("expected change overflow")
		}
	})

	t.Run("prior spend arithmetic", func(t *testing.T) {
		if _, err := ComparePriorSpendChecked(math.MinInt64, math.MaxInt64); err == nil {
			t.Fatal("expected prior spend delta overflow")
		}
	})

	t.Run("target ceil division", func(t *testing.T) {
		from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		deadline := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
		got := byDateMonthly(money.New(math.MaxInt64, "USD"), money.Zero("USD"), from, deadline)
		if want := int64(math.MaxInt64/2 + 1); got.Amount != want {
			t.Fatalf("monthly target pace = %d, want %d", got.Amount, want)
		}
	})

	t.Run("true-up rounding", func(t *testing.T) {
		if _, err := roundToChecked(math.MaxInt64, 2); err == nil {
			t.Fatal("expected rounded true-up overflow")
		}
	})

	t.Run("money values remain integral", func(t *testing.T) {
		got := averagePeriods([]money.Money{money.New(math.MaxInt64, "USD"), money.New(math.MaxInt64, "USD")}, "USD")
		if got.Amount != math.MaxInt64 {
			t.Fatalf("average near limit = %d, want %d", got.Amount, int64(math.MaxInt64))
		}
	})
}
