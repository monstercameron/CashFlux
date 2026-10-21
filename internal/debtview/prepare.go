// SPDX-License-Identifier: MIT

// Package debtview prepares checked, platform-independent facts for the debt screen.
package debtview

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
)

// Config is the payoff policy that affects debt preparation.
type Config struct {
	ExtraMinor int64
	Strategy   payoff.Strategy
}

// AccountFact holds one liability's checked balance and base-currency availability.
type AccountFact struct {
	Account       domain.Account
	Owed          money.Money
	OwedKnown     bool
	BalanceErr    error
	BaseOwed      money.Money
	BaseKnown     bool
	ConversionErr error
	Included      bool
	Rank          int
}

// Result contains per-account facts and checked aggregate payoff calculations.
type Result struct {
	Accounts    []AccountFact
	TotalOwed   money.Money
	TotalKnown  bool
	TotalErr    error
	PlanOwed    money.Money
	PlanKnown   bool
	PlanErr     error
	Debts       []payoff.Debt
	Plan        payoff.Plan
	PlanValid   bool
	PlanErrCalc error
	DebtFree    time.Time
}

// Prepare computes liability balances once, reports failures instead of substituting zero,
// and preserves Account.IncludedInPayoff and the selected payoff strategy.
func Prepare(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, base string, cfg Config, now time.Time) Result {
	if base == "" {
		base = rates.Base
	}
	out := Result{TotalOwed: money.Zero(base), TotalKnown: true, PlanOwed: money.Zero(base), PlanKnown: true}
	balances, batchErr := ledger.Balances(accounts, txns)
	_ = batchErr // Errors are attached to the specific account below.
	total, planTotal := money.Zero(base), money.Zero(base)
	var firstTotalErr, firstPlanErr error
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		fact := AccountFact{Account: a, Included: a.IncludedInPayoff()}
		bal, ok := balances[a.ID]
		if !ok {
			_, err := ledger.Balance(a, txns)
			if err == nil {
				err = fmt.Errorf("debtview: account %s: balance unavailable", a.ID)
			}
			fact.BalanceErr = err
			if firstTotalErr == nil {
				firstTotalErr = err
			}
			out.TotalKnown = false
			if fact.Included {
				if firstPlanErr == nil {
					firstPlanErr = err
				}
				out.PlanKnown = false
			}
			out.Accounts = append(out.Accounts, fact)
			continue
		}
		owed, err := bal.AbsChecked()
		if err != nil {
			fact.BalanceErr = err
			if firstTotalErr == nil {
				firstTotalErr = err
			}
			out.TotalKnown = false
			if fact.Included {
				if firstPlanErr == nil {
					firstPlanErr = err
				}
				out.PlanKnown = false
			}
			out.Accounts = append(out.Accounts, fact)
			continue
		}
		fact.Owed, fact.OwedKnown = owed, true
		converted, err := rates.Convert(owed, base)
		if err != nil {
			fact.ConversionErr = err
			if firstTotalErr == nil {
				firstTotalErr = err
			}
			out.TotalKnown = false
			if fact.Included {
				if firstPlanErr == nil {
					firstPlanErr = err
				}
				out.PlanKnown = false
			}
			out.Accounts = append(out.Accounts, fact)
			continue
		}
		fact.BaseOwed, fact.BaseKnown = converted, true
		if total, err = total.Add(converted); err != nil {
			if firstTotalErr == nil {
				firstTotalErr = err
			}
			out.TotalKnown = false
		}
		if fact.Included {
			if planTotal, err = planTotal.Add(converted); err != nil {
				if firstPlanErr == nil {
					firstPlanErr = err
				}
				out.PlanKnown = false
			}
			if owed.Amount > 0 {
				min := money.New(a.MinPayment.Amount, a.Currency)
				minBase := int64(0)
				if min.Amount > 0 {
					convertedMin, e := rates.Convert(min, base)
					if e != nil {
						fact.ConversionErr = e
						fact.BaseKnown = false
						out.PlanKnown = false
						if firstPlanErr == nil {
							firstPlanErr = e
						}
					} else {
						minBase = convertedMin.Amount
					}
				}
				if fact.BaseKnown {
					out.Debts = append(out.Debts, payoff.Debt{Name: a.Name, Balance: converted.Amount, AprPercent: a.RateAPROrZero(), MinPayment: minBase})
				}
			}
		}
		out.Accounts = append(out.Accounts, fact)
	}
	out.TotalOwed, out.PlanOwed = total, planTotal
	out.TotalErr, out.PlanErr = firstTotalErr, firstPlanErr
	order := payoff.FocusOrder(out.Debts, cfg.Strategy)
	ranks := map[string]int{}
	for i, name := range order {
		if _, exists := ranks[name]; !exists {
			ranks[name] = i + 1
		}
	}
	for i := range out.Accounts {
		out.Accounts[i].Rank = ranks[out.Accounts[i].Account.Name]
	}
	if out.PlanKnown {
		out.Plan, out.PlanValid, out.PlanErrCalc = payoff.BuildPlanChecked(out.Debts, cfg.ExtraMinor, cfg.Strategy)
		if out.PlanValid && len(out.Debts) > 0 {
			out.DebtFree = payoff.DebtFreeMonth(now, out.Plan.Months)
		}
	}
	sort.SliceStable(out.Accounts, func(i, j int) bool {
		a, b := out.Accounts[i], out.Accounts[j]
		if a.Rank > 0 && b.Rank > 0 {
			return a.Rank < b.Rank
		}
		if a.Rank > 0 {
			return true
		}
		if b.Rank > 0 {
			return false
		}
		return a.Owed.Amount > b.Owed.Amount
	})
	return out
}
