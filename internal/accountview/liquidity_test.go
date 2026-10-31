// SPDX-License-Identifier: MIT

package accountview

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestLiquidityMatchesExistingBucketsAndExactLockExpiry(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "cash", Currency: "USD", Type: domain.TypeChecking},
		{ID: "locked", Currency: "EUR", Type: domain.TypeSavings, LockUntil: now.Add(time.Hour)},
		{ID: "investments", Currency: "USD", Type: domain.TypeInvestment},
		{ID: "held", Currency: "USD", Type: domain.TypeProperty},
		{ID: "archived", Currency: "USD", Archived: true},
		{ID: "debt", Currency: "USD", Class: domain.ClassLiability},
	}
	balances := map[string]money.Money{}
	for _, a := range accounts {
		balances[a.ID] = money.New(100, a.Currency)
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	want := liquidity.Totals(accounts, now, func(a domain.Account) (int64, bool) {
		c, err := rates.Convert(balances[a.ID], "USD")
		return c.Amount, err == nil
	})
	got := Liquidity(accounts, balances, rates, "USD", now)
	if !reflect.DeepEqual(got.Totals, want) || got.Incomplete {
		t.Fatalf("liquidity = %+v, want %v", got, want)
	}
	got = Liquidity(accounts, balances, rates, "USD", accounts[1].LockUntil)
	if got.Totals[liquidity.Available] != 300 || got.Totals[liquidity.Restricted] != 0 {
		t.Fatalf("lock did not expire at boundary: %+v", got)
	}
}

func TestLiquidityDisclosesInvalidFXBalancesAndOverflow(t *testing.T) {
	accounts := []domain.Account{{ID: "huge", Currency: "USD", Type: domain.TypeChecking}, {ID: "overflow", Currency: "USD", Type: domain.TypeChecking}, {ID: "fx", Currency: "GBP"}, {ID: "invalid", Currency: "EUR"}}
	balances := map[string]money.Money{"huge": money.New(math.MaxInt64, "USD"), "overflow": money.New(1, "USD"), "fx": money.New(100, "GBP")}
	got := Liquidity(accounts, balances, currency.Rates{Base: "USD"}, "USD", time.Time{})
	if got.Totals[liquidity.Available] != math.MaxInt64 || !got.Incomplete || !got.Overflow || !reflect.DeepEqual(got.ExcludedCurrencies, []string{"EUR", "GBP"}) {
		t.Fatalf("omissions not explicit: %+v", got)
	}
}

func TestSnapshotNetWorthMatchesLedgerWithExclusionsAndInvalidBalances(t *testing.T) {
	accounts := []domain.Account{
		{ID: "usd", Name: "Cash", Currency: "USD", OpeningBalance: money.New(100, "USD")},
		{ID: "eur", Name: "Euro", Currency: "EUR", OpeningBalance: money.New(100, "EUR")},
		{ID: "debt", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(-50, "USD")},
		{ID: "omitted", Currency: "USD", ExcludeFromNetWorth: true, OpeningBalance: money.New(999, "USD")},
	}
	rates := currency.Rates{Base: "USD"}
	s := Build(accounts, nil)
	got, ge := ledger.NetWorthFromBalances(accounts, s.Balances, rates)
	want, we := ledger.NetWorthExplained(accounts, nil, rates)
	if ge != nil || we != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot net worth = %+v, %v; want %+v, %v", got, ge, want, we)
	}
	accounts[0].OpeningBalance = money.New(100, "GBP")
	s = Build(accounts, nil)
	if _, ok := s.Balances["usd"]; ok {
		t.Fatal("invalid balance retained as valid zero")
	}
	if _, err := ledger.NetWorthFromBalances(accounts, s.Balances, rates); err == nil {
		t.Fatal("invalid balance silently included")
	}
}
