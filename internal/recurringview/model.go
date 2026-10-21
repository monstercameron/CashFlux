// SPDX-License-Identifier: MIT

// Package recurringview prepares checked recurring-flow amounts and schedule
// occurrences for the Bills & Recurring surface. It has no UI or store access.
package recurringview

import (
	"errors"
	"fmt"
	"sort"
	"time"

	budgetrecurring "github.com/monstercameron/CashFlux/internal/budgetview/recurring"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// FlowFact carries the original schedule and its checked monthly amount in the
// selected base currency. A failure is attached to this flow so other readable
// rows can keep their own known values.
type FlowFact struct {
	Recurring        domain.Recurring
	MonthlyAmount    int64
	MonthlyAvailable bool
	MonthlyError     error
	SharePercent     float64
	ShareAvailable   bool
}

// OccurrenceFact is one overdue or upcoming occurrence projected from a flow.
// Amount is expressed in the model's base currency when AmountAvailable is true.
type OccurrenceFact struct {
	Recurring       domain.Recurring
	Date            time.Time
	Overdue         bool
	Amount          money.Money
	AmountAvailable bool
	AmountError     error
}

// Model is an immutable set of prepared recurring facts. Aggregate availability
// is separate from row availability: one bad currency or amount must not hide
// unrelated flow definitions or their individually known amounts.
type Model struct {
	Base string

	Flows                             []FlowFact
	MonthlyIn, MonthlyOut, MonthlyNet int64
	MonthlyAvailable                  bool
	MonthlyError                      error
	IncomeShareAvailable              bool
	ExpenseShareAvailable             bool

	Occurrences             []OccurrenceFact
	UpcomingIn, UpcomingOut int64
	UpcomingTotalsAvailable bool
	UpcomingTotalsError     error
}

// MonthlyExpenseTotal sums the positive magnitudes of known negative-flow
// monthly facts. It is used for the subscription lens subtotal.
func MonthlyExpenseTotal(flows []FlowFact) (int64, error) {
	var total int64
	for _, flow := range flows {
		if !flow.MonthlyAvailable {
			if flow.MonthlyError != nil {
				return 0, fmt.Errorf("recurring %q monthly amount: %w", flow.Recurring.ID, flow.MonthlyError)
			}
			return 0, fmt.Errorf("recurring %q monthly amount unavailable", flow.Recurring.ID)
		}
		if flow.MonthlyAmount >= 0 {
			continue
		}
		magnitude, err := money.Abs(flow.MonthlyAmount)
		if err != nil {
			return 0, fmt.Errorf("recurring %q monthly magnitude: %w", flow.Recurring.ID, err)
		}
		total, err = money.Add(total, magnitude)
		if err != nil {
			return 0, fmt.Errorf("recurring expense total: %w", err)
		}
	}
	return total, nil
}

// AnnualizeMonthly scales a monthly minor-unit amount to twelve months with a
// checked wide intermediate.
func AnnualizeMonthly(monthly int64) (int64, error) {
	return money.MulDiv(monthly, 12, 1, money.RoundTowardZero)
}

// Build prepares signed monthly equivalents and the same overdue + next-30-day
// occurrences the recurring surface has historically shown. Monthly totals
// include paused flows; paused flows do not produce occurrences. Occurrences
// include the cutoff date and retain the eight-per-flow safety cap.
func Build(flows []domain.Recurring, now time.Time, base string, rates currency.Rates) Model {
	if base == "" {
		base = rates.Base
	}
	if base == "" {
		base = "USD"
	}
	rates.Base = base
	model := Model{
		Base: base, MonthlyAvailable: true, UpcomingTotalsAvailable: true,
		IncomeShareAvailable: true, ExpenseShareAvailable: true,
	}
	model.Flows = make([]FlowFact, 0, len(flows))
	monthlyIn, monthlyOut, monthlyNet := int64(0), int64(0), int64(0)

	for _, flow := range flows {
		fact := FlowFact{Recurring: flow}
		monthly, err := budgetrecurring.MonthlyEquivalent(flow)
		if err == nil {
			amount := money.New(monthly, flow.Amount.Currency)
			if amount.Currency == "" {
				amount.Currency = base
			}
			var converted money.Money
			converted, err = rates.Convert(amount, base)
			if err == nil {
				fact.MonthlyAmount = converted.Amount
				fact.MonthlyAvailable = true
			}
		}
		if err != nil {
			fact.MonthlyError = fmt.Errorf("recurring %q monthly amount: %w", flow.ID, err)
			model.MonthlyAvailable = false
			model.MonthlyError = errors.Join(model.MonthlyError, fact.MonthlyError)
			if flow.Amount.Amount >= 0 {
				model.IncomeShareAvailable = false
			} else {
				model.ExpenseShareAvailable = false
			}
		} else {
			monthlyNet, err = money.Add(monthlyNet, fact.MonthlyAmount)
			if err == nil && fact.MonthlyAmount >= 0 {
				monthlyIn, err = money.Add(monthlyIn, fact.MonthlyAmount)
				if err != nil {
					model.IncomeShareAvailable = false
				}
			} else if err == nil {
				magnitude, absErr := money.Abs(fact.MonthlyAmount)
				err = absErr
				if err == nil {
					monthlyOut, err = money.Add(monthlyOut, magnitude)
				}
				if err != nil {
					model.ExpenseShareAvailable = false
				}
			}
			if err != nil {
				model.MonthlyAvailable = false
				model.MonthlyError = errors.Join(model.MonthlyError, fmt.Errorf("recurring monthly total: %w", err))
			}
		}
		model.Flows = append(model.Flows, fact)
	}
	if model.MonthlyAvailable {
		model.MonthlyIn, model.MonthlyOut, model.MonthlyNet = monthlyIn, monthlyOut, monthlyNet
	}
	for i := range model.Flows {
		fact := &model.Flows[i]
		if fact.MonthlyAmount >= 0 && model.IncomeShareAvailable && fact.MonthlyAvailable {
			fact.ShareAvailable = true
			if monthlyIn > 0 {
				fact.SharePercent = float64(fact.MonthlyAmount) / float64(monthlyIn) * 100
			}
		} else if fact.MonthlyAmount < 0 && model.ExpenseShareAvailable && fact.MonthlyAvailable {
			fact.ShareAvailable = true
			magnitude, err := money.Abs(fact.MonthlyAmount)
			if err == nil && monthlyOut > 0 {
				fact.SharePercent = float64(magnitude) / float64(monthlyOut) * 100
			}
		}
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := today.AddDate(0, 0, 30)
	upcomingIn, upcomingOut := int64(0), int64(0)
	for _, flow := range flows {
		if !flow.Active() {
			continue
		}
		date := flow.NextDue
		for i := 0; i < 8 && !date.After(cutoff); i++ {
			fact := OccurrenceFact{
				Recurring: flow,
				Date:      date,
				Overdue:   date.Before(today),
			}
			amount := flow.Amount
			if amount.Currency == "" {
				amount.Currency = base
			}
			fact.Amount, fact.AmountError = rates.Convert(amount, base)
			fact.AmountAvailable = fact.AmountError == nil
			if fact.AmountError != nil {
				fact.AmountError = fmt.Errorf("recurring %q occurrence amount: %w", flow.ID, fact.AmountError)
				model.UpcomingTotalsAvailable = false
				model.UpcomingTotalsError = errors.Join(model.UpcomingTotalsError, fact.AmountError)
			} else if fact.Amount.Amount >= 0 {
				var sumErr error
				upcomingIn, sumErr = money.Add(upcomingIn, fact.Amount.Amount)
				if sumErr != nil {
					model.UpcomingTotalsAvailable = false
					model.UpcomingTotalsError = errors.Join(model.UpcomingTotalsError, fmt.Errorf("recurring upcoming income total: %w", sumErr))
				}
			} else {
				var magnitude int64
				magnitude, err := money.Abs(fact.Amount.Amount)
				if err == nil {
					upcomingOut, err = money.Add(upcomingOut, magnitude)
				}
				if err != nil {
					model.UpcomingTotalsAvailable = false
					model.UpcomingTotalsError = errors.Join(model.UpcomingTotalsError, fmt.Errorf("recurring upcoming expense total: %w", err))
				}
			}
			model.Occurrences = append(model.Occurrences, fact)
			date = flow.Cadence.Next(date)
		}
	}
	sort.SliceStable(model.Flows, func(i, j int) bool {
		return model.Flows[i].Recurring.NextDue.Before(model.Flows[j].Recurring.NextDue)
	})
	sort.SliceStable(model.Occurrences, func(i, j int) bool {
		return model.Occurrences[i].Date.Before(model.Occurrences[j].Date)
	})
	if model.UpcomingTotalsAvailable {
		model.UpcomingIn, model.UpcomingOut = upcomingIn, upcomingOut
	}
	return model
}
