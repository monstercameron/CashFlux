// SPDX-License-Identifier: MIT

package healthview

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestResilienceInputsUseThreeCompletedMonthsAndKnownZeroIncome(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	account := domain.Account{ID: "cash", Type: domain.TypeChecking, Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(10_000, "USD")}
	transactions := []domain.Transaction{
		{ID: "start-in", AccountID: "cash", Date: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(30_000, "USD")},
		{ID: "old-out", AccountID: "cash", Date: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), Amount: money.New(-99_000, "USD")},
		{ID: "current-in", AccountID: "cash", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(99_000, "USD")},
	}
	facts, err := ResilienceInputs([]domain.Account{account}, transactions, currency.Rates{Base: "USD"}, now)
	if err != nil || !facts.CashFlowKnown || facts.Input.MonthlyIncome != 10_000 || facts.Input.MonthlySpend != 0 {
		t.Fatal("cash flow period boundaries or known-zero spend changed", facts, err)
	}
	transactions = nil
	facts, err = ResilienceInputs([]domain.Account{account}, transactions, currency.Rates{Base: "USD"}, now)
	if err != nil || !facts.CashFlowKnown || facts.Input.MonthlyIncome != 0 || facts.Input.MonthlySpend != 0 {
		t.Fatal("no income was treated as a source failure", facts, err)
	}
}

func TestResilienceInputsKeepFXFailuresIndependent(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	cash := domain.Account{ID: "cash", Type: domain.TypeChecking, Currency: "EUR", OpeningBalance: money.New(50_000, "EUR")}
	card := domain.Account{ID: "card", Type: domain.TypeCreditCard, Class: domain.ClassLiability, Currency: "JPY", OpeningBalance: money.New(10_000, "JPY"), InterestRateAPR: domain.APR(20)}
	facts, err := ResilienceInputs([]domain.Account{cash, card}, nil, currency.Rates{Base: "USD"}, now)
	if err != nil || !facts.CashFlowKnown || facts.LiquidityKnown || facts.CardBalancesKnown || facts.CardAPRKnown || facts.Unavailable["cash_flow"] != nil || facts.Unavailable["liquidity"] == nil || facts.Unavailable["card_balances"] == nil || facts.Unavailable["card_apr"] == nil {
		t.Fatal("missing FX availability was not isolated to dependent facts", facts, err)
	}
	// A malformed card balance must not erase independent cash flow facts.
	facts, err = ResilienceInputs([]domain.Account{cash, card}, []domain.Transaction{{ID: "mismatch", AccountID: "card", Date: now, Amount: money.New(1, "EUR")}}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1, "JPY": 150}}, now)
	if err != nil || !facts.CashFlowKnown || !facts.LiquidityKnown || facts.CardBalancesKnown || facts.CardAPRKnown {
		t.Fatal("card balance failure contaminated independent facts", facts, err)
	}
}

func TestResilienceInputsZeroCardBalanceNeedsNoAPRAndOutflowCountsDebtOnce(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	cash := domain.Account{ID: "cash", Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(100_000, "USD")}
	zeroCard := domain.Account{ID: "zero", Type: domain.TypeCreditCard, Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.Zero("USD")}
	transactions := []domain.Transaction{{ID: "pay", AccountID: "cash", Date: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(-30_000, "USD")}}
	facts, err := ResilienceInputs([]domain.Account{cash, zeroCard}, transactions, currency.Rates{Base: "USD"}, now)
	if err != nil || !facts.CashFlowKnown || !facts.LiquidityKnown || !facts.CardBalancesKnown || !facts.CardAPRKnown || facts.Input.MonthlySpend != 10_000 || facts.Input.MinDebtPayments != 0 || facts.Input.RevolvingBalance != 0 || facts.Input.AvgCardAPR != 0 {
		t.Fatal("zero-balance card or included debt outflow semantics changed", facts, err)
	}

	positiveCard := zeroCard
	positiveCard.ID = "positive"
	positiveCard.OpeningBalance = money.New(20_000, "USD")
	facts, err = ResilienceInputs([]domain.Account{positiveCard}, nil, currency.Rates{Base: "USD"}, now)
	if err != nil || facts.CardBalancesKnown != true || facts.CardAPRKnown || facts.Unavailable["card_apr"] == nil {
		t.Fatal("positive card balance without APR became a known zero APR", facts, err)
	}

	invalidAPR := domain.APR(math.Inf(1))
	positiveCard.InterestRateAPR = invalidAPR
	facts, err = ResilienceInputs([]domain.Account{positiveCard}, nil, currency.Rates{Base: "USD"}, now)
	if err != nil || facts.CardAPRKnown || facts.Unavailable["card_apr"] == nil {
		t.Fatal("non-finite APR became available", facts, err)
	}
}
