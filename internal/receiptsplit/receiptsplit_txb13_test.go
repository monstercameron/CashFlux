// SPDX-License-Identifier: MIT

package receiptsplit

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestProposeUsesEachRowsResolvedCategory(t *testing.T) {
	items := []LineItem{
		{Name: "Item", CategoryID: "groceries", Amount: money.New(500, "USD")},
		{Name: "Item", CategoryID: "household", Amount: money.New(300, "USD")},
	}
	got, ok := Propose(items, Target{Amount: money.New(-800, "USD"), CategoryID: "parent"}, nil)
	if !ok {
		t.Fatal("pre-resolved duplicate-name rows should produce a proposal")
	}
	want := []domain.CategorySplit{
		{CategoryID: "groceries", Amount: money.New(-500, "USD")},
		{CategoryID: "household", Amount: money.New(-300, "USD")},
	}
	if len(got.Splits) != len(want) {
		t.Fatalf("splits = %+v, want %+v", got.Splits, want)
	}
	for i := range want {
		if got.Splits[i] != want[i] {
			t.Errorf("split[%d] = %+v, want %+v", i, got.Splits[i], want[i])
		}
	}
}
