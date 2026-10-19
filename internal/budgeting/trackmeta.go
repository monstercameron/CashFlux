// SPDX-License-Identifier: MIT

package budgeting

import (
	"errors"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TrackingMetadata contains period expense counts and totals used by the budget
// tracking editor's category and tag pickers. Amounts are in the requested base
// currency's minor units.
type TrackingMetadata struct {
	// Error marks incomplete metadata; successful categories remain available.
	Error         error
	CategoryCount map[string]int
	CategoryTotal map[string]int64
	TagCount      map[string]int
	TagTotal      map[string]int64
}

// ComputeTrackingMetadata totals reportable expense transactions within the
// half-open period [start, end). Split expenses count once per category line;
// tags count the full transaction once per tag, matching the picker display.
// Failed amounts are omitted and Error marks the result as incomplete.
func ComputeTrackingMetadata(txns []domain.Transaction, start, end time.Time, base string, rates currency.Rates) TrackingMetadata {
	result := TrackingMetadata{
		CategoryCount: make(map[string]int),
		CategoryTotal: make(map[string]int64),
		TagCount:      make(map[string]int),
		TagTotal:      make(map[string]int64),
	}
	add := func(target map[string]int64, key string, amount money.Money) bool {
		converted, err := rates.Convert(amount, base)
		var next int64
		if err == nil {
			next, err = money.Add(target[key], converted.Amount)
		}
		if err != nil {
			result.Error = errors.Join(result.Error, err)
			return false
		}
		target[key] = next
		return true
	}
	for _, txn := range nettedForSpending(txns) {
		if !dateutil.InRange(txn.Date, start, end) {
			continue
		}
		whole, ok, err := ContributionChecked(domain.Budget{}, txn, func(string) bool { return true })
		if err != nil {
			result.Error = errors.Join(result.Error, err)
			continue
		}
		if !ok {
			continue
		}
		categories := map[string]int{txn.CategoryID: 1}
		if txn.HasSplits() {
			categories = map[string]int{}
			for _, s := range txn.Splits {
				categories[s.CategoryID]++
			}
		}
		for category, count := range categories {
			amount, ok, err := ContributionChecked(domain.Budget{}, txn, func(id string) bool { return id == category })
			if err != nil {
				result.Error = errors.Join(result.Error, err)
				continue
			}
			if ok && add(result.CategoryTotal, category, amount) {
				result.CategoryCount[category] += count
			}
		}
		for _, tag := range txn.Tags {
			key := strings.ToLower(strings.TrimSpace(tag))
			if key == "" {
				continue
			}
			if add(result.TagTotal, key, whole) {
				result.TagCount[key]++
			}
		}
	}
	return result
}
