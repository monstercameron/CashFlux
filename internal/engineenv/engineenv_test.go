// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestVars(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	d := Data{
		Accounts: []domain.Account{
			{ID: "a1", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
				OpeningBalance: money.New(100000, "USD"), BalanceAsOf: asOf},
			{ID: "a2", Class: domain.ClassAsset, Currency: "USD", Archived: true,
				OpeningBalance: money.New(50000, "USD"), BalanceAsOf: asOf},
		},
		Transactions: []domain.Transaction{
			{ID: "t1", AccountID: "a1", Date: now, Amount: money.New(420000, "USD")}, // income
			{ID: "t2", AccountID: "a1", Date: now, Amount: money.New(-5000, "USD")},  // expense
		},
		Members: []domain.Member{{ID: "m1"}},
		Budgets: []domain.Budget{{ID: "b1"}, {ID: "b2"}},
		Goals:   []domain.Goal{{ID: "g1"}},
		Tasks:   []domain.Task{{ID: "k1"}, {ID: "k2"}, {ID: "k3"}},
		Rates:   currency.Rates{Base: "USD", Rates: map[string]float64{}},
		Now:     now,
	}

	v := Vars(d)

	// Counts: one archived account is excluded; others are plain lengths.
	if v["accounts"] != 1 {
		t.Errorf("accounts = %v, want 1 (archived excluded)", v["accounts"])
	}
	if v["transactions"] != 2 || v["members"] != 1 || v["budgets"] != 2 ||
		v["goals"] != 1 || v["tasks"] != 3 {
		t.Errorf("counts wrong: %+v", v)
	}
	// Money is in major units (USD has 2 decimals → /100).
	if v["income"] != 4200 {
		t.Errorf("income = %v, want 4200", v["income"])
	}
	if v["expense"] != 50 {
		t.Errorf("expense = %v, want 50 (positive magnitude)", v["expense"])
	}
	// Determinism: same input → same output.
	if v2 := Vars(d); v2["net_worth"] != v["net_worth"] {
		t.Errorf("not deterministic: %v vs %v", v2["net_worth"], v["net_worth"])
	}
	// Every documented name is present.
	for _, n := range Names {
		if _, ok := v[n]; !ok {
			t.Errorf("missing variable %q", n)
		}
	}
}

func TestVarsDefaultsBaseCurrency(t *testing.T) {
	// No base currency set: should not panic and should default sensibly.
	v := Vars(Data{Now: time.Now()})
	if len(v) != len(Names) {
		t.Errorf("got %d vars, want %d", len(v), len(Names))
	}
}

func TestSortedNames(t *testing.T) {
	s := SortedNames()
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			t.Errorf("SortedNames not sorted: %v", s)
			break
		}
	}
	if len(s) != len(Names) {
		t.Errorf("SortedNames len %d, want %d", len(s), len(Names))
	}
}
