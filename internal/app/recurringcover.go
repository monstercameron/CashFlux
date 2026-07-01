// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/coverformula"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// applyRecurringCovers re-applies every budget's standing recurring cover once per
// period. For each destination budget with a RecurringCover, when the current period
// differs from the one last applied, it moves the configured amount into it — split
// across the source budgets by weight — and stamps the period so it can't run twice.
//
// Amount and weights may be FORMULAS: the amount is evaluated in the destination
// budget's context (so `overspend` re-covers whatever the shortfall is that period),
// and each source's weight in that source's context (so a weight can track e.g.
// `cf_budget_priority`). A blank formula falls back to the fixed AmountMinor / Weight.
//
// It is drain-safe: a source that can no longer give its share is simply skipped
// (app.CoverBudget errors are ignored per source), so a depleted source never blocks
// the rest. Called on boot (after the dataset hydrates).
func applyRecurringCovers() {
	app := appstate.Default
	if app == nil {
		return
	}
	now := time.Now()
	pr := uistate.LoadPrefs()
	weekStart := pr.WeekStartWeekday()
	var payCycleAnchor time.Time
	if pr.PayCycleAnchor != "" {
		if t, err := time.Parse("2006-01-02", pr.PayCycleAnchor); err == nil {
			payCycleAnchor = t
		}
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	mStart, mEnd := dateutil.MonthRange(now)
	surface := engineenv.Vars(engineenv.Data{
		Accounts: app.Accounts(), Transactions: app.Transactions(), Members: app.Members(),
		Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(), Recurring: app.Recurring(),
		Rates: rates, Now: now, PeriodStart: mStart, PeriodEnd: mEnd,
		CustomDefs: app.CustomFieldDefs(), Molecules: app.Molecules(),
	})
	ctx := coverformula.Context{
		Base: surface, Txns: app.Transactions(), Rates: rates, Now: now,
		WeekStart: weekStart, PayCycleAnchor: payCycleAnchor, Defs: app.CustomFieldDefs(),
	}
	byID := map[string]domain.Budget{}
	for _, bb := range app.Budgets() {
		byID[bb.ID] = bb
	}
	changed := false

	for _, b := range app.Budgets() {
		rc := b.RecurringCover
		if rc == nil || len(rc.Sources) == 0 {
			continue
		}
		start, _ := budgeting.PeriodRange(b.Period, now, weekStart)
		key := start.Format("2006-01-02")
		if rc.LastAppliedPeriod == key {
			continue // already covered this period
		}

		// Resolve the amount: a formula in the destination's context, else the fixed one.
		amt := rc.AmountMinor
		if strings.TrimSpace(rc.AmountFormula) != "" {
			if m, err := ctx.AmountMinor(rc.AmountFormula, b); err == nil {
				amt = m
			}
		}
		cur := b.Limit.Currency
		if cur == "" {
			cur = base
		}

		if amt > 0 {
			// Resolve each source's weight (formula in the source's context, else fixed).
			type sw struct {
				id string
				w  int
			}
			var srcs []sw
			totalW := 0
			for _, s := range rc.Sources {
				w := s.Weight
				if strings.TrimSpace(s.WeightFormula) != "" {
					if wf, err := ctx.Weight(s.WeightFormula, byID[s.BudgetID]); err == nil {
						w = wf
					}
				}
				if w <= 0 {
					w = 1
				}
				srcs = append(srcs, sw{s.BudgetID, w})
				totalW += w
			}
			if totalW > 0 {
				var assigned int64
				for i, s := range srcs {
					var share int64
					if i == len(srcs)-1 {
						share = amt - assigned // remainder to the last source
					} else {
						share = amt * int64(s.w) / int64(totalW)
					}
					assigned += share
					if share <= 0 {
						continue
					}
					_ = app.CoverBudget(s.id, b.ID, money.New(share, cur)) // drain-safe
				}
			}
		}

		// Stamp the period on the (re-fetched) destination so this runs at most once —
		// even when amt was 0 (a "cover overspend" rule with nothing to cover this
		// period shouldn't retry on every boot).
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
