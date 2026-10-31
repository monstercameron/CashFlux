// SPDX-License-Identifier: MIT

package idlecash

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/safespend"
)

func TestFromLedger(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	accounts := []domain.Account{
		{ID: "chk", Type: domain.TypeChecking, Currency: "USD"},
		{ID: "eur", Type: domain.TypeSavings, Currency: "EUR"},
		{ID: "brk", Type: domain.TypeInvestment, Currency: "USD"},
		{ID: "old", Type: domain.TypeChecking, Currency: "USD", Archived: true},
	}
	balances := map[string]money.Money{
		"chk": money.New(700_000, "USD"),
		"eur": money.New(100_000, "EUR"),
		"brk": money.New(9_000_000, "USD"),
		"old": money.New(1, "USD"),
	}

	t.Run("liquid cash only counts cash-type accounts", func(t *testing.T) {
		got, err := FromLedger(LedgerInputs{Accounts: accounts, Balances: balances, Rates: rates, Now: now, BenchmarkAPRPercent: 4})
		if err != nil {
			t.Fatal(err)
		}
		if got.LiquidMinor != 900_000 || got.IdleMinor != 900_000 {
			t.Fatalf("liquid %d idle %d, want 900000 each", got.LiquidMinor, got.IdleMinor)
		}
		if got.ForgoneAnnualMinor != 36_000 || !got.Flag {
			t.Fatalf("forgone %d flag %v, want 36000 true", got.ForgoneAnnualMinor, got.Flag)
		}
	})

	t.Run("a bill due this month is committed", func(t *testing.T) {
		bill := domain.Recurring{
			ID: "rent", Label: "Rent", Amount: money.New(-300_000, "USD"), Cadence: domain.CadenceMonthly,
			NextDue: now.AddDate(0, 0, 5), AccountID: "chk",
		}
		_, monthEnd := dateutil.MonthRange(now)
		due, err := safespend.BillsDueBeforeChecked(accounts, []domain.Recurring{bill}, now, monthEnd, rates)
		if err != nil || due == 0 {
			t.Fatalf("fixture bill is not due this month: due %d err %v", due, err)
		}
		got, err := FromLedger(LedgerInputs{Accounts: accounts, Balances: balances, Recurring: []domain.Recurring{bill}, Rates: rates, Now: now, BenchmarkAPRPercent: 4})
		if err != nil {
			t.Fatal(err)
		}
		if got.CommittedMinor != due || got.IdleMinor != 900_000-due {
			t.Fatalf("committed %d idle %d, want %d and %d", got.CommittedMinor, got.IdleMinor, due, 900_000-due)
		}
	})

	t.Run("no benchmark raises no flag", func(t *testing.T) {
		got, err := FromLedger(LedgerInputs{Accounts: accounts, Balances: balances, Rates: rates, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		if got.Flag || got.ForgoneAnnualMinor != 0 {
			t.Fatalf("flag %v forgone %d, want none", got.Flag, got.ForgoneAnnualMinor)
		}
	})

	t.Run("a missing balance is an error, not a zero", func(t *testing.T) {
		partial := map[string]money.Money{"chk": balances["chk"]}
		if _, err := FromLedger(LedgerInputs{Accounts: accounts, Balances: partial, Rates: rates, Now: now, BenchmarkAPRPercent: 4}); err == nil {
			t.Fatal("expected an error for a cash account with no computed balance")
		}
	})

	t.Run("a missing exchange rate is an error", func(t *testing.T) {
		if _, err := FromLedger(LedgerInputs{Accounts: accounts, Balances: balances, Rates: currency.Rates{Base: "USD"}, Now: now, BenchmarkAPRPercent: 4}); err == nil {
			t.Fatal("expected an error for an unconvertible cash balance")
		}
	})
}

func TestFromLedgerCountsABillDueLateOnTheLastDayOfTheMonth(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	accounts := []domain.Account{{ID: "chk", Type: domain.TypeChecking, Currency: "USD"}}
	balances := map[string]money.Money{"chk": money.New(1_000_000, "USD")}
	cases := []struct {
		name          string
		due           time.Time
		wantCommitted int64
	}{
		{"last day at midnight", time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), 100_000},
		{"last day late in the evening", time.Date(2026, 10, 31, 23, 59, 0, 0, time.UTC), 100_000},
		{"first of next month", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bill := domain.Recurring{ID: "b", Label: "Bill", Amount: money.New(-100_000, "USD"), Cadence: domain.CadenceYearly, NextDue: tc.due, AccountID: "chk"}
			got, err := FromLedger(LedgerInputs{Accounts: accounts, Balances: balances, Recurring: []domain.Recurring{bill}, Rates: rates, Now: now, BenchmarkAPRPercent: 4})
			if err != nil {
				t.Fatal(err)
			}
			if got.CommittedMinor != tc.wantCommitted {
				t.Fatalf("committed = %d, want %d", got.CommittedMinor, tc.wantCommitted)
			}
		})
	}
}
