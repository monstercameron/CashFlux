// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddBillsSmartVars(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	// Paydays cover the whole 60-day horizon (the engine now projects every bill
	// occurrence in the window — July's AND August's — so the pay periods must
	// span both months for the loads to be meaningful).
	paydays := []time.Time{
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC),
	}
	// Two recurring expense bills stacked late in the month (both land in the
	// same pay period each month), plenty of cash → pay-ahead should split them.
	recs := []domain.Recurring{
		{ID: "r1", Label: "Rent", Amount: money.New(-50000, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)},
		{ID: "r2", Label: "Power", Amount: money.New(-50000, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)},
	}
	accts := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(1000000, "USD")},
	}
	d := Data{
		Accounts: accts, Recurring: recs, Rates: currency.Rates{Base: "USD"}, Now: now,
		BillsSmart: BillsSmartData{Paydays: paydays, IncomePerPayday: 100000},
	}
	vars := Vars(d)

	if got := vars["bills_check_load_raw"]; got != 1000 {
		t.Errorf("bills_check_load_raw = %v, want 1000 (both bills on one check)", got)
	}
	if got := vars["bills_check_load_smart"]; got != 500 {
		t.Errorf("bills_check_load_smart = %v, want 500 (split across checks)", got)
	}
	if got := vars["bills_even_gain"]; got != 500 {
		t.Errorf("bills_even_gain = %v, want 500", got)
	}
	// Both July's AND August's stacked pairs get split — the occurrence
	// projection is what makes the August moves exist at all.
	if got := vars["bills_paid_ahead"]; got < 2 {
		t.Errorf("bills_paid_ahead = %v, want ≥ 2 (one split per month in the window)", got)
	}
}

func TestAddBillsSmartVarsNoPaydays(t *testing.T) {
	vars := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	for _, k := range BillsSmartVarNames {
		if _, ok := vars[k]; !ok {
			t.Errorf("%s should always be present", k)
		}
	}
	if vars["bills_even_gain"] != 0 || vars["bills_paid_ahead"] != 0 {
		t.Error("no paydays → gains and moves must be 0")
	}
}
