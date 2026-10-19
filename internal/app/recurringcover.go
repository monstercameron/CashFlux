// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"
	"log/slog"
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
// Debits, credit and the completion stamp commit together as period boosts.
// A failed formula or depleted source leaves the whole cover available to retry.
// Called on boot (after the dataset hydrates).
func applyRecurringCovers() {
	app := appstate.Default
	if app == nil {
		return
	}
	now := dateutil.WallClockUTC(time.Now())
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
	changed := false

	for _, b := range app.Budgets() {
		rc := b.RecurringCover
		if rc == nil || len(rc.Sources) == 0 {
			continue
		}
		key := budgeting.PeriodKeyWithWeekStart(b.Period, now, payCycleAnchor, weekStart)
		if rc.LastAppliedPeriod == key {
			continue // already covered this period
		}

		byID := map[string]domain.Budget{}
		for _, bb := range app.Budgets() {
			byID[bb.ID] = bb
		}
		b = byID[b.ID]
		shares, err := recurringCoverShares(b, byID, ctx, base)
		if err == nil {
			err = app.ApplyRecurringBudgetCoverAt(b.ID, shares, now)
		}
		if err != nil {
			slog.Error("apply recurring budget cover", "budget", b.ID, "err", err)
			continue
		}
		changed = true
	}
	if changed {
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}
}

func recurringCoverShares(b domain.Budget, byID map[string]domain.Budget, ctx coverformula.Context, base string) (map[string]money.Money, error) {
	rc := b.RecurringCover
	amt := rc.AmountMinor
	if strings.TrimSpace(rc.AmountFormula) != "" {
		var err error
		amt, err = ctx.AmountMinor(rc.AmountFormula, b)
		if err != nil {
			return nil, fmt.Errorf("recurring cover amount: %w", err)
		}
	}
	shares := make(map[string]money.Money)
	if amt <= 0 {
		return shares, nil
	}
	weights := make([]int64, len(rc.Sources))
	var total int64
	for i, source := range rc.Sources {
		from, ok := byID[source.BudgetID]
		if !ok {
			return nil, fmt.Errorf("recurring cover source %q not found", source.BudgetID)
		}
		w := source.Weight
		if strings.TrimSpace(source.WeightFormula) != "" {
			var err error
			w, err = ctx.Weight(source.WeightFormula, from)
			if err != nil {
				return nil, fmt.Errorf("recurring cover weight: %w", err)
			}
		}
		if w <= 0 {
			w = 1
		}
		weights[i] = int64(w)
		var err error
		total, err = money.Add(total, weights[i])
		if err != nil {
			return nil, fmt.Errorf("recurring cover weights: %w", err)
		}
	}
	cur := b.WithBaseCurrency(base).Limit.Currency
	var assigned int64
	for i, source := range rc.Sources {
		share := amt - assigned
		if i != len(rc.Sources)-1 {
			var err error
			share, err = money.MulDiv(amt, weights[i], total, money.RoundTowardZero)
			if err != nil {
				return nil, fmt.Errorf("recurring cover share: %w", err)
			}
		}
		assigned += share // bounded by the positive total amount
		if share == 0 {
			continue
		}
		prior := shares[source.BudgetID]
		if prior.Currency == "" {
			prior = money.Zero(cur)
		}
		var err error
		shares[source.BudgetID], err = prior.Add(money.New(share, cur))
		if err != nil {
			return nil, fmt.Errorf("recurring cover source total: %w", err)
		}
	}
	return shares, nil
}
