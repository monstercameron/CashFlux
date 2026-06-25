// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// descExpense builds a non-transfer USD expense with a Desc field (no Payee),
// mirroring TopPayees' reliance on Desc for grouping.
func descExpense(desc string, major int64, on time.Time) domain.Transaction {
	return domain.Transaction{Desc: desc, CategoryID: "x", Amount: money.New(-major*100, "USD"), Date: on}
}

// TestPayeeTrendsTopNSelection verifies that the top-N payees are chosen by
// their total spend across all buckets and that the Spend series is correct.
func TestPayeeTrendsTopNSelection(t *testing.T) {
	// Two monthly buckets: June and July 2026.
	bounds := []time.Time{
		dt(2026, time.June, 1),
		dt(2026, time.July, 1),
		dt(2026, time.August, 1),
	}
	txns := []domain.Transaction{
		// Amazon: $200 June + $300 July = $500 total → rank 1
		descExpense("Amazon", 200, dt(2026, time.June, 5)),
		descExpense("Amazon", 300, dt(2026, time.July, 10)),
		// Starbucks: $150 June + $100 July = $250 total → rank 2
		descExpense("Starbucks", 150, dt(2026, time.June, 8)),
		descExpense("Starbucks", 100, dt(2026, time.July, 12)),
		// Netflix: $15 June only = $15 total → rank 3 (should be excluded by topN=2)
		descExpense("Netflix", 15, dt(2026, time.June, 20)),
	}
	got, err := PayeeTrends(txns, bounds, usdRates(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d trends, want 2 (Amazon, Starbucks): %+v", len(got), got)
	}
	// Amazon is rank 1.
	if got[0].Payee != "Amazon" {
		t.Errorf("trend[0].Payee = %q, want Amazon", got[0].Payee)
	}
	if !equalI64(got[0].Spend, []int64{20000, 30000}) {
		t.Errorf("Amazon.Spend = %v, want [20000 30000]", got[0].Spend)
	}
	// Starbucks is rank 2.
	if got[1].Payee != "Starbucks" {
		t.Errorf("trend[1].Payee = %q, want Starbucks", got[1].Payee)
	}
	if !equalI64(got[1].Spend, []int64{15000, 10000}) {
		t.Errorf("Starbucks.Spend = %v, want [15000 10000]", got[1].Spend)
	}
}

// TestPayeeTrendsPerBucketDistribution verifies that a payee spending only in
// some buckets has zeros in the buckets where it was silent.
func TestPayeeTrendsPerBucketDistribution(t *testing.T) {
	bounds := []time.Time{
		dt(2026, time.April, 1),
		dt(2026, time.May, 1),
		dt(2026, time.June, 1),
		dt(2026, time.July, 1),
	}
	txns := []domain.Transaction{
		// Rent: only April and June.
		descExpense("Rent", 900, dt(2026, time.April, 1)),
		descExpense("Rent", 900, dt(2026, time.June, 1)),
		// Gym: only May.
		descExpense("Gym", 50, dt(2026, time.May, 15)),
	}
	got, err := PayeeTrends(txns, bounds, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d trends, want 2 (Rent, Gym): %+v", len(got), got)
	}
	// Rent is rank 1 ($1800 total).
	rentIdx := -1
	gymIdx := -1
	for i, tr := range got {
		switch tr.Payee {
		case "Rent":
			rentIdx = i
		case "Gym":
			gymIdx = i
		}
	}
	if rentIdx == -1 || gymIdx == -1 {
		t.Fatalf("missing payee in results: %+v", got)
	}
	if !equalI64(got[rentIdx].Spend, []int64{90000, 0, 90000}) {
		t.Errorf("Rent.Spend = %v, want [90000 0 90000]", got[rentIdx].Spend)
	}
	if !equalI64(got[gymIdx].Spend, []int64{0, 5000, 0}) {
		t.Errorf("Gym.Spend = %v, want [0 5000 0]", got[gymIdx].Spend)
	}
}

// TestPayeeTrendsNormalizationAndDedup verifies that payees are grouped
// case-insensitively with the first spelling kept as the display name.
func TestPayeeTrendsNormalizationAndDedup(t *testing.T) {
	bounds := []time.Time{
		dt(2026, time.June, 1),
		dt(2026, time.July, 1),
		dt(2026, time.August, 1),
	}
	txns := []domain.Transaction{
		// Three spellings of the same payee — all should merge.
		descExpense("Starbucks", 5, dt(2026, time.June, 2)),      // first spelling
		descExpense("STARBUCKS", 7, dt(2026, time.June, 9)),      // same key, different case
		descExpense("starbucks", 10, dt(2026, time.July, 3)),     // another bucket, same key
		descExpense("  Starbucks  ", 3, dt(2026, time.July, 20)), // leading/trailing whitespace
	}
	got, err := PayeeTrends(txns, bounds, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d trends, want 1 (all Starbucks variants merged): %+v", len(got), got)
	}
	// Display name is the first spelling seen (chronological order of bucket
	// iteration: June bucket comes first; within the bucket, insertion order).
	if got[0].Payee != "Starbucks" {
		t.Errorf("Payee = %q, want Starbucks (first spelling)", got[0].Payee)
	}
	// June: $5 + $7 = $12 → 1200; July: $10 + $3 = $13 → 1300.
	if !equalI64(got[0].Spend, []int64{1200, 1300}) {
		t.Errorf("Spend = %v, want [1200 1300]", got[0].Spend)
	}
}

// TestPayeeTrendsDescFallback verifies that Desc is used (not Payee) for
// grouping, matching TopPayees behavior. A transaction with a Payee field but
// an empty Desc is grouped under the "" key.
func TestPayeeTrendsDescFallback(t *testing.T) {
	bounds := []time.Time{
		dt(2026, time.June, 1),
		dt(2026, time.July, 1),
	}
	txns := []domain.Transaction{
		// Desc is non-empty — used as the key.
		{Desc: "Whole Foods", Payee: "WFM", CategoryID: "x", Amount: money.New(-5000, "USD"), Date: dt(2026, time.June, 5)},
		// Desc is empty — grouped under the "" key (like TopPayees "(no description)" behavior).
		{Desc: "", Payee: "Mystery", CategoryID: "x", Amount: money.New(-2000, "USD"), Date: dt(2026, time.June, 10)},
	}
	got, err := PayeeTrends(txns, bounds, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d trends, want 2: %+v", len(got), got)
	}
	byPayee := map[string]*PayeeTrend{}
	for i := range got {
		byPayee[got[i].Payee] = &got[i]
	}
	if wf := byPayee["Whole Foods"]; wf == nil || wf.Spend[0] != 5000 {
		t.Errorf("Whole Foods not found or wrong amount: %+v", byPayee)
	}
	// Empty-desc entry is grouped under "".
	if em := byPayee[""]; em == nil || em.Spend[0] != 2000 {
		t.Errorf("empty-desc entry not found or wrong amount: %+v", byPayee)
	}
}

// TestPayeeTrendsExcludesIncomeAndTransfers verifies that income and transfer
// transactions are excluded just as in TopPayees / CategoryTrends.
func TestPayeeTrendsExcludesIncomeAndTransfers(t *testing.T) {
	bounds := []time.Time{
		dt(2026, time.June, 1),
		dt(2026, time.July, 1),
	}
	txns := []domain.Transaction{
		descExpense("Rent", 900, dt(2026, time.June, 1)),
		// Income — excluded.
		{Desc: "Paycheck", Amount: money.New(300000, "USD"), Date: dt(2026, time.June, 15)},
		// Transfer — excluded.
		{Desc: "Savings move", Amount: money.New(-10000, "USD"), TransferAccountID: "acc2", Date: dt(2026, time.June, 20)},
		// Out-of-range expense — excluded.
		descExpense("Rent", 900, dt(2026, time.May, 31)),
	}
	got, err := PayeeTrends(txns, bounds, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Payee != "Rent" {
		t.Errorf("got %+v, want exactly [Rent]", got)
	}
	if !equalI64(got[0].Spend, []int64{90000}) {
		t.Errorf("Rent.Spend = %v, want [90000]", got[0].Spend)
	}
}

// TestPayeeTrendsEmptyInput verifies that an empty transaction slice yields nil,
// consistent with CategoryTrends behavior on empty input.
func TestPayeeTrendsEmptyInput(t *testing.T) {
	bounds := []time.Time{dt(2026, time.June, 1), dt(2026, time.July, 1)}
	got, err := PayeeTrends(nil, bounds, usdRates(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("empty input: got %+v, want nil", got)
	}
}

// TestPayeeTrendsTooFewBounds mirrors the CategoryTrends equivalent: fewer than
// two bounds means no complete bucket → return nil.
func TestPayeeTrendsTooFewBounds(t *testing.T) {
	txns := []domain.Transaction{descExpense("Rent", 100, dt(2026, time.June, 5))}
	got, err := PayeeTrends(txns, []time.Time{dt(2026, time.June, 1)}, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("single-bound: got %+v, want nil", got)
	}
}

// TestPayeeTrendsTotalTopNAllPayees verifies that topN=0 returns all payees.
func TestPayeeTrendsTotalTopNAllPayees(t *testing.T) {
	bounds := []time.Time{dt(2026, time.June, 1), dt(2026, time.July, 1)}
	txns := []domain.Transaction{
		descExpense("A", 30, dt(2026, time.June, 1)),
		descExpense("B", 20, dt(2026, time.June, 2)),
		descExpense("C", 10, dt(2026, time.June, 3)),
	}
	got, err := PayeeTrends(txns, bounds, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("topN=0: got %d trends, want 3", len(got))
	}
}

// TestPayeeTrendsDeterministicTieBreak verifies that two payees with identical
// total spend are ordered by their lowercase key for determinism.
func TestPayeeTrendsDeterministicTieBreak(t *testing.T) {
	bounds := []time.Time{dt(2026, time.June, 1), dt(2026, time.July, 1)}
	txns := []domain.Transaction{
		descExpense("Zebra", 100, dt(2026, time.June, 1)),
		descExpense("Alpha", 100, dt(2026, time.June, 2)),
	}
	got, err := PayeeTrends(txns, bounds, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d trends, want 2", len(got))
	}
	// "alpha" < "zebra" lexicographically.
	if got[0].Payee != "Alpha" || got[1].Payee != "Zebra" {
		t.Errorf("tie-break order = %q, %q; want Alpha, Zebra", got[0].Payee, got[1].Payee)
	}
}
