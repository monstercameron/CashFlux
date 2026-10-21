// SPDX-License-Identifier: MIT

// Package healthview prepares the independent financial facts used by the
// health page's resilience scenarios. It is a native boundary between the
// ledger/report models and the rendered stress-test controls.
package healthview

import (
	"fmt"
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/resilience"
)

const resilienceLookbackMonths = 3

// ResilienceFacts contains the stress-test inputs and truthful availability for
// each independent source. Unavailable values stay zeroed and carry a reason;
// consumers must check the matching Known flag before displaying a result.
type ResilienceFacts struct {
	Input             resilience.Input
	CashFlowKnown     bool
	LiquidityKnown    bool
	CardBalancesKnown bool
	CardAPRKnown      bool
	Unavailable       map[string]error
}

// ResilienceInputs derives trailing cash flow, liquid balances and revolving
// card facts from one explicit account/transaction/rate snapshot. The monthly
// outflow uses reports' full expense total, including debt payments; minimums
// therefore remain zero in resilience.Input and are never counted twice.
func ResilienceInputs(accounts []domain.Account, transactions []domain.Transaction, rates currency.Rates, now time.Time) (ResilienceFacts, error) {
	if now.IsZero() {
		return ResilienceFacts{}, fmt.Errorf("health resilience reference time is required")
	}
	if rates.Base == "" {
		rates.Base = "USD"
	}
	facts := ResilienceFacts{Unavailable: map[string]error{}}
	fail := func(key string, err error) {
		if err != nil && facts.Unavailable[key] == nil {
			facts.Unavailable[key] = err
		}
	}

	month := dateutil.MonthStart(now)
	start := dateutil.AddMonths(month, -resilienceLookbackMonths)
	flow, err := reports.IncomeVsExpense(transactions, start, month, rates)
	if err != nil {
		fail("cash_flow", fmt.Errorf("health resilience cash flow: %w", err))
	} else {
		facts.CashFlowKnown = true
		facts.Input.MonthlyIncome = flow.Income / resilienceLookbackMonths
		facts.Input.MonthlySpend = flow.Expense / resilienceLookbackMonths
	}

	balances, _ := ledger.Balances(accounts, transactions)
	liquid, err := ledger.LiquidFromBalances(accounts, balances, rates)
	if err != nil {
		fail("liquidity", fmt.Errorf("health resilience liquid cash: %w", err))
	} else {
		facts.LiquidityKnown = true
		facts.Input.LiquidCash = liquid.Amount
	}

	var revolving int64
	var aprNumerator float64
	var aprDenominator float64
	cardBalanceErr, cardAPRErr := error(nil), error(nil)
	for _, account := range accounts {
		if account.Archived || account.Type != domain.TypeCreditCard {
			continue
		}
		balance, ok := balances[account.ID]
		if !ok {
			if cardBalanceErr == nil {
				cardBalanceErr = fmt.Errorf("account %s balance unavailable", account.ID)
			}
			if cardAPRErr == nil {
				cardAPRErr = fmt.Errorf("account %s balance unavailable for APR weighting", account.ID)
			}
			continue
		}
		owed, err := money.Abs(balance.Amount)
		if err != nil {
			if cardBalanceErr == nil {
				cardBalanceErr = fmt.Errorf("account %s balance magnitude: %w", account.ID, err)
			}
			if cardAPRErr == nil {
				cardAPRErr = fmt.Errorf("account %s balance magnitude: %w", account.ID, err)
			}
			continue
		}
		converted, err := currency.ConvertBetween(owed, balance.Currency, rates.Base, rates)
		if err != nil {
			if cardBalanceErr == nil {
				cardBalanceErr = fmt.Errorf("account %s balance conversion: %w", account.ID, err)
			}
			if cardAPRErr == nil {
				cardAPRErr = fmt.Errorf("account %s balance conversion: %w", account.ID, err)
			}
			continue
		}
		revolving, err = money.Add(revolving, converted)
		if err != nil {
			if cardBalanceErr == nil {
				cardBalanceErr = fmt.Errorf("revolving balance total: %w", err)
			}
			if cardAPRErr == nil {
				cardAPRErr = fmt.Errorf("revolving balance total: %w", err)
			}
			continue
		}
		if converted == 0 {
			continue // zero balance contributes no interest, so APR is immaterial
		}
		apr, known := account.RateAPR()
		if !known || math.IsNaN(apr) || math.IsInf(apr, 0) || apr < 0 {
			if cardAPRErr == nil {
				cardAPRErr = fmt.Errorf("account %s APR unavailable or invalid", account.ID)
			}
			continue
		}
		weightedAPR := float64(converted) * apr
		if math.IsNaN(weightedAPR) || math.IsInf(weightedAPR, 0) || math.IsNaN(aprNumerator+weightedAPR) || math.IsInf(aprNumerator+weightedAPR, 0) {
			if cardAPRErr == nil {
				cardAPRErr = fmt.Errorf("account %s weighted APR is not finite", account.ID)
			}
			continue
		}
		aprNumerator += weightedAPR
		aprDenominator += float64(converted)
	}
	if cardBalanceErr != nil {
		fail("card_balances", fmt.Errorf("health resilience card balances: %w", cardBalanceErr))
	} else {
		facts.CardBalancesKnown = true
		facts.Input.RevolvingBalance = revolving
	}
	if cardAPRErr != nil {
		fail("card_apr", fmt.Errorf("health resilience card APR: %w", cardAPRErr))
	} else {
		facts.CardAPRKnown = true
		if aprDenominator > 0 {
			facts.Input.AvgCardAPR = aprNumerator / aprDenominator
		}
	}
	return facts, nil
}
