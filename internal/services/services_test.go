// SPDX-License-Identifier: MIT

package services

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var noRates = currency.Rates{Base: "USD"}

func TestOfKeepsOnlyServicesInStoredOrder(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "rent", Label: "Rent"},
		{ID: "xfinity", Label: "Comcast Xfinity", Service: true},
		{ID: "power", Label: "Power", Service: true},
	}
	got := Of(recs)
	if len(got) != 2 || got[0].ID != "xfinity" || got[1].ID != "power" {
		t.Fatalf("Of = %+v, want xfinity then power", got)
	}
}

func TestMonthlyTotalSumsActiveServicesAndSkipsPaused(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "xfinity", Label: "Xfinity", Service: true, Cadence: domain.CadenceMonthly, Amount: money.New(-8000, "USD")},
		{ID: "power", Label: "Power", Service: true, Cadence: domain.CadenceMonthly, Amount: money.New(-5000, "USD")},
		{ID: "gym", Label: "Gym", Service: true, Paused: true, Cadence: domain.CadenceMonthly, Amount: money.New(-3000, "USD")},
		{ID: "rent", Label: "Rent", Cadence: domain.CadenceMonthly, Amount: money.New(-150000, "USD")},
	}
	got, err := MonthlyTotal(recs, "USD", noRates)
	if err != nil {
		t.Fatal(err)
	}
	if got != 13000 {
		t.Fatalf("MonthlyTotal = %d, want 13000 (rent and the paused gym are left out)", got)
	}
}

func TestMonthlyTotalNormalisesCadence(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "yearly", Label: "Domain", Service: true, Cadence: domain.CadenceYearly, Amount: money.New(-12000, "USD")},
	}
	got, err := MonthlyTotal(recs, "USD", noRates)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1000 {
		t.Fatalf("yearly 120.00 as a monthly total = %d, want 1000", got)
	}
}

func TestNewMonthlyStoresASpendingServiceMarkedAsAService(t *testing.T) {
	due := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	r := NewMonthly(Draft{ID: "svc-1", Name: "Comcast Xfinity", Provider: "Comcast",
		Currency: "USD", MonthlyMinor: 8000, NextDue: due})
	if !r.Service || r.Provider != "Comcast" || r.Label != "Comcast Xfinity" {
		t.Fatalf("NewMonthly fields = %+v", r)
	}
	if r.Amount.Amount != -8000 || r.Cadence != domain.CadenceMonthly || !r.NextDue.Equal(due) {
		t.Fatalf("NewMonthly amount/cadence/due = %d %q %v", r.Amount.Amount, r.Cadence, r.NextDue)
	}
}
