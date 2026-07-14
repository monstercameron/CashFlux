// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// reliabilityData is a minimal-but-real dataset for the molecule-pass tests.
func reliabilityData(mols []domain.Molecule) Data {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return Data{
		Accounts: []domain.Account{
			{ID: "a1", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
				OpeningBalance: money.New(100000, "USD"), BalanceAsOf: asOf},
		},
		Transactions: []domain.Transaction{
			{ID: "t1", AccountID: "a1", Date: now, Amount: money.New(420000, "USD")},
			{ID: "t2", AccountID: "a1", Date: now, Amount: money.New(-5000, "USD")},
		},
		Rates:     currency.Rates{Base: "USD", Rates: map[string]float64{}},
		Now:       now,
		Molecules: mols,
	}
}

// TestMoleculeOrderIndependence: a molecule referencing a sibling declared
// AFTER it must still resolve — persisted customs load in name order, so
// "a_summary depends on z_base" is the everyday case, not the exception.
func TestMoleculeOrderIndependence(t *testing.T) {
	mols := append(DefaultMolecules(),
		domain.Molecule{Name: "a_summary", Formula: "z_base * 2"},
		domain.Molecule{Name: "z_base", Formula: "income - expense"},
	)
	v := Vars(reliabilityData(mols))
	if v["z_base"] != 4150 {
		t.Fatalf("z_base = %v, want 4150", v["z_base"])
	}
	if v["a_summary"] != 8300 {
		t.Errorf("a_summary = %v, want 8300 (forward reference must resolve)", v["a_summary"])
	}

	// A three-link chain in fully reversed order.
	mols = append(DefaultMolecules(),
		domain.Molecule{Name: "c3", Formula: "c2 + 1"},
		domain.Molecule{Name: "c2", Formula: "c1 + 1"},
		domain.Molecule{Name: "c1", Formula: "income"},
	)
	v = Vars(reliabilityData(mols))
	if v["c3"] != 4202 {
		t.Errorf("c3 = %v, want 4202 (chained forward references must resolve)", v["c3"])
	}
}

// TestMoleculeCycleTerminates: a reference cycle must not hang or panic; the
// cyclic molecules stay unset and everything else still computes.
func TestMoleculeCycleTerminates(t *testing.T) {
	mols := append(DefaultMolecules(),
		domain.Molecule{Name: "cy_a", Formula: "cy_b + 1"},
		domain.Molecule{Name: "cy_b", Formula: "cy_a + 1"},
		domain.Molecule{Name: "cy_self", Formula: "cy_self * 2"},
		domain.Molecule{Name: "fine", Formula: "income * 2"},
	)
	v := Vars(reliabilityData(mols))
	for _, name := range []string{"cy_a", "cy_b", "cy_self"} {
		if _, ok := v[name]; ok {
			t.Errorf("cyclic molecule %q should stay unset, got %v", name, v[name])
		}
	}
	if v["fine"] != 8400 {
		t.Errorf("fine = %v, want 8400 (healthy molecules must survive a cycle elsewhere)", v["fine"])
	}
	if v["net_worth"] == 0 {
		t.Error("defaults must still compute alongside a cycle")
	}
}

// TestMoleculeCannotShadowAtom: a molecule named after an atom (possible via
// dataset import, which skips form validation) is ignored — the atom wins.
func TestMoleculeCannotShadowAtom(t *testing.T) {
	mols := append(DefaultMolecules(),
		domain.Molecule{Name: "assets", Formula: "0 - 999999"},
	)
	v := Vars(reliabilityData(mols))
	if v["assets"] != 5150 { // 1000.00 opening + 4200.00 income − 50.00 expense
		t.Errorf("assets = %v, want 5150 (molecule must not overwrite the atom)", v["assets"])
	}
	if v["net_worth"] != 5150 {
		t.Errorf("net_worth = %v, want 5150 (derived from the real atom)", v["net_worth"])
	}
}

// TestMoleculeUnknownReferenceSkipped: a molecule with a typo'd reference stays
// unset (no phantom 0-valued entry) and doesn't disturb its neighbours.
func TestMoleculeUnknownReferenceSkipped(t *testing.T) {
	mols := append(DefaultMolecules(),
		domain.Molecule{Name: "typo", Formula: "liablities * 2"},
		domain.Molecule{Name: "ok_one", Formula: "liabilities * 2"},
	)
	v := Vars(reliabilityData(mols))
	if _, ok := v["typo"]; ok {
		t.Errorf("typo molecule should stay unset, got %v", v["typo"])
	}
	if _, ok := v["ok_one"]; !ok {
		t.Error("healthy molecule must still evaluate")
	}
}
