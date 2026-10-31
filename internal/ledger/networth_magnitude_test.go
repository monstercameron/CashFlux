// SPDX-License-Identifier: MIT

package ledger

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNetWorthLiabilityMinIntReturnsOverflowWithoutPanic(t *testing.T) {
	account := domain.Account{ID: "minimum", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(math.MinInt64, "USD")}
	rates := currency.Rates{Base: "USD"}
	for _, compute := range []func() error{
		func() error { _, err := NetWorthExplained([]domain.Account{account}, nil, rates); return err },
		func() error {
			_, err := NetWorthFromBalances([]domain.Account{account}, map[string]money.Money{account.ID: account.OpeningBalance}, rates)
			return err
		},
		func() error { _, _, _, err := NetWorth([]domain.Account{account}, nil, rates); return err },
	} {
		if err := compute(); !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("liability magnitude error = %v", err)
		}
	}
}
