// SPDX-License-Identifier: MIT

package splitsuggest

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// txn builds a precedent transaction with the given split lines (category ->
// minor units, all in USD).
func txn(lines ...domain.CategorySplit) domain.Transaction {
	total := int64(0)
	for _, l := range lines {
		total += l.Amount.Amount
	}
	return domain.Transaction{Amount: money.New(total, "USD"), Splits: lines}
}

// line is a terse split-line constructor for the tables below.
func line(cat string, minor int64) domain.CategorySplit {
	return domain.CategorySplit{CategoryID: cat, Amount: money.New(minor, "USD")}
}

func TestSuggest(t *testing.T) {
	tests := []struct {
		name       string
		in         Input
		wantOK     bool
		wantLines  []Line
		wantPreced int
	}{
		{
			name: "two even precedents propose the averaged shares",
			in: Input{
				AmountMinor: -10000,
				History: []domain.Transaction{
					txn(line("groceries", -7500), line("household", -2500)),
					txn(line("groceries", -7500), line("household", -2500)),
				},
			},
			wantOK:     true,
			wantPreced: 2,
			wantLines: []Line{
				{CategoryID: "groceries", AmountMinor: -7500, ShareBP: 7500},
				{CategoryID: "household", AmountMinor: -2500, ShareBP: 2500},
			},
		},
		{
			name: "shares average per precedent, not pooled by money",
			in: Input{
				AmountMinor: -10000,
				History: []domain.Transaction{
					// A huge run that is nearly all groceries...
					txn(line("groceries", -90000), line("household", -10000)),
					// ...and a small one that is half household. Pooling the money
					// would give household 10%; averaging the two precedents gives 30%.
					txn(line("groceries", -500), line("household", -500)),
				},
			},
			wantOK:     true,
			wantPreced: 2,
			wantLines: []Line{
				{CategoryID: "groceries", AmountMinor: -7000, ShareBP: 7000},
				{CategoryID: "household", AmountMinor: -3000, ShareBP: 3000},
			},
		},
		{
			name: "one precedent is an anecdote, not a pattern",
			in: Input{
				AmountMinor: -10000,
				History:     []domain.Transaction{txn(line("groceries", -7500), line("household", -2500))},
			},
			wantOK: false,
		},
		{
			name: "a dominant category is a categorization, not a split",
			in: Input{
				AmountMinor: -10000,
				History: []domain.Transaction{
					txn(line("groceries", -9900), line("household", -100)),
					txn(line("groceries", -9900), line("household", -100)),
				},
			},
			wantOK: false,
		},
		{
			name: "single-line precedents are not splits",
			in: Input{
				AmountMinor: -10000,
				History: []domain.Transaction{
					txn(line("groceries", -10000)),
					txn(line("groceries", -10000)),
				},
			},
			wantOK: false,
		},
		{
			name:   "a zero charge has nothing to divide",
			in:     Input{AmountMinor: 0, History: []domain.Transaction{txn(line("a", -50), line("b", -50)), txn(line("a", -50), line("b", -50))}},
			wantOK: false,
		},
		{
			name: "unsplit history is ignored, not counted",
			in: Input{
				AmountMinor: -10000,
				History: []domain.Transaction{
					{Amount: money.New(-4000, "USD"), CategoryID: "groceries"},
					txn(line("groceries", -5000), line("household", -5000)),
				},
			},
			wantOK: false, // only one real precedent survives the filter
		},
		{
			name: "a positive charge keeps its sign",
			in: Input{
				AmountMinor: 10000,
				History: []domain.Transaction{
					txn(line("salary", 8000), line("bonus", 2000)),
					txn(line("salary", 8000), line("bonus", 2000)),
				},
			},
			wantOK:     true,
			wantPreced: 2,
			wantLines: []Line{
				{CategoryID: "salary", AmountMinor: 8000, ShareBP: 8000},
				{CategoryID: "bonus", AmountMinor: 2000, ShareBP: 2000},
			},
		},
		{
			name: "a category named twice in one breakdown folds into one line",
			in: Input{
				AmountMinor: -10000,
				History: []domain.Transaction{
					txn(line("groceries", -2500), line("groceries", -2500), line("household", -5000)),
					txn(line("groceries", -5000), line("household", -5000)),
				},
			},
			wantOK:     true,
			wantPreced: 2,
			wantLines: []Line{
				{CategoryID: "groceries", AmountMinor: -5000, ShareBP: 5000},
				{CategoryID: "household", AmountMinor: -5000, ShareBP: 5000},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Suggest(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got %+v)", ok, tc.wantOK, got)
			}
			if !tc.wantOK {
				return
			}
			if got.Precedents != tc.wantPreced {
				t.Errorf("Precedents = %d, want %d", got.Precedents, tc.wantPreced)
			}
			if len(got.Lines) != len(tc.wantLines) {
				t.Fatalf("lines = %+v, want %+v", got.Lines, tc.wantLines)
			}
			for i, w := range tc.wantLines {
				if got.Lines[i] != w {
					t.Errorf("line %d = %+v, want %+v", i, got.Lines[i], w)
				}
			}
		})
	}
}

// TestSuggest_LinesSumToCharge is the money invariant: a proposal a user applies
// must never need a reconciling line, at any amount or share shape.
func TestSuggest_LinesSumToCharge(t *testing.T) {
	amounts := []int64{-1, -7, -99, -100, -333, -10000, -123457, 1, 999, 100003}
	histories := [][]domain.Transaction{
		{txn(line("a", -1), line("b", -1), line("c", -1)), txn(line("a", -1), line("b", -1), line("c", -1))},
		{txn(line("a", -6667), line("b", -3333)), txn(line("a", -6667), line("b", -3333))},
		{txn(line("a", -100), line("b", -200), line("c", -300), line("d", -400)),
			txn(line("a", -400), line("b", -300), line("c", -200), line("d", -100))},
	}
	for _, amt := range amounts {
		for hi, h := range histories {
			got, ok := Suggest(Input{AmountMinor: amt, History: h})
			if !ok {
				continue
			}
			sum := int64(0)
			for _, l := range got.Lines {
				sum += l.AmountMinor
			}
			if sum != amt {
				t.Errorf("amount %d history %d: lines sum to %d, want %d (%+v)", amt, hi, sum, amt, got.Lines)
			}
		}
	}
}

func TestDistribute(t *testing.T) {
	tests := []struct {
		name    string
		total   int64
		weights []int64
		want    []int64
	}{
		{name: "even thirds hand the remainder to the largest fractions", total: 100, weights: []int64{1, 1, 1}, want: []int64{34, 33, 33}},
		{name: "exact division needs no remainder", total: 100, weights: []int64{1, 1, 1, 1}, want: []int64{25, 25, 25, 25}},
		{name: "weighted split", total: 1000, weights: []int64{3, 1}, want: []int64{750, 250}},
		{name: "negative total keeps its sign", total: -100, weights: []int64{1, 1, 1}, want: []int64{-34, -33, -33}},
		{name: "zero weights drop out", total: 100, weights: []int64{1, 0, 1}, want: []int64{50, 0, 50}},
		{name: "no weights", total: 100, weights: nil, want: nil},
		{name: "all-zero weights", total: 100, weights: []int64{0, 0}, want: nil},
		{name: "one unit across three", total: 1, weights: []int64{1, 1, 1}, want: []int64{1, 0, 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Distribute(tc.total, tc.weights)
			if len(got) != len(tc.want) {
				t.Fatalf("Distribute(%d, %v) = %v, want %v", tc.total, tc.weights, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("Distribute(%d, %v) = %v, want %v", tc.total, tc.weights, got, tc.want)
				}
			}
		})
	}
}

func TestAsSplits(t *testing.T) {
	s := Suggestion{Lines: []Line{{CategoryID: "a", AmountMinor: -750}, {CategoryID: "b", AmountMinor: -250}}}
	got := AsSplits(s, "EUR")
	if len(got) != 2 {
		t.Fatalf("got %d splits, want 2", len(got))
	}
	if got[0].CategoryID != "a" || got[0].Amount.Amount != -750 || got[0].Amount.Currency != "EUR" {
		t.Errorf("split 0 = %+v", got[0])
	}
	if got[1].CategoryID != "b" || got[1].Amount.Amount != -250 {
		t.Errorf("split 1 = %+v", got[1])
	}
	if AsSplits(Suggestion{}, "USD") != nil {
		t.Error("an empty suggestion should render no splits")
	}
}

func TestSuggest_MinPrecedentsOverride(t *testing.T) {
	in := Input{
		AmountMinor:   -10000,
		MinPrecedents: 1,
		History:       []domain.Transaction{txn(line("groceries", -7500), line("household", -2500))},
	}
	got, ok := Suggest(in)
	if !ok {
		t.Fatal("a caller that lowers the threshold should get the single precedent")
	}
	if got.Precedents != 1 || len(got.Lines) != 2 {
		t.Errorf("got %+v", got)
	}
}
