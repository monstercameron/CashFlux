// SPDX-License-Identifier: MIT

package reportview

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPositionRejectsOverflowingAggregatesAndInterest(t *testing.T) {
	card := func(id string, balance, limit, minimum int64, rate float64) domain.Account {
		return domain.Account{ID: id, Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
			OpeningBalance: money.New(-balance, "USD"), CreditLimit: money.New(limit, "USD"), MinPayment: money.New(minimum, "USD"), InterestRateAPR: domain.APR(rate)}
	}
	for _, tc := range []struct {
		name     string
		accounts []domain.Account
	}{
		{"card balances", []domain.Account{card("a", math.MaxInt64/2+1, 0, 0, 0), card("b", math.MaxInt64/2+1, 0, 0, 0)}},
		{"card limits", []domain.Account{card("a", 0, math.MaxInt64/2+1, 0, 0), card("b", 0, math.MaxInt64/2+1, 0, 0)}},
		{"annual interest", []domain.Account{card("a", math.MaxInt64/4, 0, 0, 300), card("b", math.MaxInt64/4, 0, 0, 300)}},
		{"finite interest overflow", []domain.Account{card("a", 100, 0, 0, math.MaxFloat64)}},
		{"minimum service", []domain.Account{card("a", 100, 1000, math.MaxInt64/12+1, 0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildPosition(tc.accounts, nil, nil, currency.Rates{Base: "USD"}, 0)
			if !errors.Is(got.Error, money.ErrOverflow) || len(got.Debts) != 0 || got.AnnualInterestMinor != 0 || got.Vitals.HasDebts || got.Vitals.HasCards {
				t.Fatalf("position exposes partial or wrapped readings: %+v", got)
			}
		})
	}
}
