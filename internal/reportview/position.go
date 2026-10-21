// SPDX-License-Identifier: MIT

package reportview

import (
	"fmt"
	"math"
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/vitals"
)

// InterestDebt is one interest-bearing liability. Balance and Minimum retain
// native currencies; AnnualInterestMinor is converted into the report's base
// currency so rows, bars and the household interest total are comparable.
type InterestDebt struct {
	Name                string
	Balance, Minimum    money.Money
	APR                 float64
	AnnualInterestMinor int64
}

// Position is the household position snapshot and the interest-cost table.
type Position struct {
	Vitals              vitals.Result
	MonthsAveraged      int
	Debts               []InterestDebt
	AnnualInterestMinor int64
	// Error makes the position unavailable. No partial financial totals are kept.
	Error error
}

// BuildPosition calculates current household balances once for both the vitals
// and debt sections. flows must be the household series returned by BuildSeries;
// the last six buckets supply income and expense capacity, preserving the
// report's chosen historical window. essentialMonthly is in base minor units.
func BuildPosition(accounts []domain.Account, txns []domain.Transaction, flows []reports.PeriodFlow, rates currency.Rates, essentialMonthly int64) Position {
	if len(flows) > 6 {
		flows = flows[len(flows)-6:]
	}
	out := Position{MonthsAveraged: reports.ActiveMonths(flows)}
	fail := func(operation string, err error) Position {
		return Position{Error: fmt.Errorf("report position %s: %w", operation, err)}
	}
	var active []domain.Account
	for _, a := range accounts {
		if !a.Archived {
			active = append(active, a)
		}
	}
	balances, err := ledger.Balances(active, txns)
	if err != nil {
		return fail("balances", err)
	}
	liquid, err := ledger.LiquidFromBalances(active, balances, rates)
	if err != nil {
		return fail("liquid balance", err)
	}
	toBase := func(value money.Money, fallback string) (int64, error) {
		if value.Currency == "" {
			value = money.New(value.Amount, fallback)
		}
		converted, err := rates.Convert(value, rates.Base)
		return converted.Amount, err
	}
	var debts []vitals.Debt
	cards := vitals.Cards{}
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		bal, ok := balances[a.ID]
		if !ok {
			continue
		}
		converted, err := toBase(bal, a.Currency)
		if err != nil {
			return fail("debt conversion", err)
		}
		mag, err := money.Abs(converted)
		if err != nil {
			return fail("debt magnitude", err)
		}
		minimum, err := toBase(a.MinPayment, a.Currency)
		if err != nil {
			return fail("minimum payment conversion", err)
		}
		if a.Type == domain.TypeCreditCard {
			cards.HasCards = true
			cards.BalanceMinor, err = money.Add(cards.BalanceMinor, mag)
			if err != nil {
				return fail("card balances", err)
			}
			limit, err := toBase(a.CreditLimit, a.Currency)
			if err != nil {
				return fail("credit limit conversion", err)
			}
			cards.LimitMinor, err = money.Add(cards.LimitMinor, limit)
			if err != nil {
				return fail("credit limits", err)
			}
		}
		if mag != 0 || a.MinPayment.Amount != 0 {
			debts = append(debts, vitals.Debt{Name: a.Name, BalanceMinor: mag, AprPercent: a.RateAPROrZero(),
				MinPaymentMinor: minimum,
				IsMortgage:      a.Type == domain.TypeMortgage, InPayoff: a.IncludedInPayoff()})
		}
		rate, known := a.RateAPR()
		if math.IsNaN(rate) || math.IsInf(rate, 0) {
			return fail("interest rate", money.ErrOverflow)
		}
		if !known || rate <= 0 || bal.Amount == 0 {
			continue
		}
		// Round interest in the account's currency before converting, just as
		// the row's native balance is represented in integer minor units.
		nativeMagnitude, err := money.Abs(bal.Amount)
		if err != nil {
			return fail("native debt magnitude", err)
		}
		estimate := float64(nativeMagnitude) * rate / 100
		if math.IsNaN(estimate) || math.IsInf(estimate, 0) || estimate >= float64(math.MaxInt64) {
			return fail("annual interest", money.ErrOverflow)
		}
		est := money.New(int64(estimate), bal.Currency)
		baseEst, err := rates.Convert(est, rates.Base)
		if err != nil {
			return fail("annual interest conversion", err)
		}
		out.Debts = append(out.Debts, InterestDebt{Name: a.Name, Balance: bal, Minimum: a.MinPayment, APR: rate, AnnualInterestMinor: baseEst.Amount})
		out.AnnualInterestMinor, err = money.Add(out.AnnualInterestMinor, baseEst.Amount)
		if err != nil {
			return fail("annual interest total", err)
		}
	}
	sort.SliceStable(out.Debts, func(i, j int) bool { return out.Debts[i].APR > out.Debts[j].APR })
	income, expense, months, err := reports.MonthlyAveragesChecked(flows)
	if err != nil {
		return fail("monthly averages", err)
	}
	out.MonthsAveraged = months
	out.Vitals, err = vitals.EvaluateChecked(vitals.Inputs{IncomeMonthlyMinor: income,
		ExpenseMonthlyMinor: expense, MonthsAveraged: months,
		EssentialMonthlyMinor: essentialMonthly, LiquidMinor: liquid.Amount, Debts: debts, Cards: cards})
	if err != nil {
		return fail("vitals", err)
	}
	return out
}
