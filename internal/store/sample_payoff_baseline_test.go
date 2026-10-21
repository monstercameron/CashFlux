// SPDX-License-Identifier: MIT

package store

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSamplePayoffBaselineMatchesPerAccountReference(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2021, time.July, 1, 0, 0, 0, 0, time.UTC),
		sampleAuthoredNow,
		time.Date(2031, time.December, 31, 0, 0, 0, 0, time.UTC),
	} {
		ds := SampleDatasetAt(now)
		got := ds.Settings.PayoffBaseline
		if got == nil {
			t.Fatalf("at %s: sample payoff baseline missing", now.Format(time.DateOnly))
		}
		want := referenceSamplePayoffBaseline(ds.Accounts, ds.Transactions, got.StartedAt)
		if want == nil {
			t.Fatalf("at %s: reference payoff baseline missing", now.Format(time.DateOnly))
		}
		if got.TotalOwed != want.TotalOwed || got.Currency != want.Currency || !got.StartedAt.Equal(want.StartedAt) {
			t.Errorf("at %s: got %+v, want %+v", now.Format(time.DateOnly), got, want)
		}
	}
}

// referenceSamplePayoffBaseline is the former per-account implementation,
// retained in the test to guard the fixture's baseline semantics.
func referenceSamplePayoffBaseline(accounts []domain.Account, transactions []domain.Transaction, startedAt time.Time) *PayoffBaseline {
	var currentOwed int64
	for _, account := range accounts {
		if account.Archived || account.Class != domain.ClassLiability || !account.IncludedInPayoff() {
			continue
		}
		balance, err := ledger.Balance(account, transactions)
		if err != nil {
			continue
		}
		owed := balance.Amount
		if owed < 0 {
			owed = -owed
		}
		currentOwed += owed
	}
	if currentOwed <= 0 {
		return nil
	}
	return &PayoffBaseline{TotalOwed: currentOwed * 100 / 82, Currency: "USD", StartedAt: startedAt}
}

func TestSamplePayoffBaselineSkipsBalanceErrors(t *testing.T) {
	accounts := []domain.Account{
		{ID: "bad", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(1, "EUR")},
		{ID: "good", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(-8200, "USD")},
	}
	got := samplePayoffBaseline(accounts, nil, time.Time{})
	if got == nil || got.TotalOwed != 10000 {
		t.Fatalf("got %+v, want baseline from valid liability only", got)
	}
}

func TestSamplePayoffBaselineOmitsOverflow(t *testing.T) {
	liability := func(id string, amount int64) domain.Account {
		return domain.Account{ID: id, Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(amount, "USD")}
	}
	tests := []struct {
		name     string
		accounts []domain.Account
	}{
		{name: "absolute minimum", accounts: []domain.Account{liability("debt", math.MinInt64)}},
		{name: "sum overflow", accounts: []domain.Account{liability("one", math.MaxInt64/2+1), liability("two", math.MaxInt64/2+1)}},
		{name: "baseline result overflow", accounts: []domain.Account{liability("debt", math.MaxInt64)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := samplePayoffBaseline(tt.accounts, nil, time.Time{}); got != nil {
				t.Fatalf("got %+v; want no baseline on overflow", got)
			}
		})
	}
}
