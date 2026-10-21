// SPDX-License-Identifier: MIT

package audit

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
)

// RunChecked executes the existing free detectors only after checking input
// currencies. Legacy detectors can omit failed FX conversions; an assistant
// sweep must not interpret those omissions as a clean financial verdict.
// Detector findings remain estimates, not proof of independent savings.
func RunChecked(in smartengine.Input, settings smart.Settings) (Report, error) {
	if in.Now.IsZero() {
		return Report{}, fmt.Errorf("audit reference time is required")
	}
	check := func(label string, amount money.Money) error {
		if amount.Amount == 0 {
			return nil
		}
		if amount.Currency == "" {
			return fmt.Errorf("audit input %s has no currency", label)
		}
		converted, err := in.Rates.Convert(amount, in.Base)
		if err != nil {
			return fmt.Errorf("audit input %s currency: %w", label, err)
		}
		if _, err = money.Abs(converted.Amount); err != nil {
			return fmt.Errorf("audit input %s magnitude: %w", label, err)
		}
		return nil
	}
	for _, a := range in.Accounts {
		for _, m := range []money.Money{a.OpeningBalance, a.CreditLimit, a.MinPayment, a.MonthlySavings} {
			if err := check("account "+a.ID, m); err != nil {
				return Report{}, err
			}
		}
	}
	for _, t := range in.Transactions {
		if err := check("transaction "+t.ID, t.Amount); err != nil {
			return Report{}, err
		}
		for _, split := range t.Splits {
			if err := check("transaction split "+t.ID, split.Amount); err != nil {
				return Report{}, err
			}
		}
	}
	for _, b := range in.Budgets {
		if err := check("budget "+b.ID, b.Limit); err != nil {
			return Report{}, err
		}
	}
	for _, g := range in.Goals {
		for _, m := range []money.Money{g.TargetAmount, g.CurrentAmount, g.MonthlyContribution} {
			if err := check("goal "+g.ID, m); err != nil {
				return Report{}, err
			}
		}
		for _, a := range g.Allocations {
			if err := check("goal allocation "+g.ID, a.Amount); err != nil {
				return Report{}, err
			}
		}
	}
	for _, r := range in.Recurring {
		if err := check("recurring "+r.ID, r.Amount); err != nil {
			return Report{}, err
		}
	}
	return AuditChecked(smartengine.Run(in, smart.EnableFreeOnly(settings)), in.Base, in.Rates)
}
