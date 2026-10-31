// SPDX-License-Identifier: MIT

// Package services lists the recurring bills the household shows on the Accounts page as services.
// A service is an ordinary recurring bill with its Service flag set. It is not an account, so it
// stays out of the asset, liability and net worth totals. Its monthly cost counts toward budgets
// and reports through the recurring bill itself, which means nothing is counted twice. The package
// has no browser or component-framework dependencies.
package services

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgetview/recurring"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Of returns the recurring bills marked as services, in their stored order.
func Of(recs []domain.Recurring) []domain.Recurring {
	var out []domain.Recurring
	for _, r := range recs {
		if r.Service {
			out = append(out, r)
		}
	}
	return out
}

// MonthlyTotal returns the monthly cost of the active services, as positive base-currency minor
// units. Paused services are skipped, as scheduling skips them.
func MonthlyTotal(recs []domain.Recurring, base string, rates currency.Rates) (int64, error) {
	var total int64
	for _, r := range Of(recs) {
		if !r.Active() {
			continue
		}
		monthly, err := recurring.MonthlyBase(r, base, rates)
		if err != nil {
			return 0, fmt.Errorf("service %q: %w", r.Label, err)
		}
		total, err = money.Add(total, monthly)
		if err != nil {
			return 0, fmt.Errorf("service total: %w", err)
		}
	}
	return total, nil
}

// Draft is what the add form collects for a monthly service.
type Draft struct {
	ID           string
	Name         string
	Provider     string
	Currency     string
	MonthlyMinor int64 // the monthly cost in minor units of Currency
	NextDue      time.Time
}

// NewMonthly builds the stored record for a service. The cost is kept as a negative amount, the sign
// every recurring bill uses for spending. It is marked as a service, so the app keeps it out of
// budgets, reports and forecasts.
func NewMonthly(d Draft) domain.Recurring {
	return domain.Recurring{
		ID:       d.ID,
		Label:    d.Name,
		Provider: d.Provider,
		Service:  true,
		Cadence:  domain.CadenceMonthly,
		Amount:   money.New(-d.MonthlyMinor, d.Currency),
		NextDue:  d.NextDue,
	}
}
