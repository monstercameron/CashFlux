// SPDX-License-Identifier: MIT

package ledger

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPeriodTotalsMinInt64ExpenseReturnsOverflow(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	income, expense, err := PeriodTotals([]domain.Transaction{{Date: start, Amount: money.New(math.MinInt64, "USD")}}, start, start.AddDate(0, 1, 0), currency.Rates{Base: "USD"})
	if !errors.Is(err, money.ErrOverflow) || income != (money.Money{}) || expense != (money.Money{}) {
		t.Fatalf("totals=%+v/%+v err=%v", income, expense, err)
	}
}
