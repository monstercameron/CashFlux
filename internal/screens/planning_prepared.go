// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/planningmodel"
)

func preparePlanning(app *appstate.App, now time.Time, trimText, compareID, amountText, monthsText, reserveText, bufferText string, decimals int) planningmodel.Prepared {
	if app == nil || app.Store() == nil {
		return planningmodel.Prepare(planningmodel.Snapshot{}, planningmodel.Inputs{Now: now, Base: "USD", SettingsErr: fmt.Errorf("local dataset is unavailable")})
	}
	settings, settingsErr := app.Store().GetSettings()
	base := strings.ToUpper(strings.TrimSpace(settings.BaseCurrency))
	if base == "" {
		base = "USD" // preserve Planning's historical default when no base was saved.
	}
	accounts, accountsErr := app.AccountsChecked()
	txns, txnsErr := app.TransactionsChecked()
	goals, goalsErr := app.GoalsChecked()
	recurring := app.Recurring()
	var recurringErr error
	plans, plansErr := app.Store().ListPlans()
	var comparePlan *domain.Plan
	var comparePlanErr error
	if compareID != "" && plansErr == nil {
		found := false
		for i := range plans {
			if plans[i].ID == compareID {
				comparePlan = &plans[i]
				found = true
				break
			}
		}
		if !found {
			comparePlanErr = fmt.Errorf("selected plan %q is unavailable", compareID)
		}
	} else if compareID != "" {
		comparePlanErr = plansErr
	}
	var trim *int64
	var trimErr error
	if value := strings.TrimSpace(trimText); value != "" {
		minor, err := money.ParseMinor(value, decimals)
		if err != nil {
			trimErr = fmt.Errorf("invalid trim amount: %w", err)
		} else {
			trim = &minor
		}
	}
	var affordability *planningmodel.AffordabilityInput
	var affordabilityErr error
	amountValue := strings.TrimSpace(amountText)
	if amountValue != "" {
		amount, err := money.ParseMinor(amountValue, decimals)
		if err != nil {
			affordabilityErr = fmt.Errorf("invalid purchase amount: %w", err)
		} else {
			months := 0
			var monthErr error
			if strings.TrimSpace(monthsText) != "" {
				months, monthErr = strconv.Atoi(strings.TrimSpace(monthsText))
			}
			if monthErr != nil || months < 0 {
				affordabilityErr = fmt.Errorf("purchase months must be a nonnegative whole number")
			} else {
				reserve := int64(0)
				var reserveErr error
				if strings.TrimSpace(reserveText) != "" {
					reserve, reserveErr = money.ParseMinor(strings.TrimSpace(reserveText), decimals)
				}
				if reserveErr != nil {
					affordabilityErr = fmt.Errorf("invalid purchase reserve: %w", reserveErr)
				} else {
					affordability = &planningmodel.AffordabilityInput{AmountMinor: amount, Months: months, ReserveMinor: reserve}
				}
			}
		}
	}
	buffer := int64(0)
	var bufferErr error
	if strings.TrimSpace(bufferText) != "" {
		buffer, bufferErr = money.ParseMinor(strings.TrimSpace(bufferText), decimals)
	}
	return planningmodel.Prepare(planningmodel.Snapshot{
		Accounts: accounts, AccountsErr: accountsErr, Transactions: txns, TransactionsErr: txnsErr,
		Recurring: recurring, RecurringErr: recurringErr, Goals: goals, GoalsErr: goalsErr,
	}, planningmodel.Inputs{
		Now: now, Base: base, FXRates: settings.FXRates, SettingsErr: settingsErr,
		TrimMinor: trim, TrimErr: trimErr, ComparePlan: comparePlan, ComparePlanErr: comparePlanErr,
		Affordability: affordability, AffordabilityInputErr: affordabilityErr, RunwayBufferMinor: buffer, RunwayBufferErr: bufferErr,
	})
}

func planningFactMessage[T any](fact planningmodel.Fact[T]) string {
	if fact.Err != nil {
		return fact.Err.Error()
	}
	if fact.Reason != "" {
		return fact.Reason
	}
	return "This planning result is unavailable."
}
