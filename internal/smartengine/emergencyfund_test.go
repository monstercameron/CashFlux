// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// efInput builds an Input with a fixed recurring rent commitment and a few months
// of essential (non-flex) spend so EssentialBasis has something to derive from.
func efInput() Input {
	in := baseInput()
	in.Categories = []domain.Category{
		{ID: "rent", Name: "Rent", CategoryClass: domain.ClassFixed},
		{ID: "groc", Name: "Groceries", CategoryClass: domain.ClassFixed},
		{ID: "fun", Name: "Dining out", CategoryClass: domain.ClassFlex},
	}
	in.Recurring = []domain.Recurring{
		{ID: "r1", Label: "Rent", Amount: usd(-150000), Cadence: domain.CadenceMonthly, NextDue: ref},
	}
	// Essential groceries $400/mo for the 3 trailing months; discretionary dining
	// that must be excluded from the essential month.
	for k := 1; k <= 3; k++ {
		when := ref.AddDate(0, -k, 0)
		in.Transactions = append(in.Transactions,
			domain.Transaction{ID: "g" + itoa64(int64(k)), CategoryID: "groc", Date: when, Amount: usd(-40000), Desc: "food"},
			domain.Transaction{ID: "d" + itoa64(int64(k)), CategoryID: "fun", Date: when, Amount: usd(-20000), Desc: "dining"},
		)
	}
	return in
}

func TestEssentialBasis(t *testing.T) {
	b := efInput().EssentialBasis()
	if b.FixedMonthlyMinor != 150000 {
		t.Fatalf("fixed = %d, want 150000", b.FixedMonthlyMinor)
	}
	if b.EssentialSpendMonthlyMinor != 40000 {
		t.Fatalf("essential spend = %d, want 40000 (dining excluded)", b.EssentialSpendMonthlyMinor)
	}
	if got := b.EssentialMonthlyMinor(); got != 190000 {
		t.Fatalf("essential month = %d, want 190000", got)
	}
}

func TestG21EmergencyResize(t *testing.T) {
	in := efInput()
	// Essential month derives to $1,900. Give the emergency goal a stale basis of
	// $1,500 (drift > 10%) and a 3-month target sized off that stale basis.
	in.Goals = []domain.Goal{{
		ID:                  "ef",
		Name:                "Emergency Fund",
		TargetAmount:        usd(450000), // 3 × $1,500
		CurrentAmount:       usd(100000),
		EssentialBasisMinor: 150000,
	}}
	out := g21EmergencyResize(in)
	if len(out) != 1 {
		t.Fatalf("want 1 insight, got %d", len(out))
	}
	ins := out[0]
	if !strings.Contains(ins.Key, ":3") {
		t.Fatalf("key should encode level 3, got %q", ins.Key)
	}
	// New 3-month target off $1,900 essential = $5,700.
	if ins.Amount.Amount != 570000 {
		t.Fatalf("suggested target = %d, want 570000", ins.Amount.Amount)
	}
}

func TestG21QuietWhenNoBasis(t *testing.T) {
	in := efInput()
	in.Goals = []domain.Goal{{ID: "ef", Name: "Emergency Fund", TargetAmount: usd(450000)}}
	if out := g21EmergencyResize(in); len(out) != 0 {
		t.Fatalf("should stay quiet with no stored basis, got %d", len(out))
	}
}

func TestG21QuietWhenWithinDrift(t *testing.T) {
	in := efInput()
	in.Goals = []domain.Goal{{
		ID:                  "ef",
		Name:                "Emergency Fund",
		TargetAmount:        usd(570000),
		EssentialBasisMinor: 190000, // matches derived → no drift
	}}
	if out := g21EmergencyResize(in); len(out) != 0 {
		t.Fatalf("should stay quiet within drift, got %d", len(out))
	}
}
