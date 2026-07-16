// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// TagSpend is one tag's expense total for a period: the tag (first-seen
// casing), how much was spent on charges carrying it, how many charges, and
// the prior-period total for trending. Because a single charge can carry
// several tags it counts toward each of them — tag totals deliberately
// overlap and must not be summed into a grand total.
type TagSpend struct {
	Tag    string
	Amount int64 // absolute minor units, base currency
	Count  int   // charges carrying the tag this period
	Prior  int64 // absolute minor units in the comparison period (0 when !compare)
}

// SpendingByTag totals expense transactions per tag over [start, end),
// optionally alongside a prior period for trending. Tags fold
// case-insensitively (the first casing seen wins), transfers and
// excluded-from-reports charges are skipped, refund-pair netting applies, and
// amounts convert to the base currency. Rows sort by current amount
// descending (then tag for stability); a tag with prior spend but nothing
// current still appears so a dropped habit shows as a mover.
func SpendingByTag(
	txns []domain.Transaction,
	start, end time.Time,
	compare bool,
	priorStart, priorEnd time.Time,
	rates currency.Rates,
) ([]TagSpend, error) {
	cur, counts, names, err := tagTotals(txns, start, end, rates)
	if err != nil {
		return nil, err
	}
	var prior map[string]int64
	if compare {
		var priorNames map[string]string
		prior, _, priorNames, err = tagTotals(txns, priorStart, priorEnd, rates)
		if err != nil {
			return nil, err
		}
		for k, n := range priorNames {
			if _, ok := names[k]; !ok {
				names[k] = n
			}
		}
	}

	keys := make(map[string]struct{}, len(cur)+len(prior))
	for k := range cur {
		keys[k] = struct{}{}
	}
	for k := range prior {
		keys[k] = struct{}{}
	}
	out := make([]TagSpend, 0, len(keys))
	for k := range keys {
		out = append(out, TagSpend{Tag: names[k], Amount: cur[k], Count: counts[k], Prior: prior[k]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].Tag < out[j].Tag
	})
	return out, nil
}

// tagTotals accumulates per-tag absolute expense totals, charge counts, and
// the display casing for one period.
func tagTotals(txns []domain.Transaction, start, end time.Time, rates currency.Rates) (map[string]int64, map[string]int, map[string]string, error) {
	txns = netted(txns)
	amounts := map[string]int64{}
	counts := map[string]int{}
	names := map[string]string{}
	for _, t := range txns {
		if len(t.Tags) == 0 || !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, nil, nil, err
		}
		amt := conv.Abs().Amount
		seen := map[string]struct{}{} // a duplicated tag on one charge counts once
		for _, tag := range t.Tags {
			trimmed := strings.TrimSpace(tag)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			if _, ok := names[key]; !ok {
				names[key] = trimmed
			}
			amounts[key] += amt
			counts[key]++
		}
	}
	return amounts, counts, names, nil
}
