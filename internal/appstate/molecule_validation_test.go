// SPDX-License-Identifier: MIT

package appstate

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestPutMoleculeRejectsSelfReference: a molecule may not reference itself —
// it would never evaluate and every reader would see a silent gap.
func TestPutMoleculeRejectsSelfReference(t *testing.T) {
	a := newApp(t, false)
	err := a.PutMolecule(domain.Molecule{Name: "loop_me", Formula: "loop_me + 1"})
	if err == nil {
		t.Fatal("self-referencing molecule should be rejected")
	}
}

// TestPutMoleculeRejectsCycles: direct and indirect reference cycles are
// caught at save time, with the cycle path in the message.
func TestPutMoleculeRejectsCycles(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutMolecule(domain.Molecule{Name: "cyc_a", Formula: "income + 1"}); err != nil {
		t.Fatalf("cyc_a: %v", err)
	}
	if err := a.PutMolecule(domain.Molecule{Name: "cyc_b", Formula: "cyc_a * 2"}); err != nil {
		t.Fatalf("cyc_b: %v", err)
	}
	// Re-pointing cyc_a at cyc_b closes a → b → a.
	err := a.PutMolecule(domain.Molecule{Name: "cyc_a", Formula: "cyc_b - 1"})
	if err == nil {
		t.Fatal("indirect cycle should be rejected")
	}
	if !strings.Contains(err.Error(), "cyc_") {
		t.Errorf("cycle error should name the path, got: %v", err)
	}
	// A three-link chain without a cycle stays legal.
	if err := a.PutMolecule(domain.Molecule{Name: "cyc_c", Formula: "cyc_b + cyc_a"}); err != nil {
		t.Fatalf("acyclic chain rejected: %v", err)
	}
}

// TestPutMoleculeRejectsReservedNames: atoms and other built-in non-molecule
// variables can't be taken as molecule names; default molecules stay
// overridable.
func TestPutMoleculeRejectsReservedNames(t *testing.T) {
	a := newApp(t, false)
	for _, name := range []string{"assets", "income", "liquid_cash", "health_savings"} {
		if err := a.PutMolecule(domain.Molecule{Name: name, Formula: "1 + 1"}); err == nil {
			t.Errorf("molecule named %q (a built-in variable) should be rejected", name)
		}
	}
	// Overriding a DEFAULT molecule remains allowed (the Studio contract).
	if err := a.PutMolecule(domain.Molecule{Name: "net_worth", Formula: "assets - liabilities - 100"}); err != nil {
		t.Errorf("overriding a default molecule should stay allowed: %v", err)
	}
}

// TestDeleteMoleculeBlocksWhenReferenced: deleting a custom molecule another
// formula depends on errors and names the dependent; once the dependent is
// gone the delete succeeds. Deleting an overridden built-in (which restores
// the default) is never blocked.
func TestDeleteMoleculeBlocksWhenReferenced(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutMolecule(domain.Molecule{Name: "base_fig", Formula: "income - expense"}); err != nil {
		t.Fatalf("base_fig: %v", err)
	}
	if err := a.PutMolecule(domain.Molecule{Name: "dep_fig", Formula: "base_fig * 100"}); err != nil {
		t.Fatalf("dep_fig: %v", err)
	}
	if _, err := a.DeleteMolecule("base_fig"); err == nil {
		t.Fatal("deleting a molecule with dependents should error")
	} else if !strings.Contains(err.Error(), "dep_fig") {
		t.Errorf("error should name the dependent, got: %v", err)
	}
	if _, err := a.DeleteMolecule("dep_fig"); err != nil {
		t.Fatalf("deleting the dependent first should work: %v", err)
	}
	if _, err := a.DeleteMolecule("base_fig"); err != nil {
		t.Fatalf("delete after dependents gone should work: %v", err)
	}
	// Built-in override delete = restore, never blocked: net_worth is referenced
	// by nothing here, but health_score-style overrides always restore.
	if err := a.PutMolecule(domain.Molecule{Name: "net_worth", Formula: "assets"}); err != nil {
		t.Fatalf("override net_worth: %v", err)
	}
	if _, err := a.DeleteMolecule("net_worth"); err != nil {
		t.Fatalf("deleting a built-in override must restore, not block: %v", err)
	}
}
