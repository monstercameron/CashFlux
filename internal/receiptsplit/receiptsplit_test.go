// SPDX-License-Identifier: MIT

package receiptsplit

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// usd builds a positive USD line-item amount from a minor value.
func usd(minor int64) money.Money { return money.New(minor, "USD") }

// keywordMatch is a tiny MatchFunc for tests: exact name -> category id.
func keywordMatch(m map[string]string) MatchFunc {
	return func(name string) string { return m[name] }
}

func TestPropose(t *testing.T) {
	tests := []struct {
		name       string
		items      []LineItem
		target     Target
		match      MatchFunc
		wantOK     bool
		wantSplits []domain.CategorySplit
		wantNote   bool // whether a caution note is expected
		wantRem    int64
	}{
		{
			name: "exact match no remainder",
			items: []LineItem{
				{Name: "Milk", Amount: usd(400)},
				{Name: "Screwdriver", Amount: usd(600)},
			},
			target: Target{Amount: money.New(-1000, "USD"), CategoryID: "cat-shopping"},
			match:  keywordMatch(map[string]string{"Milk": "cat-grocery", "Screwdriver": "cat-tools"}),
			wantOK: true,
			wantSplits: []domain.CategorySplit{
				{CategoryID: "cat-grocery", Amount: money.New(-400, "USD")},
				{CategoryID: "cat-tools", Amount: money.New(-600, "USD")},
			},
			wantNote: false,
			wantRem:  0,
		},
		{
			name: "remainder line for tax on parent category",
			items: []LineItem{
				{Name: "Milk", Amount: usd(400)},
				{Name: "Bread", Amount: usd(500)},
			},
			// txn total is 1000, lines are 900 -> 100 tax lands on the parent category.
			target: Target{Amount: money.New(-1000, "USD"), CategoryID: "cat-grocery"},
			match:  keywordMatch(map[string]string{"Milk": "cat-dairy", "Bread": "cat-bakery"}),
			wantOK: true,
			wantSplits: []domain.CategorySplit{
				{CategoryID: "cat-dairy", Amount: money.New(-400, "USD")},
				{CategoryID: "cat-bakery", Amount: money.New(-500, "USD")},
				{CategoryID: "cat-grocery", Amount: money.New(-100, "USD")},
			},
			wantNote: true, // lines total less than txn
			wantRem:  -100,
		},
		{
			name: "unmatched lines fold into remainder but one match still proposes",
			items: []LineItem{
				{Name: "Milk", Amount: usd(400)},
				{Name: "Mystery", Amount: usd(600)},
			},
			target: Target{Amount: money.New(-1000, "USD"), CategoryID: "cat-grocery"},
			match:  keywordMatch(map[string]string{"Milk": "cat-dairy"}),
			wantOK: true,
			wantSplits: []domain.CategorySplit{
				{CategoryID: "cat-dairy", Amount: money.New(-400, "USD")},
				{CategoryID: "cat-grocery", Amount: money.New(-600, "USD")},
			},
			wantNote: true,
			wantRem:  -600,
		},
		{
			name:   "single unmatched line proposes nothing",
			items:  []LineItem{{Name: "Mystery", Amount: usd(1000)}},
			target: Target{Amount: money.New(-1000, "USD"), CategoryID: "cat-grocery"},
			match:  keywordMatch(map[string]string{}),
			wantOK: false,
		},
		{
			name: "all unmatched proposes nothing",
			items: []LineItem{
				{Name: "A", Amount: usd(400)},
				{Name: "B", Amount: usd(600)},
			},
			target: Target{Amount: money.New(-1000, "USD"), CategoryID: "cat-grocery"},
			match:  keywordMatch(map[string]string{}),
			wantOK: false,
		},
		{
			name: "currency mismatch proposes nothing",
			items: []LineItem{
				{Name: "Milk", Amount: money.New(400, "EUR")},
				{Name: "Bread", Amount: usd(600)},
			},
			target: Target{Amount: money.New(-1000, "USD"), CategoryID: "cat-grocery"},
			match:  keywordMatch(map[string]string{"Milk": "cat-dairy", "Bread": "cat-bakery"}),
			wantOK: false,
		},
		{
			name: "grouping sums lines sharing a category",
			items: []LineItem{
				{Name: "Milk", Amount: usd(300)},
				{Name: "Cheese", Amount: usd(200)},
				{Name: "Nails", Amount: usd(500)},
			},
			target: Target{Amount: money.New(-1000, "USD"), CategoryID: "cat-shopping"},
			match:  keywordMatch(map[string]string{"Milk": "cat-dairy", "Cheese": "cat-dairy", "Nails": "cat-tools"}),
			wantOK: true,
			wantSplits: []domain.CategorySplit{
				{CategoryID: "cat-dairy", Amount: money.New(-500, "USD")},
				{CategoryID: "cat-tools", Amount: money.New(-500, "USD")},
			},
			wantNote: false,
			wantRem:  0,
		},
		{
			name: "lines overshoot txn puts negative-magnitude remainder on parent",
			items: []LineItem{
				{Name: "Milk", Amount: usd(700)},
				{Name: "Nails", Amount: usd(500)},
			},
			// txn is 1000, lines are 1200 -> remainder is -200 magnitude on parent.
			target: Target{Amount: money.New(-1000, "USD"), CategoryID: "cat-shopping"},
			match:  keywordMatch(map[string]string{"Milk": "cat-dairy", "Nails": "cat-tools"}),
			wantOK: true,
			wantSplits: []domain.CategorySplit{
				{CategoryID: "cat-dairy", Amount: money.New(-700, "USD")},
				{CategoryID: "cat-tools", Amount: money.New(-500, "USD")},
				{CategoryID: "cat-shopping", Amount: money.New(200, "USD")},
			},
			wantNote: true,
			wantRem:  200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Propose(tt.items, tt.target, tt.match)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if len(got.Splits) != len(tt.wantSplits) {
				t.Fatalf("splits = %+v, want %+v", got.Splits, tt.wantSplits)
			}
			for i, s := range got.Splits {
				w := tt.wantSplits[i]
				if s.CategoryID != w.CategoryID || s.Amount.Amount != w.Amount.Amount || s.Amount.Currency != w.Amount.Currency {
					t.Errorf("split[%d] = %+v, want %+v", i, s, w)
				}
			}
			// The split invariant: Σ(splits) == txn amount exactly.
			if !domain.SplitsReconcile(tt.target.Amount, got.Splits) {
				t.Errorf("splits do not reconcile to %v: total %v", tt.target.Amount, domain.SplitsTotal(got.Splits))
			}
			if got.Remainder != tt.wantRem {
				t.Errorf("remainder = %d, want %d", got.Remainder, tt.wantRem)
			}
			if (got.Note != "") != tt.wantNote {
				t.Errorf("note = %q, wantNote %v", got.Note, tt.wantNote)
			}
		})
	}
}

func TestProposeEmptyAndNilMatch(t *testing.T) {
	if _, ok := Propose(nil, Target{Amount: money.New(-100, "USD")}, keywordMatch(nil)); ok {
		t.Error("empty items should not propose")
	}
	if _, ok := Propose([]LineItem{{Name: "X", Amount: usd(100)}}, Target{Amount: money.New(-100, "USD")}, nil); ok {
		t.Error("nil match should not propose")
	}
}
