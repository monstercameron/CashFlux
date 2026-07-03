// SPDX-License-Identifier: MIT

package engineenv

// This file exposes the /assistant Insights surface's briefing figures as
// engine variables: the month-to-date spending story (spent so far, last
// month's total, last month at the same point — the "pace" baseline), the
// count of notable category shifts, and the top payee's trailing-90-day spend
// — as assistant_* variables usable in any formula or dashboard widget. The
// derivations are shared with the surface via the exported AssistantSpendStory
// and AssistantHighlights helpers, so an assistant_* figure always matches the
// page.

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// AssistantVarNames are the fixed assistant variables addAssistantVars exposes,
// in a stable order. Money figures are major units of the base currency.
var AssistantVarNames = []string{
	"assistant_spend_mtd",        // total spending this month so far
	"assistant_spend_prev",       // last month's total spending
	"assistant_spend_pace",       // last month's spending through the same day count
	"assistant_spend_pace_delta", // spend_mtd − spend_pace (signed; positive = ahead of pace)
	"assistant_highlights",       // count of notable category shifts this month
	"assistant_top_merchant",     // the top payee's spend over the trailing 90 days
}

func init() { Names = append(Names, AssistantVarNames...) }

// AssistantMerchantWindowDays is the trailing window the assistant surface (and
// the assistant_top_merchant variable) rank payees over.
const AssistantMerchantWindowDays = 90

// AssistantSpendStory computes the briefing's three spending figures in minor
// units of the base currency: spending this month so far (mtd), last month's
// total (prev), and last month's spending through the same number of elapsed
// days (pace) — the honest like-for-like baseline for "am I ahead or behind".
// Shared by the /assistant hero tile and addAssistantVars so they always agree.
func AssistantSpendStory(txns []domain.Transaction, rates currency.Rates, now time.Time) (mtd, prev, pace int64, err error) {
	curStart := dateutil.MonthStart(now)
	prevStart := dateutil.AddMonths(curStart, -1)
	// The MTD window is [curStart, tomorrow) so today's transactions count; the
	// pace window covers the same span of last month, clamped to the month edge
	// (a 31-day elapsed span never spills into the current month).
	end := now.AddDate(0, 0, 1)
	paceEnd := prevStart.Add(end.Sub(curStart))
	if paceEnd.After(curStart) {
		paceEnd = curStart
	}

	bounds := []time.Time{prevStart, paceEnd, curStart, end}
	if !paceEnd.Before(curStart) {
		// Full-month pace: the pace period IS the previous month.
		bounds = []time.Time{prevStart, curStart, end}
	}
	flows, ferr := reports.IncomeExpenseSeries(txns, bounds, rates)
	if ferr != nil || len(flows) < 2 {
		return 0, 0, 0, ferr
	}
	pace = flows[0].Expense
	prev = pace
	if len(flows) == 3 {
		prev += flows[1].Expense
	}
	mtd = flows[len(flows)-1].Expense
	return mtd, prev, pace, nil
}

// AssistantHighlights detects the notable per-category spending shifts the
// briefing surfaces: the last four monthly periods per category, decreases
// suppressed until the month is ~90% elapsed (an unspent category mid-month is
// not a real "down 100%"). uncategorized names the empty-category series for
// display. Shared by the /assistant highlights tile and addAssistantVars.
func AssistantHighlights(txns []domain.Transaction, categories []domain.Category, rates currency.Rates, now time.Time, uncategorized string) []insights.Anomaly {
	curStart := dateutil.MonthStart(now)
	bounds := []time.Time{
		dateutil.AddMonths(curStart, -3),
		dateutil.AddMonths(curStart, -2),
		dateutil.AddMonths(curStart, -1),
		curStart,
		dateutil.AddMonths(curStart, 1),
	}
	spendByCat, err := ledger.CategorySpendSeries(txns, bounds, rates)
	if err != nil || len(spendByCat) == 0 {
		return nil
	}
	names := make(map[string]string, len(categories))
	for _, c := range categories {
		names[c.ID] = c.Name
	}
	series := make([]insights.CategorySeries, 0, len(spendByCat))
	for catID, spend := range spendByCat {
		name := names[catID]
		if name == "" {
			name = uncategorized
		}
		series = append(series, insights.CategorySeries{Category: name, Spend: spend})
	}
	opts := insights.DefaultOptions()
	monthEnd := dateutil.AddMonths(curStart, 1)
	if monthDays := monthEnd.Sub(curStart).Hours() / 24; monthDays > 0 {
		if elapsed := now.Sub(curStart).Hours() / 24; elapsed/monthDays < 0.9 {
			opts.SuppressDecrease = true
		}
	}
	return insights.Detect(series, opts)
}

// addAssistantVars derives the assistant_* variables from the shared briefing
// helpers over the fundamental Data fields, so a formula figure always matches
// what the /assistant Insights surface renders.
func addAssistantVars(out map[string]float64, d Data, major func(int64) float64) {
	for _, name := range AssistantVarNames {
		out[name] = 0
	}

	if mtd, prev, pace, err := AssistantSpendStory(d.Transactions, d.Rates, d.Now); err == nil {
		out["assistant_spend_mtd"] = major(mtd)
		out["assistant_spend_prev"] = major(prev)
		out["assistant_spend_pace"] = major(pace)
		out["assistant_spend_pace_delta"] = major(mtd - pace)
	}

	out["assistant_highlights"] = float64(len(AssistantHighlights(d.Transactions, d.Categories, d.Rates, d.Now, "")))

	if top, err := reports.TopPayeesTrailing(d.Transactions, AssistantMerchantWindowDays, d.Now, d.Rates, 1); err == nil && len(top) > 0 {
		out["assistant_top_merchant"] = major(top[0].Amount)
	}
}
