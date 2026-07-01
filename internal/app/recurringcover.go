// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// applyRecurringCovers re-applies every budget's standing recurring cover once per
// period. For each destination budget with a RecurringCover, when the current period
// differs from the one last applied, it moves the configured amount into it — split
// across the source budgets by weight — and stamps the period so it can't run twice.
//
// It is drain-safe: a source that can no longer give its share is simply skipped
// (app.CoverBudget errors are ignored per source), so a depleted source never blocks
// the rest. Called on boot (after the dataset hydrates) and when /budgets mounts.
func applyRecurringCovers() {
	app := appstate.Default
	if app == nil {
		return
	}
	now := time.Now()
	weekStart := uistate.LoadPrefs().WeekStartWeekday()
	changed := false

	for _, b := range app.Budgets() {
		rc := b.RecurringCover
		if rc == nil || rc.AmountMinor <= 0 || len(rc.Sources) == 0 {
			continue
		}
		start, _ := budgeting.PeriodRange(b.Period, now, weekStart)
		key := start.Format("2006-01-02")
		if rc.LastAppliedPeriod == key {
			continue // already covered this period
		}

		totalW := 0
		for _, s := range rc.Sources {
			w := s.Weight
			if w <= 0 {
				w = 1
			}
			totalW += w
		}
		if totalW == 0 {
			continue
		}
		cur := b.Limit.Currency
		if cur == "" {
			cur = app.Settings().BaseCurrency
		}
		var assigned int64
		n := len(rc.Sources)
		for i, s := range rc.Sources {
			w := s.Weight
			if w <= 0 {
				w = 1
			}
			var share int64
			if i == n-1 {
				share = rc.AmountMinor - assigned // remainder to the last source
			} else {
				share = rc.AmountMinor * int64(w) / int64(totalW)
			}
			assigned += share
			if share <= 0 {
				continue
			}
			_ = app.CoverBudget(s.BudgetID, b.ID, money.New(share, cur)) // drain-safe: skip a source that can't give
		}

		// Stamp the period on the (re-fetched) destination so this runs at most once.
		for _, nb := range app.Budgets() {
			if nb.ID == b.ID && nb.RecurringCover != nil {
				nb.RecurringCover.LastAppliedPeriod = key
				if err := app.PutBudget(nb); err == nil {
					changed = true
				}
				break
			}
		}
	}
	if changed {
		uistate.BumpDataRevision()
	}
}
