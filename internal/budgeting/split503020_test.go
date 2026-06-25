// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func expCat(id, name string) domain.Category {
	return domain.Category{ID: id, Name: name, Kind: domain.KindExpense}
}

func incCat(id, name string) domain.Category {
	return domain.Category{ID: id, Name: name, Kind: domain.KindIncome}
}

func txnFor(catID string, amountMinor int64, day string) domain.Transaction {
	return domain.Transaction{
		CategoryID: catID,
		Amount:     money.New(-amountMinor, "USD"), // negative = expense
		Date:       mustDate(day),
	}
}

// ── Classify ─────────────────────────────────────────────────────────────────

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		catName  string
		want     Bucket
	}{
		// Needs keywords
		{"rent",            "Rent",            BucketNeeds},
		{"housing",         "Housing",         BucketNeeds},
		{"groceries",       "Groceries",       BucketNeeds},
		{"health",          "Health & Medical", BucketNeeds},
		{"transport",       "Transport",       BucketNeeds},
		{"utilities",       "Utilities",       BucketNeeds},
		{"insurance",       "Car Insurance",   BucketNeeds},
		{"childcare",       "Childcare",       BucketNeeds},
		// Savings keywords
		{"savings",         "Savings",         BucketSavings},
		{"investment",      "Investment",      BucketSavings},
		{"emergency fund",  "Emergency Fund",  BucketSavings},
		{"retirement",      "Retirement",      BucketSavings},
		// Wants keywords
		{"dining",          "Dining Out",      BucketWants},
		{"entertainment",   "Entertainment",   BucketWants},
		{"shopping",        "Shopping",        BucketWants},
		{"subscription",    "Subscriptions",   BucketWants},
		{"personal",        "Personal Care",   BucketWants},
		// Default → Wants
		{"unknown",         "Miscellaneous",   BucketWants},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat := expCat("id-"+tt.name, tt.catName)
			got := Classify(cat, nil)
			if got != tt.want {
				t.Errorf("Classify(%q) = %s, want %s", tt.catName, got, tt.want)
			}
		})
	}
}

func TestClassifyOverride(t *testing.T) {
	cat := expCat("cat-food", "Groceries") // would normally be Needs
	overrides := map[string]Bucket{"cat-food": BucketWants}
	got := Classify(cat, overrides)
	if got != BucketWants {
		t.Errorf("override should force Wants, got %s", got)
	}
}

func TestClassifyOverrideSavings(t *testing.T) {
	cat := expCat("cat-misc", "Miscellaneous") // default Wants
	overrides := map[string]Bucket{"cat-misc": BucketSavings}
	if got := Classify(cat, overrides); got != BucketSavings {
		t.Errorf("override to Savings failed, got %s", got)
	}
}

// ── BucketString ─────────────────────────────────────────────────────────────

func TestBucketString(t *testing.T) {
	if BucketNeeds.String() != "Needs" {
		t.Errorf("BucketNeeds.String() = %q", BucketNeeds.String())
	}
	if BucketWants.String() != "Wants" {
		t.Errorf("BucketWants.String() = %q", BucketWants.String())
	}
	if BucketSavings.String() != "Savings" {
		t.Errorf("BucketSavings.String() = %q", BucketSavings.String())
	}
}

// ── Generate5030: split sums exactly to income ───────────────────────────────

func TestGenerate5030SumExact(t *testing.T) {
	tests := []struct {
		name   string
		income int64
	}{
		{"round hundreds",   500000},  // 5000.00
		{"odd remainder",    100001},  // forces non-zero remainder
		{"odd remainder 2",  333333},
		{"minimal income",   1},
		{"large income",     10_000_000_00}, // 10 million dollars
	}

	cats := []domain.Category{
		expCat("rent",    "Rent"),
		expCat("food",    "Groceries"),
		expCat("savings", "Savings"),
		expCat("dining",  "Dining Out"),
	}
	now := mustDate("2026-06-25")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Generate5030(tt.income, cats, nil, now)
			if r.NeedsTarget+r.WantsTarget+r.SavingsTarget != tt.income {
				t.Errorf("income=%d: Needs(%d)+Wants(%d)+Savings(%d) = %d ≠ %d",
					tt.income,
					r.NeedsTarget, r.WantsTarget, r.SavingsTarget,
					r.NeedsTarget+r.WantsTarget+r.SavingsTarget,
					tt.income,
				)
			}
		})
	}
}

func TestGenerate5030Percentages(t *testing.T) {
	income := int64(600000) // 6000.00
	cats := []domain.Category{
		expCat("rent",    "Rent"),
		expCat("savings", "Savings"),
		expCat("dining",  "Dining Out"),
	}
	now := mustDate("2026-06-25")
	r := Generate5030(income, cats, nil, now)

	if r.NeedsTarget != 300000 { // 50%
		t.Errorf("NeedsTarget = %d, want 300000", r.NeedsTarget)
	}
	if r.WantsTarget != 180000 { // 30%
		t.Errorf("WantsTarget = %d, want 180000", r.WantsTarget)
	}
	if r.SavingsTarget != 120000 { // 20%
		t.Errorf("SavingsTarget = %d, want 120000", r.SavingsTarget)
	}
}

// ── Generate5030: income categories are excluded ──────────────────────────────

func TestGenerate5030SkipsIncomeCategories(t *testing.T) {
	cats := []domain.Category{
		expCat("rent",   "Rent"),
		incCat("salary", "Salary"),
	}
	now := mustDate("2026-06-25")
	r := Generate5030(100000, cats, nil, now)

	for _, p := range r.Proposals {
		if p.Category.ID == "salary" {
			t.Error("income category should not appear in Proposals")
		}
	}
}

// ── Generate5030: zero income returns empty result ────────────────────────────

func TestGenerate5030ZeroIncome(t *testing.T) {
	cats := []domain.Category{expCat("rent", "Rent")}
	now := mustDate("2026-06-25")

	for _, inc := range []int64{0, -1, -1000} {
		r := Generate5030(inc, cats, nil, now)
		if r.Income != 0 || r.NeedsTarget != 0 || r.WantsTarget != 0 || r.SavingsTarget != 0 {
			t.Errorf("income=%d: expected zero result, got %+v", inc, r)
		}
		if len(r.Proposals) != 0 {
			t.Errorf("income=%d: expected no proposals, got %d", inc, len(r.Proposals))
		}
	}
}

// ── Generate5030: proportional distribution by trailing spend ─────────────────

func TestGenerate5030ProportionalDistribution(t *testing.T) {
	// Two Wants categories: dining spent 3000, entertainment spent 1000.
	// Trailing spend window: 3 months before June = Mar/Apr/May.
	cats := []domain.Category{
		expCat("dining",  "Dining Out"),
		expCat("entert",  "Entertainment"),
	}
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	// Spend in the trailing window (March–May 2026).
	txns := []domain.Transaction{
		txnFor("dining",  3000, "2026-05-10"),
		txnFor("entert",  1000, "2026-04-20"),
		// Outside window — should be ignored.
		txnFor("dining",  9999, "2025-12-01"),
		txnFor("entert",  9999, "2026-06-05"), // current month, excluded
	}

	income := int64(100000) // 1000.00
	r := Generate5030(income, cats, txns, now)

	// Wants target = 30% of 100000 = 30000.
	// Weights: dining=3000, entert=1000; total=4000.
	// dining = 30000 * 3000 / 4000 = 22500; entert = 30000 * 1000 / 4000 = 7500.
	// No remainder (30000 * 3000 is divisible).
	var diningLimit, entertLimit int64
	for _, p := range r.Proposals {
		switch p.Category.ID {
		case "dining":
			diningLimit = p.LimitMinor
		case "entert":
			entertLimit = p.LimitMinor
		}
	}
	if diningLimit != 22500 {
		t.Errorf("dining limit = %d, want 22500", diningLimit)
	}
	if entertLimit != 7500 {
		t.Errorf("entertainment limit = %d, want 7500", entertLimit)
	}
}

// ── Generate5030: even split when no historical spend ────────────────────────

func TestGenerate5030EvenSplitNoSpend(t *testing.T) {
	// Three Needs categories, no transactions → even split.
	cats := []domain.Category{
		expCat("rent",   "Rent"),
		expCat("food",   "Groceries"),
		expCat("util",   "Utilities"),
	}
	now := mustDate("2026-06-25")
	income := int64(300000) // 3000.00
	r := Generate5030(income, cats, nil, now)

	// NeedsTarget = 50% of 300000 = 150000; 3 categories → 50000 each.
	// (150000 / 3 = 50000 with 0 remainder).
	var needsTotal int64
	for _, p := range r.Proposals {
		if p.Bucket == BucketNeeds {
			if p.LimitMinor != 50000 {
				t.Errorf("category %q: LimitMinor = %d, want 50000", p.Category.ID, p.LimitMinor)
			}
			needsTotal += p.LimitMinor
		}
	}
	if needsTotal != 150000 {
		t.Errorf("needs proposals total = %d, want 150000", needsTotal)
	}
}

// ── Generate5030: proposals sum to their bucket target ───────────────────────

func TestGenerate5030ProposalsSumToBucketTarget(t *testing.T) {
	cats := []domain.Category{
		expCat("rent",    "Rent"),
		expCat("food",    "Groceries"),
		expCat("util",    "Utilities"),
		expCat("dining",  "Dining Out"),
		expCat("entert",  "Entertainment"),
		expCat("savings", "Savings"),
		expCat("invest",  "Investment"),
	}
	txns := []domain.Transaction{
		txnFor("rent",   12000, "2026-05-01"),
		txnFor("food",    8000, "2026-05-15"),
		txnFor("dining",  4000, "2026-04-10"),
		txnFor("savings", 3000, "2026-03-20"),
	}
	income := int64(500000)
	now := mustDate("2026-06-25")
	r := Generate5030(income, cats, txns, now)

	bucketTotals := map[Bucket]int64{}
	for _, p := range r.Proposals {
		bucketTotals[p.Bucket] += p.LimitMinor
	}

	if bucketTotals[BucketNeeds] != r.NeedsTarget {
		t.Errorf("Needs proposals sum = %d, want NeedsTarget=%d", bucketTotals[BucketNeeds], r.NeedsTarget)
	}
	if bucketTotals[BucketWants] != r.WantsTarget {
		t.Errorf("Wants proposals sum = %d, want WantsTarget=%d", bucketTotals[BucketWants], r.WantsTarget)
	}
	if bucketTotals[BucketSavings] != r.SavingsTarget {
		t.Errorf("Savings proposals sum = %d, want SavingsTarget=%d", bucketTotals[BucketSavings], r.SavingsTarget)
	}
}

// ── Generate5030: empty category list ────────────────────────────────────────

func TestGenerate5030NoCats(t *testing.T) {
	now := mustDate("2026-06-25")
	r := Generate5030(200000, nil, nil, now)
	if r.Income != 200000 {
		t.Errorf("Income = %d, want 200000", r.Income)
	}
	if len(r.Proposals) != 0 {
		t.Errorf("expected no proposals, got %d", len(r.Proposals))
	}
	if r.NeedsTarget+r.WantsTarget+r.SavingsTarget != 200000 {
		t.Errorf("targets do not sum to income")
	}
}

// ── distribute helper ─────────────────────────────────────────────────────────

func TestDistributeEven(t *testing.T) {
	limits := distribute(90, []int64{0, 0, 0})
	var sum int64
	for _, l := range limits {
		sum += l
	}
	if sum != 90 {
		t.Errorf("even distribute(90,3 zeros) sum = %d, want 90", sum)
	}
}

func TestDistributeProportional(t *testing.T) {
	// weights 1:2:3 → shares 10, 20, 30 of 60
	limits := distribute(60, []int64{1, 2, 3})
	want := []int64{10, 20, 30}
	for i, l := range limits {
		if l != want[i] {
			t.Errorf("limits[%d] = %d, want %d", i, l, want[i])
		}
	}
}

func TestDistributeRemainderToFirst(t *testing.T) {
	// 10 / 3 = 3 remainder 1 → [4, 3, 3]
	limits := distribute(10, []int64{0, 0, 0})
	if limits[0] != 4 || limits[1] != 3 || limits[2] != 3 {
		t.Errorf("remainder distribution = %v, want [4 3 3]", limits)
	}
}

func TestDistributeZeroTarget(t *testing.T) {
	limits := distribute(0, []int64{100, 200})
	for _, l := range limits {
		if l != 0 {
			t.Errorf("distribute(0,...) should return all zeros, got %v", limits)
		}
	}
}
