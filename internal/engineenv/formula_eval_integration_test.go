// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestFormulaEvalOverEngineVars validates the exact chain the assistant's
// evaluate_formula tool relies on: Vars() produces the atom+molecule surface in
// MAJOR units, formula.Eval resolves both a bare molecule name and an arithmetic
// expression over it, and Explain traces a molecule to its atom inputs.
func TestFormulaEvalOverEngineVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	acct := func(id string, ty domain.AccountType, class domain.AccountClass, minor int64) domain.Account {
		return domain.Account{ID: id, Type: ty, Class: class, Currency: "USD",
			OpeningBalance: money.New(minor, "USD"), BalanceAsOf: asOf}
	}
	d := Data{
		Accounts: []domain.Account{
			acct("a1", domain.TypeChecking, domain.ClassAsset, 3000000),      // $30,000
			acct("l1", domain.TypeCreditCard, domain.ClassLiability, -50000), // $500 owed
		},
		Rates: currency.Rates{Base: "USD"}, Now: now,
	}
	vars := Vars(d)

	// Atoms are in major units.
	if got := vars["assets"]; got != 30000 {
		t.Fatalf("assets atom = %v, want 30000 (major units)", got)
	}
	if got := vars["liabilities"]; got != 500 {
		t.Fatalf("liabilities atom = %v, want 500", got)
	}

	env := formula.Env{Vars: vars}
	// A bare molecule name resolves to its value.
	if v, err := formula.Eval("net_worth", env); err != nil || v != float64(29500) {
		t.Fatalf("Eval(net_worth) = %v, err %v; want 29500", v, err)
	}
	// An arithmetic expression over atoms + molecules.
	if v, err := formula.Eval("assets - liabilities", env); err != nil || v != float64(29500) {
		t.Fatalf("Eval(assets - liabilities) = %v, err %v; want 29500", v, err)
	}
	if v, err := formula.Eval("net_worth * 0.04 / 12", env); err != nil {
		t.Fatalf("Eval(net_worth * 0.04 / 12) err %v", err)
	} else if got, _ := v.(float64); got < 98 || got > 99 {
		t.Fatalf("Eval(net_worth * 0.04 / 12) = %v, want ~98.33", v)
	}

	// Explain traces a molecule to its atom inputs.
	if der, ok := Explain("net_worth", vars, nil); !ok {
		t.Fatal("Explain(net_worth) not found")
	} else {
		if der.Kind != "molecule" {
			t.Errorf("net_worth Kind = %q, want molecule", der.Kind)
		}
		if der.Formula == "" {
			t.Error("net_worth molecule has no formula")
		}
		if len(der.Inputs) == 0 {
			t.Error("net_worth molecule has no traced inputs")
		}
	}
	// An atom explains to a source description, not a formula.
	if der, ok := Explain("assets", vars, nil); !ok || der.Kind != "atom" || der.Source == "" {
		t.Errorf("Explain(assets) = %+v ok=%v, want atom with a source", der, ok)
	}
}
