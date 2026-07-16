// SPDX-License-Identifier: MIT

// Package reports is the pure, client-side reporting core for CashFlux (B21).
// Each report is a deterministic function over the domain data (transactions,
// rates) that returns plain result rows — no syscall/js, no charting, no I/O —
// so reports unit-test on native Go and the UI/chart layer renders them on top.
//
// Amounts are integer minor units in the base currency; foreign amounts are
// converted through the FX table like the rest of the ledger, and transfers are
// excluded (they move money between own accounts, they aren't spend or income).
package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

// CategorySpend is one category's spend over the reporting period, in base-
// currency minor units, with its change against the comparison period for the
// "vs last period" / top-movers view. CategoryID is empty for uncategorized
// spend; the caller resolves names and labels the empty id.
type CategorySpend struct {
	CategoryID string
	Amount     int64 // this period (absolute spend, base currency minor units)
	Prior      int64 // comparison period (0 when no comparison was requested)
	DeltaPct   int64 // percent change vs Prior (see HasDelta)
	HasDelta   bool  // whether DeltaPct is meaningful (a comparison ran with a non-zero prior)
	PriorZero  bool  // true when a comparison ran but the prior period was zero and current > 0 (C238: show "new" instead of hiding the badge)
}

// categoryTotals sums absolute expense amounts by category over [start, end),
// converted to the base currency. Transfers and income are excluded (IsExpense).
func categoryTotals(txns []domain.Transaction, start, end time.Time, rates currency.Rates) (map[string]int64, error) {
	txns = netted(txns) // XC2: fold refund-pair netting into per-category totals
	out := map[string]int64{}
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		// Split transactions (C58) attribute each line's amount to its own
		// category instead of the whole-transaction category, so per-category
		// spend is correct (and receipt-imported splits stop being invisible).
		//
		// XC10 limitation: a split line may carry its own owner (MemberID), but
		// this aggregation is NOT member-scoped — report member scoping happens
		// upstream on whole transactions (the caller pre-filters by t.MemberID
		// before passing txns here). A member-scoped report therefore attributes a
		// shared transaction's split lines by the payer, not by each line's owner.
		// Enforcing per-line owner here would require threading the member scope
		// into categoryTotals; deferred until reports scope members per line.
		if t.HasSplits() {
			for _, s := range t.Splits {
				conv, err := rates.Convert(s.Amount, rates.Base)
				if err != nil {
					return nil, err
				}
				out[s.CategoryID] += conv.Abs().Amount
			}
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		out[t.CategoryID] += conv.Abs().Amount
	}
	return out, nil
}

// SpendingByCategory totals expenses by category over [start, end) in the base
// currency, largest first (ties broken by category id for determinism). When
// compare is true it also computes each category's spend over the prior period
// [priorStart, priorEnd) and the percent change, so the report can show "vs last
// period" and rank top movers; categories that had spend only in the prior
// period are included with a zero current amount so a drop to zero still shows.
// With compare false the prior fields are left zero and HasDelta is false.
func SpendingByCategory(
	txns []domain.Transaction,
	start, end time.Time,
	compare bool,
	priorStart, priorEnd time.Time,
	rates currency.Rates,
) ([]CategorySpend, error) {
	cur, err := categoryTotals(txns, start, end, rates)
	if err != nil {
		return nil, err
	}
	var prior map[string]int64
	if compare {
		prior, err = categoryTotals(txns, priorStart, priorEnd, rates)
		if err != nil {
			return nil, err
		}
	}

	// Union of category ids across both periods so a category that dropped to
	// zero this period (but had prior spend) still appears as a mover.
	ids := make(map[string]struct{}, len(cur)+len(prior))
	for id := range cur {
		ids[id] = struct{}{}
	}
	for id := range prior {
		ids[id] = struct{}{}
	}

	out := make([]CategorySpend, 0, len(ids))
	for id := range ids {
		row := CategorySpend{CategoryID: id, Amount: cur[id]}
		if compare {
			row.Prior = prior[id]
			pct, ok := ledger.PercentChange(row.Amount, row.Prior)
			row.DeltaPct, row.HasDelta = pct, ok
			// C238: when the prior period was zero but there IS spend this period,
			// suppress %-change (undefined) and flag PriorZero so the UI can render
			// a "new" badge instead of hiding the delta indicator entirely.
			if !ok && row.Amount > 0 {
				row.PriorZero = true
			}
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out, nil
}

// Total sums the current-period amounts of a category-spend report (base
// currency minor units) — the report's headline spend figure.
func Total(rows []CategorySpend) int64 {
	var sum int64
	for _, r := range rows {
		sum += r.Amount
	}
	return sum
}
