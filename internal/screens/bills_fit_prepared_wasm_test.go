// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBillsPreparedFitUsesFreshFXAndShowsUnavailable(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	now := app.ClockNow()
	settings, err := app.Store().GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.BaseCurrency = "USD"
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "fit-food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutBudget(domain.Budget{ID: "fit-food-budget", Name: "Food", CategoryID: "fit-food",
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Period: domain.PeriodMonthly,
		Limit: money.New(10000, "USD")}); err != nil {
		t.Fatal(err)
	}
	due := now.AddDate(0, 0, 2)
	for _, flow := range []domain.Recurring{
		{ID: "fit-usd", Label: "USD charge", CategoryID: "fit-food", Amount: money.New(-1000, "USD"), Cadence: domain.CadenceMonthly, NextDue: due},
		{ID: "fit-eur", Label: "EUR charge", CategoryID: "fit-food", Amount: money.New(-1000, "EUR"), Cadence: domain.CadenceMonthly, NextDue: due.AddDate(0, 0, 1)},
	} {
		if err := app.PutRecurring(flow); err != nil {
			t.Fatal(err)
		}
	}
	candidate := billByID(bills.UpcomingAll(app.Accounts(), app.Recurring(), now), "recurring:fit-usd")
	if candidate.AccountID == "" {
		t.Fatal("candidate bill was not prepared")
	}
	chip := billFitFor(prepareBillFitModel(app, []bills.Bill{candidate}, now), candidate)
	if chip == nil || chip.Available || chip.Amount != "" {
		t.Fatalf("missing pending EUR rate was shown as a value: %+v", chip)
	}

	settings.FXRates = map[string]float64{"EUR": 1.2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	chip = billFitFor(prepareBillFitModel(app, []bills.Bill{candidate}, now), candidate)
	if chip == nil || !chip.Available || !chip.Fits || chip.Amount == "" {
		t.Fatalf("refreshed FX did not restore a checked fit: %+v", chip)
	}
}

func billByID(items []bills.Bill, id string) bills.Bill {
	for _, item := range items {
		if item.AccountID == id {
			return item
		}
	}
	return bills.Bill{}
}
