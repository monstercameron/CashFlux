// SPDX-License-Identifier: MIT

package ledger

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNetWorthSelectionAgreesAcrossStrictExplainedAndBalances(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.25}}
	accounts := []domain.Account{
		{ID: "cash", Currency: "USD", OpeningBalance: usd(10000)},
		{ID: "eur", Currency: "EUR", OpeningBalance: money.New(400, "EUR")},
		{ID: "negative-debt", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(-1000)},
		{ID: "positive-debt", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(500)},
		{ID: "chosen", Name: "Custodial", Currency: "USD", OpeningBalance: usd(900000), ExcludeFromNetWorth: true},
		{ID: "chosen-debt", Name: "Business debt", Class: domain.ClassLiability, Currency: "EUR", OpeningBalance: money.New(-10000, "EUR"), ExcludeFromNetWorth: true},
		{ID: "archived", Currency: "USD", OpeningBalance: usd(500000), Archived: true},
	}
	cutoffs := []time.Time{mustDate("2026-01-01"), mustDate("2026-02-01"), mustDate("2026-03-01")}
	txns := []domain.Transaction{
		{AccountID: "chosen", Date: mustDate("2026-01-02"), Amount: usd(500000)},
		{AccountID: "cash", Date: cutoffs[0], Amount: usd(100)},
		{AccountID: "eur", Date: mustDate("2026-01-15"), Amount: money.New(4, "EUR")},
		{AccountID: "negative-debt", Date: cutoffs[1], Amount: usd(200)},
		{AccountID: "cash", Date: cutoffs[2], Amount: usd(999999)},
	}
	before, _ := json.Marshal([]any{accounts, txns})
	series, err := NetWorthSeries(accounts, txns, cutoffs, rates)
	if err != nil {
		t.Fatal(err)
	}
	for i, at := range cutoffs {
		var upto []domain.Transaction
		for _, txn := range txns {
			if txn.Date.Before(at) {
				upto = append(upto, txn)
			}
		}
		net, assets, debt, err := NetWorth(accounts, upto, rates)
		if err != nil {
			t.Fatal(err)
		}
		explained, err := NetWorthExplained(accounts, upto, rates)
		if err != nil {
			t.Fatal(err)
		}
		balances, err := Balances(accounts, upto)
		if err != nil {
			t.Fatal(err)
		}
		cached, err := NetWorthFromBalances(accounts, balances, rates)
		if err != nil {
			t.Fatal(err)
		}
		want := []int64{9000, 9105, 9305}[i]
		if net.Amount != want || net != series[i] || net != explained.Net || net != cached.Net || assets != explained.Assets || debt != explained.Liabilities || !reflect.DeepEqual(cached, explained) {
			t.Fatalf("cutoff %d strict=%v/%v/%v explained=%+v cached=%+v series=%v want=%d", i, net, assets, debt, explained, cached, series[i], want)
		}
		if !reflect.DeepEqual(explained.ExcludedByChoice, []string{"Custodial", "Business debt"}) {
			t.Fatalf("disclosure=%v", explained.ExcludedByChoice)
		}
	}
	after, _ := json.Marshal([]any{accounts, txns})
	if string(before) != string(after) {
		t.Fatal("source mutated")
	}
}

func TestNetWorthExcludedAccountsCannotInvalidateStrictTotals(t *testing.T) {
	for _, bad := range []domain.Account{
		{ID: "unrated", Currency: "EUR", OpeningBalance: money.New(100, "EUR")},
		{ID: "mismatch", Currency: "USD", OpeningBalance: money.New(100, "EUR")},
		{ID: "magnitude", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: usd(math.MinInt64)},
		{ID: "balance-overflow", Currency: "USD", OpeningBalance: usd(math.MaxInt64)},
	} {
		t.Run(bad.ID, func(t *testing.T) {
			txns := []domain.Transaction{{AccountID: "balance-overflow", Date: mustDate("2026-01-01"), Amount: usd(1)}}
			for _, archived := range []bool{false, true} {
				bad.ExcludeFromNetWorth, bad.Archived = !archived, archived
				accounts := []domain.Account{{ID: "good", Currency: "USD", OpeningBalance: usd(100)}, bad}
				net, _, _, err := NetWorth(accounts, txns, currency.Rates{Base: "USD"})
				if err != nil || net.Amount != 100 {
					t.Fatalf("excluded/archived bad input: %v %v", net, err)
				}
				series, err := NetWorthSeries(accounts, txns, []time.Time{mustDate("2026-02-01")}, currency.Rates{Base: "USD"})
				if err != nil || len(series) != 1 || series[0] != net {
					t.Fatalf("series=%v err=%v", series, err)
				}
			}
			bad.ExcludeFromNetWorth, bad.Archived = false, false
			_, _, _, err := NetWorth([]domain.Account{bad}, txns, currency.Rates{Base: "USD"})
			if err == nil {
				t.Fatal("included corrupt/unrated account succeeded")
			}
			if bad.ID == "unrated" && !errors.Is(err, currency.ErrUnknownRate) {
				t.Fatalf("missing rate wrapping: %v", err)
			}
			if (bad.ID == "magnitude" || bad.ID == "balance-overflow") && !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("overflow wrapping: %v", err)
			}
		})
	}
}

func TestAccountNetValueSeriesPreservesClassViewsAndCheckedSemantics(t *testing.T) {
	accounts := []domain.Account{
		{ID: "chosen", Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(1000), ExcludeFromNetWorth: true},
		{ID: "chosen-debt", Type: domain.TypeInvestment, Class: domain.ClassLiability, Currency: "EUR", OpeningBalance: money.New(100, "EUR"), ExcludeFromNetWorth: true},
		{ID: "archived", Currency: "USD", OpeningBalance: usd(99999), Archived: true},
	}
	at := mustDate("2026-02-01")
	txns := []domain.Transaction{{AccountID: "chosen", Date: at, Amount: usd(300)}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	got, err := AccountNetValueSeries(accounts, txns, []time.Time{at, at.Add(time.Nanosecond)}, rates)
	if err != nil || !reflect.DeepEqual(got, []money.Money{usd(800), usd(1100)}) {
		t.Fatalf("class values=%v err=%v", got, err)
	}
	nw, err := NetWorthSeries(accounts, txns, []time.Time{at}, rates)
	if err != nil || nw[0].Amount != 0 {
		t.Fatalf("household values=%v err=%v", nw, err)
	}
	if !accounts[0].ExcludeFromNetWorth || !accounts[1].ExcludeFromNetWorth {
		t.Fatal("class calculation cleared persisted flags")
	}
	_, err = AccountNetValueSeries(accounts, txns, []time.Time{at}, currency.Rates{Base: "USD"})
	if !errors.Is(err, currency.ErrUnknownRate) {
		t.Fatalf("explicit class values must fail unknown FX: %v", err)
	}
}

func TestStrictNetWorthPreservesFirstFailureAndDuplicateIDBalances(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	_, _, _, err := NetWorth([]domain.Account{
		{ID: "first-unrated", Currency: "EUR", OpeningBalance: money.New(100, "EUR")},
		{ID: "later-corrupt", Currency: "USD", OpeningBalance: money.New(100, "GBP")},
	}, nil, rates)
	if !errors.Is(err, currency.ErrUnknownRate) {
		t.Fatalf("strict first error changed: %v", err)
	}
	accounts := []domain.Account{
		{ID: "same", Currency: "USD", OpeningBalance: usd(10)},
		{ID: "same", Currency: "USD", OpeningBalance: usd(20)},
		{ID: "same", Currency: "USD", OpeningBalance: usd(50), ExcludeFromNetWorth: true},
	}
	txns := []domain.Transaction{{AccountID: "same", Date: mustDate("2026-01-01"), Amount: usd(3)}}
	net, _, _, err := NetWorth(accounts, txns, rates)
	if err != nil || net.Amount != 36 {
		t.Fatalf("duplicate ID strict=%v err=%v", net, err)
	}
	explained, err := NetWorthExplained(accounts, txns, rates)
	if err != nil || explained.Net != net {
		t.Fatalf("duplicate ID explained=%+v err=%v", explained, err)
	}
	class, err := AccountNetValueSeries(accounts, txns, []time.Time{mustDate("2026-02-01")}, rates)
	if err != nil || class[0].Amount != 89 {
		t.Fatalf("duplicate ID class=%v err=%v", class, err)
	}
}
