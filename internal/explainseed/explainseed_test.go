// SPDX-License-Identifier: MIT

package explainseed

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/engineenv"
)

func TestLabel(t *testing.T) {
	if got := Label("net_worth"); got != "net worth" {
		t.Fatalf("Label = %q", got)
	}
}

func TestSeedTextMolecule(t *testing.T) {
	d := engineenv.Derivation{
		Name:    "net_worth",
		Kind:    "molecule",
		Formula: "assets - liabilities",
		Value:   42000,
		Inputs:  map[string]float64{"assets": 50000, "liabilities": 8000},
	}
	got := SeedText(d, nil)
	for _, want := range []string{"net worth", "assets - liabilities", "assets = 50000", "liabilities = 8000", "evaluate_formula"} {
		if !strings.Contains(got, want) {
			t.Fatalf("seed missing %q:\n%s", want, got)
		}
	}
}

func TestSeedTextAtomUsesFormatter(t *testing.T) {
	d := engineenv.Derivation{Name: "liquid_cash", Kind: "atom", Source: "Σ balances of cash accounts", Value: 1234}
	got := SeedText(d, func(v float64) string { return "$" + strings.TrimSpace(strings.Trim("12.34", " ")) })
	if !strings.Contains(got, "$12.34") {
		t.Fatalf("formatter not applied: %s", got)
	}
	if !strings.Contains(got, "Σ balances of cash accounts") {
		t.Fatalf("atom source missing: %s", got)
	}
}

func TestSeedTextCustom(t *testing.T) {
	d := engineenv.Derivation{Name: "cf_txn_tip", Kind: "custom", Source: "custom field sum", Value: 5}
	got := SeedText(d, nil)
	if !strings.Contains(got, "custom field sum") {
		t.Fatalf("custom source missing: %s", got)
	}
}
