// SPDX-License-Identifier: MIT

package creditview

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func modelInput() Input {
	return Input{Accounts: []domain.Account{{ID: "card", Name: "Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "EUR", OpeningBalance: money.New(-80000, "EUR"), CreditLimit: money.New(100000, "EUR"), BalanceAsOf: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), DueDayOfMonth: 15}}, Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, Now: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), Details: true, History: true}
}
func TestModelNativeCurrencySummaryAndSourceAvailability(t *testing.T) {
	in := modelInput()
	full := Prepare(in)
	if full.Fatal || !full.ProxyKnown || full.Scope != "household" || full.AsOf != in.Now || full.BalanceFolds != 1 || full.HistoryBuilds != 1 || full.Cards[0].Owed != money.New(80000, "EUR") || full.Cards[0].Target30 != money.New(50000, "EUR") {
		t.Fatalf("native model %+v", full)
	}
	in.Details = false
	summary := Prepare(in)
	if len(summary.Cards) != 0 || len(summary.Advice) != 0 || summary.HistoryBuilds != 0 || summary.ProxyScore != full.ProxyScore {
		t.Fatalf("summary prepared expensive details: %+v", summary)
	}
	in.Details = true
	in.TransactionsErr = errors.New("read failed")
	unknown := Prepare(in)
	if unknown.ProxyKnown || unknown.Cards[0].OwedErr == nil || unknown.OnTime.Err == nil || unknown.Age.Err != nil || unknown.Age.Score < 0 {
		t.Fatalf("independent source availability %+v", unknown)
	}
	in.AccountsErr = errors.New("accounts failed")
	failed := Prepare(in)
	if !failed.Fatal || failed.Empty {
		t.Fatalf("source failure became no cards: %+v", failed)
	}
}
func TestModelUnknownFXAndOverflowDoNotDiscardHistoryOrKnownFacts(t *testing.T) {
	in := modelInput()
	in.Rates.Rates = nil
	got := Prepare(in)
	if got.ProxyKnown || got.UtilErr == nil || !got.Cards[0].HasPayment || got.Cards[0].OwedErr != nil || got.OnTime.Err != nil || !got.Incomplete {
		t.Fatalf("unknown FX discarded native facts: %+v", got)
	}
	in = modelInput()
	in.Accounts[0].OpeningBalance = money.New(math.MinInt64, "EUR")
	got = Prepare(in)
	if got.ProxyKnown || got.Cards[0].OwedErr == nil || got.Age.Score < 0 {
		t.Fatalf("overflow fabricated facts: %+v", got)
	}
	in = modelInput()
	in.Histories = map[string]HistoryInput{"card": {Err: errors.New("history failed")}}
	got = Prepare(in)
	if !got.ProxyKnown || got.Cards[0].History.Err == nil || !got.Incomplete {
		t.Fatalf("independent history failure: %+v", got)
	}
}

func TestModelHouseholdEligibilityAndPreparedMeterBoundaries(t *testing.T) {
	for _, row := range []struct {
		owed, want int64
		met        bool
		meter      int
	}{{0, 0, true, 0}, {29000, 29, true, 29}, {30000, 30, false, 30}, {101000, 101, false, 100}} {
		in := modelInput()
		in.Accounts[0].OpeningBalance = money.New(-row.owed, "EUR")
		// Bank records and archived cards never alter household revolving facts.
		archived := in.Accounts[0]
		archived.ID, archived.Archived = "archived", true
		bank := in.Accounts[0]
		bank.ID, bank.Type = "bank", domain.TypeChecking
		bank.OpeningBalance = money.New(math.MinInt64, "EUR")
		in.Accounts = append(in.Accounts, archived, bank)
		m := Prepare(in)
		if len(m.Cards) != 1 || m.UtilErr != nil || m.UtilPct != row.want || m.UtilOnTarget != row.met || m.UtilMeter != row.meter || m.Cards[0].Meter != row.meter || m.Scope != "household" {
			t.Fatalf("boundary owed%d: %+v", row.owed, m)
		}
	}
}
