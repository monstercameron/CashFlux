// SPDX-License-Identifier: MIT

package reportview

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/safespend"
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
	balances, _ := ledger.Balances(accounts, txns)
	liquid, _ := ledger.LiquidFromBalances(accounts, balances, rates)
	toBase := safespend.ToBaseFunc(rates)
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
		mag := toBase(bal.Amount, bal.Currency)
		if mag < 0 {
			mag = -mag
		}
		if a.Type == domain.TypeCreditCard {
			cards.HasCards = true
			cards.BalanceMinor += mag
			cards.LimitMinor += toBase(a.CreditLimit.Amount, a.CreditLimit.Currency)
		}
		if mag != 0 || a.MinPayment.Amount != 0 {
			debts = append(debts, vitals.Debt{Name: a.Name, BalanceMinor: mag, AprPercent: a.RateAPROrZero(),
				MinPaymentMinor: toBase(a.MinPayment.Amount, a.MinPayment.Currency),
				IsMortgage:      a.Type == domain.TypeMortgage, InPayoff: a.IncludedInPayoff()})
		}
		rate, known := a.RateAPR()
		if !known || rate <= 0 || bal.Amount == 0 {
			continue
		}
		// Round interest in the account's currency before converting, just as
		// the row's native balance is represented in integer minor units.
		est := money.New(int64(float64(bal.Abs().Amount)*rate/100), bal.Currency)
		baseEst, err := rates.Convert(est, rates.Base)
		if err != nil {
			continue
		}
		out.Debts = append(out.Debts, InterestDebt{Name: a.Name, Balance: bal, Minimum: a.MinPayment, APR: rate, AnnualInterestMinor: baseEst.Amount})
		out.AnnualInterestMinor += baseEst.Amount
	}
	sort.SliceStable(out.Debts, func(i, j int) bool { return out.Debts[i].APR > out.Debts[j].APR })
	out.Vitals = vitals.Evaluate(vitals.Inputs{IncomeMonthlyMinor: reports.AverageMonthlyIncome(flows),
		ExpenseMonthlyMinor: reports.AverageMonthlyExpense(flows), MonthsAveraged: out.MonthsAveraged,
		EssentialMonthlyMinor: essentialMonthly, LiquidMinor: liquid.Amount, Debts: debts, Cards: cards})
	return out
}
