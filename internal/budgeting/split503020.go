// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
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

// trailingSpend returns the category's checked, base-currency contribution over
// the three complete months before now. Split lines are attributed to their own
// categories, and unsupported currencies fail rather than being added as if they
// shared a minor unit.
func trailingSpend(catID string, txns NettedLedger, now time.Time, rates currency.Rates) (int64, error) {

	// Window: the 3 complete calendar months before the current month.
	curMonthStart := dateutil.MonthStart(now)
	windowEnd := curMonthStart
	windowStart := dateutil.AddMonths(curMonthStart, -3)

	var total int64
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() {
			continue
		}
		if t.Date.Before(windowStart) || !t.Date.Before(windowEnd) {
			continue
		}
		part, ok, err := ContributionChecked(domain.Budget{}, t, func(id string) bool { return id == catID })
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		converted, err := rates.Convert(part, rates.Base)
		if err != nil {
			return 0, err
		}
		total, err = money.Add(total, converted.Amount)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
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
func Generate5030(income int64, cats []domain.Category, txns NettedLedger, now time.Time) Split5030Result {
	result, _ := Generate5030WithRates(income, cats, txns, now, currency.Rates{Base: "USD"})
	return result
}

// Generate5030WithRates builds the 50/30/20 proposal using checked split-aware
// contributions converted into the selected base currency.
func Generate5030WithRates(income int64, cats []domain.Category, txns NettedLedger, now time.Time, rates currency.Rates) (Split5030Result, error) {
	if income <= 0 {
		return Split5030Result{}, nil
	}

	needs, err := money.MulDiv(income, 50, 100, money.RoundTowardZero)
	if err != nil {
		return Split5030Result{}, fmt.Errorf("calculate needs target: %w", err)
	}
	wants, err := money.MulDiv(income, 30, 100, money.RoundTowardZero)
	if err != nil {
		return Split5030Result{}, fmt.Errorf("calculate wants target: %w", err)
	}
	savings, err := money.Sub(income, needs)
	if err == nil {
		savings, err = money.Sub(savings, wants)
	}
	if err != nil {
		return Split5030Result{}, fmt.Errorf("calculate savings target: %w", err)
	}

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
		spend, err := trailingSpend(c.ID, txns, now, rates)
		if err != nil {
			return Split5030Result{}, err
		}
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
		limits, err := distributeChecked(target, weights)
		if err != nil {
			return Split5030Result{}, err
		}
		for i, e := range entries {
			proposals = append(proposals, BudgetProposal{
				Category:   e.cat,
				Bucket:     b,
				LimitMinor: limits[i],
			})
		}
	}

	result.Proposals = proposals
	return result, nil
}

func distributeChecked(target int64, weights []int64) ([]int64, error) {
	n := len(weights)
	if n == 0 || target <= 0 {
		return make([]int64, n), nil
	}
	var totalWeight int64
	for _, weight := range weights {
		if weight < 0 {
			return nil, fmt.Errorf("negative spending weight")
		}
		var err error
		totalWeight, err = money.Add(totalWeight, weight)
		if err != nil {
			return nil, fmt.Errorf("sum template spending weights: %w", err)
		}
	}
	limits := make([]int64, n)
	if totalWeight == 0 {
		base := target / int64(n)
		for i := range limits {
			limits[i] = base
		}
		limits[0], _ = money.Add(limits[0], target-base*int64(n))
		return limits, nil
	}
	var distributed int64
	first := -1
	for i, weight := range weights {
		if weight > 0 && first < 0 {
			first = i
		}
		value, err := money.MulDiv(target, weight, totalWeight, money.RoundTowardZero)
		if err != nil {
			return nil, fmt.Errorf("allocate template target: %w", err)
		}
		limits[i] = value
		distributed, err = money.Add(distributed, value)
		if err != nil {
			return nil, fmt.Errorf("sum template allocation: %w", err)
		}
	}
	remainder, err := money.Sub(target, distributed)
	if err != nil {
		return nil, fmt.Errorf("calculate template remainder: %w", err)
	}
	if first < 0 {
		first = 0
	}
	limits[first], err = money.Add(limits[first], remainder)
	if err != nil {
		return nil, fmt.Errorf("apply template remainder: %w", err)
	}
	return limits, nil
}
