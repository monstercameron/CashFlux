// SPDX-License-Identifier: MIT

package budgeting

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Bucket is the 50/30/20 spending category that a budget category belongs to.
type Bucket int

const (
	// BucketNeeds covers essential, non-optional expenses (housing, food, health,
	// transport, utilities, insurance).
	BucketNeeds Bucket = iota
	// BucketWants covers discretionary spending (dining out, entertainment,
	// shopping, subscriptions, personal).
	BucketWants
	// BucketSavings covers savings, investments, and emergency-fund contributions.
	BucketSavings
)

// String returns the human-readable name of the bucket.
func (b Bucket) String() string {
	switch b {
	case BucketNeeds:
		return "Needs"
	case BucketWants:
		return "Wants"
	case BucketSavings:
		return "Savings"
	default:
		return "Wants"
	}
}

// needsKeywords are lower-case substrings found in category names that map to
// BucketNeeds. The list is intentionally broad so typical out-of-the-box
// category sets classify correctly without manual overrides.
var needsKeywords = []string{
	"housing", "rent", "mortgage", "utilities", "utility", "electric",
	"gas", "water", "groceries", "grocery", "supermarket",
	"insurance", "health", "medical", "pharmacy", "doctor",
	"transport", "transit", "bus", "subway", "metro", "commute",
	"fuel", "petrol", "childcare", "education", "tuition",
}

// wantsKeywords are lower-case substrings found in category names that map to
// BucketWants (evaluated only when needsKeywords didn't match, so a category
// named "dining/groceries" goes to Needs first).
var wantsKeywords = []string{
	"dining", "restaurant", "cafe", "coffee", "takeout", "takeaway",
	"entertainment", "movie", "cinema", "concert", "gaming", "games",
	"shopping", "clothing", "fashion", "apparel", "accessories",
	"personal", "beauty", "haircut", "gym", "fitness",
	"subscription", "streaming", "netflix", "spotify", "hulu",
	"hobbies", "hobby", "travel", "vacation", "holiday",
	"bar", "pub", "alcohol", "beer", "wine",
}

// savingsKeywords are lower-case substrings found in category names that map
// to BucketSavings.
var savingsKeywords = []string{
	"savings", "saving", "investment", "invest", "retirement",
	"emergency", "pension", "401k", "ira", "brokerage",
	"reserve", "fund",
}

// Classify returns the 50/30/20 bucket for a category. When the category ID is
// present in overrides, that value wins unconditionally. Otherwise the category
// Name is matched (case-insensitively) against a built-in keyword table: Needs
// keywords are tried first, then Savings, then Wants. Income categories are
// treated as Wants (they are excluded from budget proposals by Generate5030).
// The default for an unmatched expense category is Wants.
func Classify(c domain.Category, overrides map[string]Bucket) Bucket {
	if b, ok := overrides[c.ID]; ok {
		return b
	}
	name := strings.ToLower(c.Name)
	for _, kw := range needsKeywords {
		if strings.Contains(name, kw) {
			return BucketNeeds
		}
	}
	for _, kw := range savingsKeywords {
		if strings.Contains(name, kw) {
			return BucketSavings
		}
	}
	for _, kw := range wantsKeywords {
		if strings.Contains(name, kw) {
			return BucketWants
		}
	}
	return BucketWants
}

// BudgetProposal is a recommended monthly spending limit for one category under
// the 50/30/20 framework.
type BudgetProposal struct {
	Category   domain.Category
	Bucket     Bucket
	LimitMinor int64 // recommended limit in minor currency units (e.g. cents)
}

// Split5030Result is the complete output of Generate5030.
type Split5030Result struct {
	// Income is the monthly income used as the base for the split (minor units).
	Income int64
	// NeedsTarget is 50% of Income (minor units).
	NeedsTarget int64
	// WantsTarget is 30% of Income (minor units).
	WantsTarget int64
	// SavingsTarget is the remainder after Needs and Wants so that the three
	// targets sum exactly to Income (minor units).
	SavingsTarget int64
	// Proposals is one entry per expense category, in the same order as the
	// input cats slice (income categories are omitted).
	Proposals []BudgetProposal
	// Unclassified holds categories that could not be placed into any bucket
	// (currently none under the built-in classifier, but overrides may
	// explicitly exclude a category by mapping it to a sentinel value outside
	// the three buckets — reserved for future use).
	Unclassified []domain.Category
}

// trailingSpend returns the total absolute expense amount (minor units) posted
// against catID in txns, capped to the 3 full calendar months ending before
// the month that contains now. Transfers and income transactions are excluded.
// Integer math only; no FX conversion (all amounts summed as-is).
func trailingSpend(catID string, txns []domain.Transaction, now time.Time) int64 {
	// Window: the 3 complete calendar months before the current month.
	curMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	windowEnd := curMonthStart
	windowStart := time.Date(now.Year(), now.Month()-3, 1, 0, 0, 0, 0, now.Location())

	var total int64
	for _, t := range txns {
		if !t.IsExpense() || t.CategoryID != catID {
			continue
		}
		if t.Date.Before(windowStart) || !t.Date.Before(windowEnd) {
			continue
		}
		// Amount is negative for expenses; take absolute value.
		amt := t.Amount.Amount
		if amt < 0 {
			amt = -amt
		}
		total += amt
	}
	return total
}

// distribute spreads target proportionally across cats according to weight[i].
// If all weights are zero, the target is split evenly. Integer rounding
// distributes any remainder to the first category that has a non-zero weight
// (or to the first category when weights are all zero).
//
// Returns one limit per category, in the same order.
func distribute(target int64, weights []int64) []int64 {
	n := len(weights)
	if n == 0 || target <= 0 {
		limits := make([]int64, n)
		return limits
	}

	var totalWeight int64
	for _, w := range weights {
		totalWeight += w
	}

	limits := make([]int64, n)
	if totalWeight == 0 {
		// Even split.
		base := target / int64(n)
		remainder := target - base*int64(n)
		for i := range limits {
			limits[i] = base
		}
		limits[0] += remainder
		return limits
	}

	// Proportional split.
	var distributed int64
	firstNonZero := -1
	for i, w := range weights {
		if w > 0 && firstNonZero < 0 {
			firstNonZero = i
		}
		limits[i] = target * w / totalWeight
		distributed += limits[i]
	}
	// Assign remainder to the first weighted category.
	remainder := target - distributed
	if remainder > 0 {
		if firstNonZero >= 0 {
			limits[firstNonZero] += remainder
		} else {
			limits[0] += remainder
		}
	}
	return limits
}

// Generate5030 produces a 50/30/20 budget proposal for the given income and
// expense categories.
//
//   - income is the monthly take-home income in minor currency units (e.g. cents).
//   - cats is the full list of categories; income-kind categories are skipped.
//   - txns is the transaction history used to weight limits by trailing spend
//     (the 3 most recent complete months before now). Categories with no
//     historical spend receive an even share of their bucket's target.
//   - now anchors the trailing-spend window and is typically time.Now().
//
// The three targets sum exactly to income: NeedsTarget = income×50/100,
// WantsTarget = income×30/100, SavingsTarget = income − NeedsTarget −
// WantsTarget (integer remainder goes to savings). Within each bucket the
// target is distributed proportionally by trailing spend; ties (all zero
// spend) are split evenly. An income of ≤ 0 returns an empty result.
func Generate5030(income int64, cats []domain.Category, txns []domain.Transaction, now time.Time) Split5030Result {
	if income <= 0 {
		return Split5030Result{}
	}

	needs := income * 50 / 100
	wants := income * 30 / 100
	savings := income - needs - wants // remainder → savings, so the three sum exactly to income

	result := Split5030Result{
		Income:        income,
		NeedsTarget:   needs,
		WantsTarget:   wants,
		SavingsTarget: savings,
	}

	// Separate expense categories by bucket.
	type catEntry struct {
		cat    domain.Category
		bucket Bucket
		spend  int64
	}

	bucketEntries := map[Bucket][]catEntry{
		BucketNeeds:   {},
		BucketWants:   {},
		BucketSavings: {},
	}

	for _, c := range cats {
		if c.Kind == domain.KindIncome {
			continue // income categories are not budgeted
		}
		b := Classify(c, nil)
		spend := trailingSpend(c.ID, txns, now)
		bucketEntries[b] = append(bucketEntries[b], catEntry{cat: c, bucket: b, spend: spend})
	}

	targets := map[Bucket]int64{
		BucketNeeds:   needs,
		BucketWants:   wants,
		BucketSavings: savings,
	}

	proposals := make([]BudgetProposal, 0, len(cats))

	// Process buckets in a stable order so Proposals is deterministic.
	for _, b := range []Bucket{BucketNeeds, BucketWants, BucketSavings} {
		entries := bucketEntries[b]
		if len(entries) == 0 {
			continue
		}
		target := targets[b]
		weights := make([]int64, len(entries))
		for i, e := range entries {
			weights[i] = e.spend
		}
		limits := distribute(target, weights)
		for i, e := range entries {
			proposals = append(proposals, BudgetProposal{
				Category:   e.cat,
				Bucket:     b,
				LimitMinor: limits[i],
			})
		}
	}

	result.Proposals = proposals
	return result
}
