// SPDX-License-Identifier: MIT

package networthview

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func account(id, code string, amount int64, liability bool) domain.Account {
	class := domain.ClassAsset
	if liability {
		class = domain.ClassLiability
	}
	return domain.Account{ID: id, Name: id, Currency: code, Class: class, OpeningBalance: money.New(amount, code)}
}

func TestNetWorthSnapshotDistinguishesExactZeroPartialAndFatal(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	for _, test := range []struct {
		name           string
		accounts       []domain.Account
		known, partial bool
		net            int64
	}{
		{"true zero", []domain.Account{account("zero", "USD", 0, false)}, true, false, 0},
		{"complete", []domain.Account{account("bank", "USD", 100, false), account("loan", "USD", -25, true)}, true, false, 75},
		{"missing asset FX", []domain.Account{account("bank", "USD", 100, false), account("foreign", "EUR", 200, false)}, true, true, 100},
		{"missing debt FX", []domain.Account{account("bank", "USD", 100, false), account("foreign", "EUR", 200, true)}, true, true, 100},
		{"all FX missing", []domain.Account{account("foreign", "EUR", 200, false)}, true, true, 0},
		{"unavailable balance", []domain.Account{{ID: "bad", Currency: "USD", OpeningBalance: money.New(100, "EUR")}}, false, false, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := append([]domain.Account(nil), test.accounts...)
			got := Build(test.accounts, nil, rates)
			balances, _ := ledger.Balances(test.accounts, nil)
			indexed := FromBalances(test.accounts, balances, rates)
			for _, view := range []Snapshot{got, indexed} {
				if view.Known != test.known || view.Partial != test.partial || view.Net.Amount != test.net || view.Complete() != (test.known && !test.partial) || (view.Err != nil) == test.known {
					t.Fatalf("wrong availability: %+v", view)
				}
				if view.Partial && (len(view.ExcludedAccounts) != 1 || !reflect.DeepEqual(view.MissingCurrencies, []string{"EUR"})) {
					t.Fatalf("partial result lost its disclosure: %+v", view)
				}
			}
			if !reflect.DeepEqual(test.accounts, before) {
				t.Fatal("snapshot mutated source accounts")
			}
		})
	}
}

func TestNetWorthSnapshotFatalErrorsNeverBecomeKnownMoney(t *testing.T) {
	for _, test := range []struct {
		name         string
		accounts     []domain.Account
		transactions []domain.Transaction
		rates        currency.Rates
		cause        error
	}{
		{"transaction currency", []domain.Account{account("a", "USD", 100, false)}, []domain.Transaction{{AccountID: "a", Amount: money.New(1, "EUR")}}, currency.Rates{Base: "USD"}, money.ErrCurrencyMismatch},
		{"account sum", []domain.Account{account("a", "USD", math.MaxInt64, false)}, []domain.Transaction{{AccountID: "a", Amount: money.New(1, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"asset sum", []domain.Account{account("a", "USD", math.MaxInt64, false), account("b", "USD", 1, false)}, nil, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"net subtraction", []domain.Account{account("a", "USD", math.MinInt64, false), account("b", "USD", 1, true)}, nil, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"FX overflow", []domain.Account{account("a", "EUR", math.MaxInt64, false)}, nil, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, money.ErrOverflow},
		{"invalid FX", []domain.Account{account("a", "EUR", 100, false)}, nil, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": math.NaN()}}, currency.ErrInvalidRate},
		{"liability magnitude", []domain.Account{account("a", "USD", math.MinInt64, true)}, nil, currency.Rates{Base: "USD"}, money.ErrOverflow},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := Build(test.accounts, test.transactions, test.rates)
			if got.Known || got.Partial || got.Complete() || !errors.Is(got.Err, test.cause) || got.Net.Currency != "" {
				t.Fatalf("fatal failure became usable money: %+v", got)
			}
		})
	}
}

func TestNetWorthSnapshotPreservesIntentionalExclusions(t *testing.T) {
	excluded := account("excluded", "USD", 100, false)
	excluded.ExcludeFromNetWorth, excluded.OpeningBalance = true, money.New(1, "EUR")
	archived := excluded
	archived.ID, archived.Name, archived.Archived = "archived", "archived", true
	view := Build([]domain.Account{account("included", "USD", 100, false), excluded, archived}, nil, currency.Rates{Base: "USD"})
	if !view.Complete() || view.Net.Amount != 100 || !reflect.DeepEqual(view.ExcludedByChoice, []string{"excluded"}) {
		t.Fatalf("irrelevant malformed balances invalidated the stated total: %+v", view)
	}
}
