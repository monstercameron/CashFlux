// SPDX-License-Identifier: MIT

package notificationview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNoSpendStreak(t *testing.T) {
	now := time.Date(2026, 3, 9, 8, 0, 0, 0, time.FixedZone("after DST", -4*3600))
	expense := func(offset int) domain.Transaction {
		return domain.Transaction{Date: now.AddDate(0, 0, offset), Amount: money.New(-100, "USD")}
	}
	transfer := expense(-1)
	transfer.TransferAccountID = "other"
	income := expense(-1)
	income.Amount = money.New(100, "USD")
	for _, tc := range []struct {
		name      string
		txns      []domain.Transaction
		max, want int
	}{
		{"bounded empty history", nil, 90, 90},
		{"yesterday expense", []domain.Transaction{expense(-1)}, 90, 0},
		{"today excluded", []domain.Transaction{expense(0), expense(-3)}, 90, 2},
		{"transfers and income excluded", []domain.Transaction{transfer, income, expense(-3)}, 90, 2},
		{"zero bound", nil, 0, 0},
		{"negative bound", nil, -1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NoSpendStreak(tc.txns, now, tc.max); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}
