// SPDX-License-Identifier: MIT

package reviewqueue

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestGroupTotalsKeepCurrenciesSeparate(t *testing.T) {
	g := Group{Items: []domain.Transaction{
		{Amount: money.New(-2000, "USD")},
		{Amount: money.New(-2000, "EUR")},
		{Amount: money.New(-500, "USD")},
	}}
	want := map[string]int64{"USD": -2500, "EUR": -2000}
	if got := g.TotalsByCurrency(); !reflect.DeepEqual(got, want) {
		t.Fatalf("TotalsByCurrency() = %#v, want %#v", got, want)
	}
}

func TestUnbalancedSplitCannotBeResolvedByCategoryAssignment(t *testing.T) {
	tx := domain.Transaction{Amount: money.New(-1000, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-400, "USD")},
	}}
	if CanAssignCategory(tx) {
		t.Fatal("CanAssignCategory returned true for an unbalanced split")
	}
	flat := domain.Transaction{Amount: money.New(-1000, "USD")}
	if !CanAssignCategory(flat) {
		t.Fatal("CanAssignCategory returned false for an ordinary uncategorized charge")
	}
}
