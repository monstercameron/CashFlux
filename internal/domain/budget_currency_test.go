// SPDX-License-Identifier: MIT

package domain

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetWithBaseCurrency(t *testing.T) {
	for _, tc := range []struct{ base, cur, want string }{{"EUR", "", "EUR"}, {"USD", "", "USD"}, {"", "", "USD"}, {"USD", "JPY", "JPY"}} {
		t.Run(tc.base+tc.cur, func(t *testing.T) {
			b := Budget{Limit: money.New(600000, tc.cur), TargetKind: TargetSetAside, TargetAmount: money.New(100, tc.cur)}
			got := b.WithBaseCurrency(tc.base)
			if got.Limit.Currency != tc.want || got.TargetAmount.Currency != tc.want || got.Limit.Amount != 600000 || b.Limit.Currency != tc.cur {
				t.Fatalf("resolved=%+v original=%+v", got, b)
			}
		})
	}
}
