// SPDX-License-Identifier: MIT

package domain

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func split(cat string, cents int64) CategorySplit {
	return CategorySplit{CategoryID: cat, Amount: money.New(cents, "USD")}
}

func TestSplitsTotal(t *testing.T) {
	got := SplitsTotal([]CategorySplit{split("produce", 700), split("dairy", 500), split("household", 300)})
	if got.Amount != 1500 {
		t.Errorf("SplitsTotal = %d, want 1500", got.Amount)
	}
	if SplitsTotal(nil).Amount != 0 {
		t.Errorf("empty splits should total zero")
	}
}

func TestSplitsReconcile(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		splits []CategorySplit
		want   bool
	}{
		{
			name:   "splits sum to the amount",
			amount: 1500,
			splits: []CategorySplit{split("produce", 700), split("dairy", 500), split("household", 300)},
			want:   true,
		},
		{
			name:   "splits with a discount line still reconcile",
			amount: 800,
			splits: []CategorySplit{split("groceries", 1000), split("coupon", -200)},
			want:   true,
		},
		{
			name:   "splits that fall short do not reconcile",
			amount: 1500,
			splits: []CategorySplit{split("produce", 700), split("dairy", 500)},
			want:   false,
		},
		{
			name:   "no splits reconciles trivially",
			amount: 1500,
			splits: nil,
			want:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SplitsReconcile(money.New(tc.amount, "USD"), tc.splits); got != tc.want {
				t.Errorf("SplitsReconcile = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTransactionSplitHelpers(t *testing.T) {
	plain := Transaction{Amount: money.New(1500, "USD")}
	if plain.HasSplits() {
		t.Error("a transaction with no splits should report HasSplits=false")
	}
	if !plain.SplitsReconcile() {
		t.Error("an unsplit transaction reconciles trivially")
	}
	receipt := Transaction{Amount: money.New(1500, "USD"), Splits: []CategorySplit{split("produce", 1000), split("dairy", 500)}}
	if !receipt.HasSplits() || !receipt.SplitsReconcile() {
		t.Errorf("a balanced receipt should have splits and reconcile: %+v", receipt)
	}
}
