// SPDX-License-Identifier: MIT

// Package loanview prepares checked amortization facts for installment-loan cards.
package loanview

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
)

// Input provides the saved account and current editable loan-card values.
type Input struct {
	Account    domain.Account
	Balance    money.Money
	BalanceErr error
	TermMonths int
	ExtraText  string
	Now        time.Time
}

// Result contains amortization schedules and their summaries, with explicit source/input state.
type Result struct {
	Balance                                  money.Money
	BalanceKnown                             bool
	BalanceErr                               error
	TermMonths                               int
	TermErr                                  error
	ExtraMinor                               int64
	ExtraErr                                 error
	BaseRows, ExtraRows                      []payoff.AmortRow
	MonthlyPayment, TotalInterest, TotalPaid int64
	PayoffDate                               time.Time
	ExtraInterest, InterestSaved             int64
	MonthsSaved                              int
	ExtraPayoffDate                          time.Time
}

// Prepare parses extra payment exactly in account-currency minor units and reuses payoff schedules.
func Prepare(in Input) Result {
	a := in.Account
	cur := a.Currency
	if cur == "" {
		cur = in.Balance.Currency
	}
	out := Result{Balance: in.Balance, BalanceErr: in.BalanceErr, BalanceKnown: in.BalanceErr == nil, TermMonths: in.TermMonths}
	if in.BalanceErr != nil {
		return out
	}
	if out.Balance.Amount < 0 {
		var err error
		out.Balance, err = out.Balance.AbsChecked()
		if err != nil {
			out.BalanceErr = err
			out.BalanceKnown = false
			return out
		}
	}
	term := in.TermMonths
	if term <= 0 || term > 1200 {
		out.TermErr = fmt.Errorf("loanview: term must be between 1 and 1200 months")
		return out
	}
	if out.Balance.Amount <= 0 {
		return out
	}
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	apr, _ := a.RateAPR()
	out.BaseRows = payoff.AmortizeFixed(out.Balance.Amount, apr, term, in.Now)
	if len(out.BaseRows) > 0 {
		out.MonthlyPayment = out.BaseRows[0].PaymentMinor
		out.TotalInterest, out.TotalPaid, out.PayoffDate = payoff.AmortSummary(out.BaseRows)
	}
	if in.ExtraText != "" {
		extra, err := money.ParseMinor(in.ExtraText, currency.Decimals(cur))
		if err != nil {
			out.ExtraErr = err
			return out
		}
		if extra < 0 {
			out.ExtraErr = fmt.Errorf("loanview: extra payment must not be negative")
			return out
		}
		out.ExtraMinor = extra
	}
	if out.ExtraMinor > 0 {
		out.ExtraRows = payoff.AmortizeWithExtra(out.Balance.Amount, apr, term, out.ExtraMinor, in.Now)
		if len(out.ExtraRows) > 0 {
			out.ExtraInterest, _, out.ExtraPayoffDate = payoff.AmortSummary(out.ExtraRows)
			out.MonthsSaved = len(out.BaseRows) - len(out.ExtraRows)
			if out.MonthsSaved < 0 {
				out.MonthsSaved = 0
			}
			out.InterestSaved = out.TotalInterest - out.ExtraInterest
			if out.InterestSaved < 0 {
				out.InterestSaved = 0
			}
		}
	}
	return out
}
