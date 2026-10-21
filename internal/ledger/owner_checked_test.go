// SPDX-License-Identifier: MIT

package ledger

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSplitBySharesCheckedExtremeAmountsAndImmutableWeights(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount int64
		shares map[string]int
		want   map[string]int64
	}{
		{"maximum half", math.MaxInt64, map[string]int{"bob": 50, "alice": 50}, map[string]int64{"alice": 4611686018427387904, "bob": 4611686018427387903}},
		{"minimum half", math.MinInt64, map[string]int{"bob": 50, "alice": 50}, map[string]int64{"alice": -4611686018427387904, "bob": -4611686018427387904}},
		{"maximum fractional", math.MaxInt64, map[string]int{"alice": 60, "bob": 40}, map[string]int64{"alice": 5534023222112865484, "bob": 3689348814741910323}},
		{"minimum fractional", math.MinInt64, map[string]int{"alice": 60, "bob": 40}, map[string]int64{"alice": -5534023222112865485, "bob": -3689348814741910323}},
		{"minimum single", math.MinInt64, map[string]int{"alice": 100}, map[string]int64{"alice": math.MinInt64}},
		{"negative remainder tie", -1, map[string]int{"bob": 50, "alice": 50}, map[string]int64{"alice": -1, "bob": 0}},
		{"zero weight", math.MaxInt64, map[string]int{"alice": 0, "bob": 100}, map[string]int64{"alice": 0, "bob": math.MaxInt64}},
		{"empty", 1, nil, map[string]int64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := json.Marshal(tc.shares)
			got, err := SplitBySharesChecked(tc.amount, tc.shares)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got%v,%v want%v", got, err, tc.want)
			}
			after, _ := json.Marshal(tc.shares)
			if string(before) != string(after) {
				t.Fatal("ownership weights mutated")
			}
			if len(tc.shares) > 0 {
				var values []int64
				for _, part := range got {
					values = append(values, part)
				}
				sum, err := money.SumMinor(values...)
				if err != nil || sum != tc.amount {
					t.Fatalf("parts not conserved: %d,%v", sum, err)
				}
			}
		})
	}
}

func TestSplitBySharesCheckedRejectsInvalidAndPreservesOrdinaryHamilton(t *testing.T) {
	for _, weights := range []map[string]int{{"a": -1, "b": 101}, {"a": 101}, {"a": 99}, {"a": 0}, {"a": math.MaxInt, "b": math.MaxInt}} {
		got, err := SplitBySharesChecked(1, weights)
		if !errors.Is(err, money.ErrInvalidWeights) || got != nil {
			t.Fatalf("bad weights %v returned %v,%v", weights, got, err)
		}
	}
	for _, weights := range []map[string]int{{"a": 60, "b": 40}, {"a": 33, "b": 33, "c": 34}, {"b": 50, "a": 50}, {"a": 0, "b": 100}} {
		for amount := int64(-105); amount <= 105; amount++ {
			got, err := SplitBySharesChecked(amount, weights)
			want := SplitByShares(amount, weights)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("ordinary Hamilton changed for %d/%v: got%v,%v want%v", amount, weights, got, err, want)
			}
		}
	}
}

func TestNetByOwnerCheckedFractionsAndInputs(t *testing.T) {
	for _, amount := range []int64{math.MinInt64, math.MaxInt64} {
		accounts := []domain.Account{{ID: "joint", OwnerID: domain.GroupOwnerID, Currency: "USD", OpeningBalance: money.New(amount, "USD"), OwnershipShares: map[string]int{"alice": 60, "bob": 40}}}
		rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}}
		before, _ := json.Marshal(struct {
			Accounts []domain.Account
			Rates    currency.Rates
		}{accounts, rates})
		got, err := NetByOwner(accounts, nil, rates)
		want, splitErr := SplitBySharesChecked(amount, accounts[0].OwnershipShares)
		if err != nil || splitErr != nil || len(got) != 2 || got["alice"] != money.New(want["alice"], "USD") || got["bob"] != money.New(want["bob"], "USD") {
			t.Fatalf("NetByOwner bypassed checked fractions: %v,%v", got, err)
		}
		after, _ := json.Marshal(struct {
			Accounts []domain.Account
			Rates    currency.Rates
		}{accounts, rates})
		if string(before) != string(after) {
			t.Fatal("owner preparation mutated accounts/shares/rates")
		}
	}
	// Conversion rounds once at account level, before its ownership split.
	accounts := []domain.Account{{ID: "joint", Currency: "EUR", OpeningBalance: money.New(1, "EUR"), OwnershipShares: map[string]int{"alice": 50, "bob": 50}}}
	got, err := NetByOwner(accounts, nil, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}})
	if err != nil || got["alice"].Amount != 1 || got["bob"].Amount != 1 {
		t.Fatalf("conversion/split rounding order changed: %v,%v", got, err)
	}
}

func TestNetByOwnerCheckedErrorsNeverReturnPartialOwners(t *testing.T) {
	account := func(id string, amount int64, weights map[string]int) domain.Account {
		return domain.Account{ID: id, OwnerID: "alice", Currency: "USD", OpeningBalance: money.New(amount, "USD"), OwnershipShares: weights}
	}
	for _, tc := range []struct {
		name     string
		accounts []domain.Account
		want     error
	}{
		{"invalid shares", []domain.Account{account("good", 1, nil), account("bad", 1, map[string]int{"alice": 99})}, money.ErrInvalidWeights},
		{"fractional owner sum", []domain.Account{account("first", math.MaxInt64, map[string]int{"alice": 50, "bob": 50}), account("second", math.MaxInt64, map[string]int{"alice": 50, "bob": 50})}, money.ErrOverflow},
		{"single owner sum", []domain.Account{account("first", math.MaxInt64, nil), account("second", 1, nil)}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NetByOwner(tc.accounts, nil, currency.Rates{Base: "USD"})
			if !errors.Is(err, tc.want) || got != nil {
				t.Fatalf("got%v,%v want no partial map and %v", got, err, tc.want)
			}
		})
	}
	bad := account("archived-bad", 1, map[string]int{"alice": 99})
	bad.Archived = true
	got, err := NetByOwner([]domain.Account{bad}, nil, currency.Rates{Base: "USD"})
	if err != nil || len(got) != 0 {
		t.Fatalf("archived selection changed: %v,%v", got, err)
	}
	foreign := domain.Account{ID: "foreign", Currency: "EUR", OpeningBalance: money.New(1, "EUR"), OwnershipShares: map[string]int{"alice": 100}}
	got, err = NetByOwner([]domain.Account{foreign}, nil, currency.Rates{Base: "USD"})
	if !errors.Is(err, currency.ErrUnknownRate) || got != nil {
		t.Fatalf("FX failure lost: %v,%v", got, err)
	}
}
