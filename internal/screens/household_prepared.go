// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/householdview"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

type householdPrepared struct {
	App        *appstate.App
	Model      householdview.Model
	Members    []domain.Member
	MemberRows map[string]householdview.MemberRow
	SourceErr  error
}

var householdPreparedCache = newViewCache[*householdPrepared](2)
var householdPrepareObserver func()

// householdPreparedAt reads one bound source snapshot and shares its native
// preparation across the hub, roster, analytics and standalone Members route.
// A fresh settings read prevents warm reuse after a closed/unreadable store;
// failed source snapshots are not retained as successful empty datasets.
func householdPreparedAt(app *appstate.App, start, end time.Time) *householdPrepared {
	if app == nil || app.Store() == nil {
		err := fmt.Errorf("household: local dataset is unavailable")
		return &householdPrepared{App: app, Model: householdview.Prepare(householdview.Input{PeriodStart: start, PeriodEnd: end, Errors: householdview.SourceErrors{Settings: err, Accounts: err, Transactions: err, Members: err}})}
	}
	s := app.Store()
	settings, settingsErr := s.GetSettings()
	build := func() *householdPrepared {
		accounts, accountsErr := s.ListAccounts()
		txns, txnsErr := s.ListTransactions()
		members, membersErr := s.ListMembers()
		if membersErr != nil {
			members = nil
		}
		base := ""
		if settingsErr == nil {
			base = settings.BaseCurrency
			if base == "" {
				base = "USD"
			}
		}
		if householdPrepareObserver != nil {
			householdPrepareObserver()
		}
		model := householdview.Prepare(householdview.Input{Accounts: accounts, Transactions: txns, Members: members, Rates: currency.Rates{Base: base, Rates: settings.FXRates}, PeriodStart: start, PeriodEnd: end,
			Errors: householdview.SourceErrors{Settings: settingsErr, Accounts: accountsErr, Transactions: txnsErr, Members: membersErr}})
		out := &householdPrepared{App: app, Model: model, Members: members, MemberRows: make(map[string]householdview.MemberRow, len(model.Members)), SourceErr: errors.Join(settingsErr, accountsErr, txnsErr, membersErr)}
		for _, member := range model.Members {
			out.MemberRows[member.ID] = member
		}
		return out
	}
	if settingsErr != nil {
		out := build()
		householdPreparedCache.Clear()
		return out
	}
	// All native inputs are persisted data or these concrete period bounds.
	// UI notifications/preferences do not change this whole-household model;
	// including their revision would reprepare the same committed store twice.
	key := fmt.Sprintf("%d:%d|household|%s|%s", app.InstanceID(), app.Rev(), start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano))
	out := householdPreparedCache.GetOrCompute(key, build)
	// GetOrCompute may insert the failed result after build returns. Do not
	// persist read failures; financial service errors stay valid for this revision.
	if out.SourceErr != nil {
		householdPreparedCache.Clear()
	}
	return out
}

func householdAmountText(fact householdview.AmountFact) string {
	if fact.Err != nil {
		return uistate.T("calculation.unavailable")
	}
	return fmtMoney(fact.Value)
}

func householdAmountTone(fact householdview.AmountFact) string {
	if fact.Err != nil {
		return ""
	}
	return accentFor(fact.Value)
}
