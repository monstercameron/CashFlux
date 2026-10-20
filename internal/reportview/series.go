// SPDX-License-Identifier: MIT

// Package reportview builds unformatted report models shared by the report
// sections. Its inputs include scope and dates explicitly; no browser state or
// wall clock is read while calculating figures.
package reportview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
)

// Series holds the scoped cash-flow and credit series, alongside a distinct
// household flow basis for household-wide position and capacity figures.
type Series struct {
	AccountIDs     []string
	Accounts       []domain.Account
	Transactions   []domain.Transaction
	Flow           reports.PeriodFlow
	MonthFlows     []reports.PeriodFlow
	HouseholdFlows []reports.PeriodFlow
	SavingsRates   []int
	Credit         []int
	FlowError      error
	HouseholdError error
}

// BuildSeries resolves one account scope for every series. Position figures
// intentionally keep a household flow basis, matching their household cash and
// debt balances. With no active scope this reuses MonthFlows without another
// ledger scan. bounds are consecutive half-open reporting buckets.
func BuildSeries(accounts []domain.Account, txns []domain.Transaction, sc scope.ReportScope, bounds []time.Time, rates currency.Rates) Series {
	ids := scope.ResolveScope(accounts, sc, func(a domain.Account) string { return a.Institution })
	v := Series{AccountIDs: ids, Accounts: scope.ApplyScopeToAccounts(accounts, ids), Transactions: scope.ApplyScopeToTxns(txns, ids)}
	v.MonthFlows, v.FlowError = reports.IncomeExpenseSeries(v.Transactions, bounds, rates)
	if len(bounds) >= 2 {
		v.Flow.Start, v.Flow.End = bounds[0], bounds[len(bounds)-1]
	}
	for _, f := range v.MonthFlows {
		income, err := money.New(v.Flow.Income, rates.Base).Add(money.New(f.Income, rates.Base))
		if err != nil {
			v.FlowError = err
			break
		}
		expense, err := money.New(v.Flow.Expense, rates.Base).Add(money.New(f.Expense, rates.Base))
		if err != nil {
			v.FlowError = err
			break
		}
		v.Flow.Income, v.Flow.Expense = income.Amount, expense.Amount
	}
	if v.FlowError != nil {
		v.Flow.Income, v.Flow.Expense = 0, 0
	}
	v.SavingsRates = reports.SavingsRates(v.MonthFlows)
	v.HouseholdFlows, v.HouseholdError = v.MonthFlows, v.FlowError
	if !sc.IsAll() {
		allIDs := scope.ResolveScope(accounts, scope.ReportScope{}, func(a domain.Account) string { return a.Institution })
		householdTxns := scope.ApplyScopeToTxns(txns, allIDs)
		v.HouseholdFlows, v.HouseholdError = reports.IncomeExpenseSeries(householdTxns, bounds, rates)
	}
	v.Credit = CreditSeries(v.Accounts, v.Transactions, bounds)
	return v
}

// CreditSeries returns the local credit proxy at each bucket's exclusive end.
// Accounts and transactions must already carry the same report scope. Invalid
// card balances are omitted as in the standalone credit-health input builder.
func CreditSeries(accounts []domain.Account, txns []domain.Transaction, bounds []time.Time) []int {
	var cards []domain.Account
	for _, a := range accounts {
		if !a.Archived && a.Type == domain.TypeCreditCard {
			cards = append(cards, a)
		}
	}
	if len(cards) == 0 || len(bounds) < 2 {
		return nil
	}
	out := make([]int, 0, len(bounds)-1)
	for _, cutoff := range bounds[1:] {
		upto := make([]domain.Transaction, 0, len(txns))
		for _, txn := range txns {
			if txn.Date.Before(cutoff) {
				upto = append(upto, txn)
			}
		}
		// One balance pass across the selected cards replaces one full ledger
		// scan per card and keeps the malformed-account behavior of Balance.
		balances, _ := ledger.Balances(cards, upto)
		minor := make(map[string]int64, len(balances))
		for id, balance := range balances {
			minor[id] = balance.Amount
		}
		cr := credithealth.Evaluate(credithealth.Inputs{Accounts: accounts, Balances: minor, Transactions: upto, Now: cutoff})
		out = append(out, cr.ProxyScore)
	}
	return out
}
