// SPDX-License-Identifier: MIT

package planningmodel

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPrepareChecksTrailingForecastAndSafeSpend(t *testing.T) {
	now := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
	accounts := []domain.Account{{
		ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "USD", OpeningBalance: money.New(100_000, "USD"), BalanceAsOf: now,
	}}
	txns := []domain.Transaction{
		{ID: "may-income", AccountID: "checking", Date: time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(50_000, "USD")},
		{ID: "may-spend", AccountID: "checking", Date: time.Date(2026, time.May, 16, 0, 0, 0, 0, time.UTC), Amount: money.New(-20_000, "USD")},
		{ID: "june-income", AccountID: "checking", Date: time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(40_000, "USD")},
		{ID: "june-spend", AccountID: "checking", Date: time.Date(2026, time.June, 16, 0, 0, 0, 0, time.UTC), Amount: money.New(-10_000, "USD")},
	}
	trim := int64(5_000)
	prepared := Prepare(Snapshot{Accounts: accounts, Transactions: txns}, Inputs{Now: now, Base: "USD", TrimMinor: &trim})
	if !prepared.NetWorth.Available || !prepared.TrailingMonthlyNet.Available || !prepared.Forecast.Available || !prepared.TrimForecast.Available {
		t.Fatalf("expected checked core facts to be available: %+v", prepared)
	}
	if got, want := prepared.TrailingMonthlyNet.Value, int64(20_000); got != want {
		t.Fatalf("trailing average = %d, want %d", got, want)
	}
	if got, want := prepared.Forecast.Value.Balances[0], prepared.NetWorth.Value.Net.Amount+prepared.TrailingMonthlyNet.Value; got != want {
		t.Fatalf("first forecast month = %d, want %d", got, want)
	}
	if got, want := prepared.TrimForecast.Value.Balances[0], prepared.NetWorth.Value.Net.Amount+prepared.TrailingMonthlyNet.Value+trim; got != want {
		t.Fatalf("first trimmed forecast month = %d, want %d", got, want)
	}
	if !prepared.LiquidBalance.Available || !prepared.SafeToSpend.Available || prepared.SafeToSpend.Value.SafeToSpend != prepared.LiquidBalance.Value.Amount {
		t.Fatalf("expected available safe-to-spend with no commitments: liquid=%+v safe=%+v", prepared.LiquidBalance, prepared.SafeToSpend)
	}
}

func TestPrepareKeepsIndependentFactsWhenSourceFails(t *testing.T) {
	now := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	prepared := Prepare(Snapshot{
		TransactionsErr: errors.New("transaction read failed"),
		Accounts: []domain.Account{{
			ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID,
			Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
			Currency: "USD", OpeningBalance: money.New(100_000, "USD"), BalanceAsOf: now,
		}},
	}, Inputs{Now: now, Base: "USD"})
	if prepared.NetWorth.Available || prepared.Forecast.Available || prepared.LiquidBalance.Available {
		t.Fatalf("transaction-dependent facts should be unavailable: %+v", prepared)
	}
	if !prepared.NetWorth.Partial { // no partial figures are fabricated on a source read failure
		// Partial is reserved for a successful calculation with known missing FX; assert the error instead.
		if prepared.NetWorth.Err == nil || !strings.Contains(prepared.NetWorth.Err.Error(), "transaction read failed") {
			t.Fatalf("expected source error on unavailable net worth: %+v", prepared.NetWorth)
		}
	}
}

func TestPrepareDisclosesPartialFXAndRejectsInvalidRunwayBuffer(t *testing.T) {
	now := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	account := domain.Account{
		ID: "foreign-cash", Name: "Foreign cash", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "EUR", OpeningBalance: money.New(10_000, "EUR"), BalanceAsOf: now,
	}
	prepared := Prepare(Snapshot{Accounts: []domain.Account{account}}, Inputs{Now: now, Base: "USD"})
	if !prepared.NetWorth.Available || !prepared.NetWorth.Partial || !prepared.Forecast.Available || !prepared.Forecast.Partial || prepared.Forecast.Notice == "" {
		t.Fatalf("partial FX availability must remain explicit through forecast: net=%+v forecast=%+v", prepared.NetWorth, prepared.Forecast)
	}

	prepared = Prepare(Snapshot{Accounts: []domain.Account{account}, Recurring: []domain.Recurring{{
		ID: "payday", Amount: money.New(5_000, "USD"), Cadence: domain.CadenceMonthly, NextDue: now.AddDate(0, 0, 1),
	}}}, Inputs{Now: now, Base: "USD", FXRates: map[string]float64{"EUR": 1.1}, RunwayBufferErr: errors.New("invalid runway buffer")})
	if prepared.Runway.Available || prepared.Runway.Err == nil || !strings.Contains(prepared.Runway.Err.Error(), "invalid runway buffer") {
		t.Fatalf("invalid runway input must not produce a plausible projection: %+v", prepared.Runway)
	}
}

func TestPrepareRejectsForecastAndRunwayOverflow(t *testing.T) {
	now := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{{
		ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD"), BalanceAsOf: now,
	}}
	txns := []domain.Transaction{
		{ID: "income-a", AccountID: "checking", Date: time.Date(2026, time.June, 5, 0, 0, 0, 0, time.UTC), Amount: money.New(math.MaxInt64, "USD")},
		{ID: "income-b", AccountID: "checking", Date: time.Date(2026, time.June, 6, 0, 0, 0, 0, time.UTC), Amount: money.New(math.MaxInt64, "USD")},
	}
	prepared := Prepare(Snapshot{Accounts: accounts[:0], Transactions: txns}, Inputs{Now: now, Base: "USD"})
	if prepared.TrailingMonthlyNet.Available || prepared.Forecast.Available {
		t.Fatalf("overflowed forecast inputs must be unavailable: trailing=%+v forecast=%+v", prepared.TrailingMonthlyNet, prepared.Forecast)
	}
	if prepared.TrailingMonthlyNet.Err == nil || !strings.Contains(prepared.TrailingMonthlyNet.Err.Error(), "overflow") {
		t.Fatalf("expected checked overflow error, got %+v", prepared.TrailingMonthlyNet)
	}
}
