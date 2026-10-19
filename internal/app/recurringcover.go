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
// budget's last closed period context (so `overspend` settles that shortfall),
// and each source's weight in that source's context (so a weight can track e.g.
// `cf_budget_priority`). A blank formula falls back to the fixed AmountMinor / Weight.
//
// Debits, credit and the completion stamp commit together as period boosts.
// A failed formula or depleted source leaves the whole cover available to retry.
// Called on boot (after the dataset hydrates).
func applyRecurringCovers() {
	applyRecurringCoversAt(time.Now())
}

func applyRecurringCoversAt(ref time.Time) {
	app := appstate.Default
	if app == nil {
		return
	}
	now := dateutil.WallClockUTC(ref)
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
	budgetLedger := budgeting.NewNettedLedger(app.Transactions(), app.TxnLinks())
	changed := false

	for _, b := range app.Budgets() {
		rc := b.RecurringCover
		if rc == nil || len(rc.Sources) == 0 {
			continue
		}
		ref := now
		// Formula arrangements settle the last closed period, when its spending
		// and effective cap are known. A zero at the first boot of a new period
		// must not mark that still-open period as already covered.
		if strings.TrimSpace(rc.AmountFormula) != "" {
			start, _ := budgeting.PeriodRangeAnchored(b.Period, now, weekStart, payCycleAnchor)
			ref = start.Add(-time.Nanosecond)
		}
		key := budgeting.PeriodKeyWithWeekStart(b.Period, ref, payCycleAnchor, weekStart)
		if rc.LastAppliedPeriod == key {
			continue // already covered this period
		}

		byID := map[string]domain.Budget{}
		for _, bb := range app.Budgets() {
			byID[bb.ID] = bb
		}
		b = byID[b.ID]
		mStart, mEnd := dateutil.MonthRange(ref)
		surface := engineenv.Vars(engineenv.Data{
			Ledger: budgetLedger, Accounts: app.Accounts(), Transactions: app.Transactions(), Members: app.Members(),
			Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(), Recurring: app.Recurring(),
			Rates: rates, Now: ref, PeriodStart: mStart, PeriodEnd: mEnd, WeekStart: weekStart,
			Categories: app.Categories(),
			CustomDefs: app.CustomFieldDefs(), Molecules: app.Molecules(),
		})
		periodContext := coverformula.Context{
			Base: surface, Txns: budgetLedger, Rates: rates, Now: ref,
			WeekStart: weekStart, PayCycleAnchor: payCycleAnchor, Defs: app.CustomFieldDefs(),
			Categories: app.Categories(), Recurring: app.Recurring(), PoolRollover: pr.BudgetRolloverLeftover,
		}
		shares, err := recurringCoverShares(b, byID, periodContext, base)
		if err == nil {
			err = app.ApplyRecurringBudgetCoverAt(b.ID, shares, ref)
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
	inputs := make([]budgeting.CoverSplitInput, 0, len(rc.Sources))
	seen := map[string]bool{}
	cur := b.WithBaseCurrency(base).Limit.Currency
	for _, source := range rc.Sources {
		from, ok := byID[source.BudgetID]
		if !ok || source.BudgetID == b.ID || seen[source.BudgetID] {
			return nil, fmt.Errorf("missing, duplicate or self recurring cover source %q", source.BudgetID)
		}
		seen[source.BudgetID] = true
		from = from.WithBaseCurrency(base)
		if from.Limit.Currency != cur {
			return nil, budgeting.ErrTransferCurrency
		}
		funds, err := budgeting.CoverSourceFunds(from, ctx.ViewInput())
		if err != nil {
			return nil, err
		}
		weight := source.Weight
		if strings.TrimSpace(source.WeightFormula) != "" {
			weight, err = ctx.Weight(source.WeightFormula, from)
			if err != nil {
				return nil, fmt.Errorf("recurring cover weight: %w", err)
			}
		}
		inputs = append(inputs, budgeting.CoverSplitInput{ID: from.ID, Cap: funds.Free.Amount, Weight: weight})
	}
	split, short := budgeting.CoverSplit(amt, inputs)
	if short > 0 {
		return nil, fmt.Errorf("%w: recurring cover short by %d minor units", budgeting.ErrInsufficientSource, short)
	}
	for id, share := range split {
		if share > 0 {
			shares[id] = money.New(share, cur)
		}
	}
	return shares, nil
}
