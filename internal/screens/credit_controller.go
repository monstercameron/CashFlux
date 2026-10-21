// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/creditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// prepareCredit acquires each household source once. Summary and AI callers do
// not read historical observations; the native model owns all financial facts.
func prepareCredit(app *appstate.App, now time.Time, details, history bool) creditview.Model {
	in := creditview.Input{Now: now, Details: details, History: history}
	if app == nil {
		in.AccountsErr = fmt.Errorf("credit: app is not ready")
		return creditview.Prepare(in)
	}
	in.Accounts, in.AccountsErr = app.Store().ListAccounts()
	in.Transactions, in.TransactionsErr = app.Store().ListTransactions()
	settings, err := app.Store().GetSettings()
	in.RatesErr = err
	if err == nil {
		base := settings.BaseCurrency
		if base == "" {
			base = "USD"
		}
		in.Rates = currency.Rates{Base: base, Rates: settings.FXRates}
	}
	if details && history && in.AccountsErr == nil {
		in.Histories = make(map[string]creditview.HistoryInput)
		for _, a := range in.Accounts {
			if a.Type == domain.TypeCreditCard && !a.Archived {
				snapshots, readErr := app.Store().ListBalanceSnapshots(a.ID)
				in.Histories[a.ID] = creditview.HistoryInput{Snapshots: snapshots, Err: readErr}
			}
		}
	}
	return creditview.Prepare(in)
}

// saveCreditLimit re-reads the current account so a draft cannot overwrite
// concurrent name, owner, or balance changes. Currency changes require a new edit.
func saveCreditLimit(app *appstate.App, id, accountCurrency string, limit money.Money) error {
	if app == nil {
		return fmt.Errorf("credit limit: app is not ready")
	}
	account, found, err := app.Store().GetAccount(id)
	if err != nil {
		return fmt.Errorf("credit limit: read account: %w", err)
	}
	if !found || account.Archived || account.Type != domain.TypeCreditCard {
		return fmt.Errorf("credit limit: account is unavailable")
	}
	currentCurrency := account.CreditLimit.Currency
	if currentCurrency == "" {
		currentCurrency = account.Currency
	}
	if account.Currency != accountCurrency || currentCurrency != limit.Currency || limit.Amount < 0 || !currency.Valid(limit.Currency) {
		return fmt.Errorf("credit limit: account currency changed or limit is invalid")
	}
	account.CreditLimit = limit
	if err := app.PutAccount(account); err != nil {
		return fmt.Errorf("credit limit: save account: %w", err)
	}
	uistate.BumpDataRevision()
	return nil
}
