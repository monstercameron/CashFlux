// SPDX-License-Identifier: MIT

package budgeting

import (
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
	CategoryCount map[string]int
	CategoryTotal map[string]int64
	TagCount      map[string]int
	TagTotal      map[string]int64
}

// ComputeTrackingMetadata totals reportable expense transactions within the
// half-open period [start, end). Split expenses count once per category line;
// tags count the full transaction once per tag, matching the picker display.
// Amounts that cannot be converted are omitted, as in the prior picker logic.
func ComputeTrackingMetadata(txns []domain.Transaction, start, end time.Time, base string, rates currency.Rates) TrackingMetadata {
	result := TrackingMetadata{
		CategoryCount: make(map[string]int),
		CategoryTotal: make(map[string]int64),
		TagCount:      make(map[string]int),
		TagTotal:      make(map[string]int64),
	}
	for _, txn := range txns {
		if !txn.CountsInReports() || txn.IsTransfer() || !txn.Amount.IsNegative() || !dateutil.InRange(txn.Date, start, end) {
			continue
		}
		whole, err := currency.ConvertBetween(txn.Amount.Abs().Amount, txn.Amount.Currency, base, rates)
		if err != nil {
			continue
		}
		if txn.HasSplits() {
			for _, split := range txn.Splits {
				amount, err := currency.ConvertBetween(split.Amount.Abs().Amount, split.Amount.Currency, base, rates)
				if err != nil {
					continue
				}
				result.CategoryCount[split.CategoryID]++
				result.CategoryTotal[split.CategoryID], _ = money.Add(result.CategoryTotal[split.CategoryID], amount)
			}
		} else {
			result.CategoryCount[txn.CategoryID]++
			result.CategoryTotal[txn.CategoryID], _ = money.Add(result.CategoryTotal[txn.CategoryID], whole)
		}
		for _, tag := range txn.Tags {
			key := strings.ToLower(strings.TrimSpace(tag))
			if key == "" {
				continue
			}
			result.TagCount[key]++
			result.TagTotal[key], _ = money.Add(result.TagTotal[key], whole)
		}
	}
	return result
}
