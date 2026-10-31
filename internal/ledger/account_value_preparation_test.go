// SPDX-License-Identifier: MIT

package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// frozenAccountValueSeries is the prior implementation: copy the full ledger
// before every cutoff, then scan that copy in Balance once per account record.
// It deliberately does not use the new indexing or cutoff balance helpers.
func frozenAccountValueSeries(accounts []domain.Account, all []domain.Transaction, cutoffs []time.Time, rates currency.Rates, policy accountTotalsPolicy, label string) ([]money.Money, error) {
	out := make([]money.Money, len(cutoffs))
	for i, c := range cutoffs {
		upto := make([]domain.Transaction, 0, len(all))
		for _, txn := range all {
			if txn.Date.Before(c) {
				upto = append(upto, txn)
			}
		}
		res, err := strictAccountTotals(accounts, upto, rates, policy)
		if err != nil {
			return nil, fmt.Errorf("ledger: %s as of %s: %w", label, c.Format(dateutil.Layout), err)
		}
		out[i] = res.Net
	}
	return out, nil
}

func assertAccountSeriesParity(t *testing.T, accounts []domain.Account, txns []domain.Transaction, cutoffs []time.Time, rates currency.Rates, policy accountTotalsPolicy, label string) ([]money.Money, error) {
	t.Helper()
	want, wantErr := frozenAccountValueSeries(accounts, txns, cutoffs, rates, policy, label)
	got, gotErr := accountValueSeries(accounts, txns, cutoffs, rates, policy, label)
	if !reflect.DeepEqual(got, want) || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
		t.Fatalf("indexed=%v err=%v; frozen=%v err=%v", got, gotErr, want, wantErr)
	}
	for _, target := range []error{money.ErrCurrencyMismatch, money.ErrOverflow, currency.ErrUnknownRate, currency.ErrInvalidRate} {
		if errors.Is(gotErr, target) != errors.Is(wantErr, target) {
			t.Fatalf("error wrapping differs for %v: %v vs %v", target, gotErr, wantErr)
		}
	}
	return got, gotErr
}

func TestAccountValuePreparationAdversarialParity(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC) }
	a := func(id string, n int64) domain.Account {
		return domain.Account{ID: id, Currency: "USD", OpeningBalance: usd(n)}
	}
	txn := func(id string, d int, n int64) domain.Transaction {
		return domain.Transaction{AccountID: id, Date: day(d), Amount: usd(n)}
	}
	chosen, archived := a("chosen", 99999), a("archived", 88888)
	chosen.ExcludeFromNetWorth, archived.Archived = true, true
	negative, positive := a("negative", -400), a("positive", 300)
	negative.Class, positive.Class = domain.ClassLiability, domain.ClassLiability
	eur := domain.Account{ID: "eur", Currency: "EUR", OpeningBalance: money.New(1, "EUR")}
	unrated := domain.Account{ID: "unrated", Currency: "GBP", OpeningBalance: money.New(10, "GBP")}
	badOpening := domain.Account{ID: "bad-opening", Currency: "USD", OpeningBalance: money.New(1, "EUR")}
	wrongTxn := txn("ordered", 15, 1)
	wrongTxn.Amount.Currency = "EUR"
	cutoffs := []time.Time{day(20), day(10), day(20), day(5), day(31)}
	tests := []struct {
		name        string
		accounts    []domain.Account
		txns        []domain.Transaction
		cutoffs     []time.Time
		rates       currency.Rates
		want        []int64
		errContains string
	}{
		{name: "empty cutoffs never validate bad data", accounts: []domain.Account{badOpening}, txns: []domain.Transaction{wrongTxn}},
		{name: "empty accounts ignore malformed unrelated rows", txns: []domain.Transaction{wrongTxn}, cutoffs: cutoffs, want: []int64{0, 0, 0, 0, 0}},
		{name: "unsorted repeated cutoffs and source rows", accounts: []domain.Account{a("a", 100)}, txns: []domain.Transaction{txn("a", 20, 7), txn("a", 4, 2), txn("a", 10, 3), txn("unknown", 1, math.MaxInt64)}, cutoffs: cutoffs, want: []int64{105, 102, 105, 102, 112}},
		{name: "duplicate IDs retain opening balances", accounts: []domain.Account{a("same", 10), a("same", 20)}, txns: []domain.Transaction{txn("same", 2, 3)}, cutoffs: []time.Time{day(3)}, want: []int64{36}},
		{name: "blank ID still matches its rows", accounts: []domain.Account{a("", 10)}, txns: []domain.Transaction{txn("", 2, 3)}, cutoffs: []time.Time{day(3)}, want: []int64{13}},
		{name: "chosen and archived selection", accounts: []domain.Account{a("a", 100), chosen, archived}, txns: []domain.Transaction{txn("chosen", 2, 1), txn("archived", 2, 1)}, cutoffs: []time.Time{day(3)}},
		{name: "liability signs and per-account FX rounding", accounts: []domain.Account{negative, positive, eur, {ID: "eur2", Currency: "EUR", OpeningBalance: money.New(1, "EUR")}}, cutoffs: []time.Time{day(3)}, rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": .4}}, want: []int64{-700}},
		{name: "irrelevant corrupt chosen and archived accounts", accounts: []domain.Account{a("a", 100), {ID: "bad", Currency: "USD", OpeningBalance: money.New(1, "EUR"), Archived: true}, {ID: "bad2", Currency: "GBP", OpeningBalance: money.New(1, "GBP"), ExcludeFromNetWorth: true}}, cutoffs: []time.Time{day(3)}},
		{name: "strict first account error beats row encounter order", accounts: []domain.Account{unrated, a("later", math.MaxInt64)}, txns: []domain.Transaction{txn("later", 1, 1)}, cutoffs: []time.Time{day(3)}, errContains: "GBP"},
		{name: "opening mismatch before transaction arithmetic", accounts: []domain.Account{badOpening}, txns: []domain.Transaction{txn("bad-opening", 1, math.MaxInt64)}, cutoffs: []time.Time{day(3)}, errContains: "opening balance currency"},
		{name: "intermediate overflow before later cancellation", accounts: []domain.Account{a("ordered", math.MaxInt64-1)}, txns: []domain.Transaction{txn("ordered", 15, 2), txn("ordered", 10, -2)}, cutoffs: []time.Time{day(12), day(20)}, errContains: "2026-07-20"},
		{name: "unsorted cutoffs retain first failure context", accounts: []domain.Account{a("ordered", math.MaxInt64-1)}, txns: []domain.Transaction{txn("ordered", 15, 2), txn("ordered", 10, -2)}, cutoffs: []time.Time{day(20), day(12)}, errContains: "2026-07-20"},
		{name: "source mismatch precedes earlier-dated overflow", accounts: []domain.Account{a("ordered", math.MaxInt64)}, txns: []domain.Transaction{wrongTxn, txn("ordered", 10, 1)}, cutoffs: []time.Time{day(20)}, errContains: "currency mismatch"},
		{name: "future malformed row stays outside cutoff", accounts: []domain.Account{a("ordered", 10)}, txns: []domain.Transaction{wrongTxn, txn("ordered", 2, 1)}, cutoffs: []time.Time{day(15)}, want: []int64{11}},
		{name: "checked liability magnitude", accounts: []domain.Account{{ID: "min", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(math.MinInt64)}}, cutoffs: []time.Time{day(3)}, errContains: "liability magnitude"},
		{name: "checked account iteration total overflow", accounts: []domain.Account{a("first", math.MaxInt64), a("second", 1), a("third", -1)}, cutoffs: []time.Time{day(3)}, errContains: "amount overflow"},
		{name: "blank opening currency preserves normalization", accounts: []domain.Account{{ID: "blank", Currency: "USD", OpeningBalance: money.Money{Amount: 99}}}, cutoffs: []time.Time{day(3)}, want: []int64{0}},
		{name: "invalid FX result remains an error", accounts: []domain.Account{eur}, cutoffs: []time.Time{day(3)}, rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": math.Inf(1)}}, errContains: "finite and positive"},
	}
	for _, tc := range tests {
		for _, classView := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/class=%v", tc.name, classView), func(t *testing.T) {
				rates := tc.rates
				if rates.Base == "" {
					rates.Base = "USD"
				}
				before, _ := json.Marshal([]any{tc.accounts, tc.txns, tc.cutoffs})
				got, err := assertAccountSeriesParity(t, tc.accounts, tc.txns, tc.cutoffs, rates, accountTotalsPolicy{includeChosenAccounts: classView}, "value")
				if tc.errContains != "" && (err == nil || !strings.Contains(err.Error(), tc.errContains)) {
					t.Fatalf("expected error %q, got %v", tc.errContains, err)
				}
				if tc.want != nil {
					if err != nil || len(got) != len(tc.want) {
						t.Fatalf("got=%v err=%v want=%v", got, err, tc.want)
					}
					for i := range got {
						if got[i].Amount != tc.want[i] {
							t.Fatalf("point %d=%v want=%d", i, got[i], tc.want[i])
						}
					}
				}
				after, _ := json.Marshal([]any{tc.accounts, tc.txns, tc.cutoffs})
				if string(after) != string(before) {
					t.Fatal("source mutated")
				}
			})
		}
	}
}

func TestAccountValuePreparationSeededParity(t *testing.T) {
	random := rand.New(rand.NewPCG(472, 991))
	amounts := []int64{0, 1, -1, 100, -100, math.MaxInt64, math.MinInt64}
	currencies := []string{"USD", "EUR", "JPY", "GBP", ""}
	at := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": .4, "JPY": .01}}
	for fixture := range 400 {
		accounts := make([]domain.Account, random.IntN(7))
		for i := range accounts {
			code := currencies[random.IntN(len(currencies)-1)]
			a := domain.Account{ID: fmt.Sprint(random.IntN(4)), Currency: code, OpeningBalance: money.New(amounts[random.IntN(len(amounts))], code), Archived: random.IntN(7) == 0, ExcludeFromNetWorth: random.IntN(4) == 0}
			if random.IntN(3) == 0 {
				a.Class = domain.ClassLiability
			}
			if random.IntN(5) == 0 {
				a.OpeningBalance.Currency = currencies[random.IntN(len(currencies))]
			}
			accounts[i] = a
		}
		txns := make([]domain.Transaction, random.IntN(30))
		for i := range txns {
			txns[i] = domain.Transaction{AccountID: fmt.Sprint(random.IntN(6)), Date: at.Add(time.Duration(random.IntN(5)) * time.Hour), Amount: money.New(amounts[random.IntN(len(amounts))], currencies[random.IntN(len(currencies))])}
		}
		cutoffs := make([]time.Time, random.IntN(7))
		for i := range cutoffs {
			cutoffs[i] = at.Add(time.Duration(random.IntN(6)) * time.Hour)
		}
		for _, classView := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/class=%v", fixture, classView), func(t *testing.T) {
				assertAccountSeriesParity(t, accounts, txns, cutoffs, rates, accountTotalsPolicy{includeChosenAccounts: classView}, "net worth")
			})
		}
	}
}
