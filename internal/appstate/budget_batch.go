// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/validate"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// PutBudgetsAtomic validates and persists budget creates and updates as
// one SQLite transaction. No budget changes are visible if validation or the
// underlying transaction fails.
func (a *App) PutBudgetsAtomic(writes []domain.Budget) error {
	return a.PutBudgetsAtomicAt(writes, a.clock())
}

// PutBudgetsAtomicAt records each budget's history from its own period containing
// ref and commits creates and updates together. Existing history remains owned
// by the stored row; new rows start their history at ref. Drafts are not mutated.
func (a *App) PutBudgetsAtomicAt(writes []domain.Budget, ref time.Time) error {
	return a.putBudgetsAt(writes, ref, true)
}

// putBudgetsAt snapshots preferences and the budget index once. Both the single
// upsert and batch APIs use the same history and validation policy.
func (a *App) putBudgetsAt(writes []domain.Budget, ref time.Time, allowCreate bool) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if len(writes) == 0 {
		return fmt.Errorf("appstate: budget batch is empty")
	}
	p, anchor, err := a.budgetPreferences()
	if err != nil {
		return err
	}
	prepared := make([]domain.Budget, len(writes))
	base := a.baseCurrency()
	seen := make(map[string]bool, len(writes))
	current := make(map[string]domain.Budget, len(writes))
	for _, budget := range a.Budgets() {
		current[budget.ID] = budget
	}
	for i, budget := range writes {
		budget = budget.WithBaseCurrency(base)
		if seen[budget.ID] {
			return fmt.Errorf("appstate: budget %q appears more than once in batch", budget.ID)
		}
		seen[budget.ID] = true
		var previous *domain.Budget
		if old, ok := current[budget.ID]; ok {
			old = old.WithBaseCurrency(base)
			previous = &old
		} else if !allowCreate {
			return fmt.Errorf("appstate: budget %q not found", budget.ID)
		}
		prepared[i], err = prepareBudgetWrite(budget, previous, ref, p.WeekStartWeekday(), anchor)
		if err != nil {
			return err
		}
		budget = prepared[i]
		if is := validate.ValidateBudget(budget); !is.OK() {
			return fmt.Errorf("appstate: validate budget %q: %w", budget.ID, is)
		}
		if err := a.validateCustom("budget", budget.Custom); err != nil {
			return fmt.Errorf("appstate: validate custom fields for budget %q: %w", budget.ID, err)
		}
	}
	// Most writes cannot fire an automation. When one is configured, net the
	// ledger once for the whole batch and evaluate each new state only once.
	// Only a newly-over state needs the corresponding old-state evaluation.
	trigger := false
	if !a.triggersSuspended {
		for _, w := range a.Workflows() {
			if w.Enabled && w.Trigger.Kind == workflow.TriggerBudgetExceeded {
				trigger = true
				break
			}
		}
	}
	fire := false
	if trigger {
		in := a.budgetViewInput(a.clock(), p, anchor)
		for _, budget := range prepared {
			next := budgeting.EvaluateViewBudget(budget, in)
			if !next.Valid || next.Status.State != budgeting.StateOver {
				continue
			}
			if old, ok := current[budget.ID]; ok {
				before := budgeting.EvaluateViewBudget(old, in)
				if before.Valid && before.Status.State == budgeting.StateOver {
					continue
				}
			}
			fire = true
		}
	}
	// Keep the single-row store boundary (and its change-capture hooks) for
	// upserts; multi-row actions use the transactional batch boundary.
	if allowCreate && len(prepared) == 1 {
		err = a.store.PutBudget(prepared[0])
	} else {
		err = a.store.PutBudgetsAtomic(prepared)
	}
	if err != nil {
		return fmt.Errorf("appstate: commit budget batch: %w", err)
	}
	for _, budget := range prepared {
		a.log.Info("budget saved", "id", budget.ID)
	}
	if fire {
		a.RunTriggered(workflow.TriggerBudgetExceeded, nil)
	}
	return nil
}

// ApplyRecurringBudgetCoverAt applies a resolved standing cover once for the
// destination period containing ref. All debits, the credit and the replay stamp
// commit together. Insufficient sources fail the whole cover, allowing a safe
// retry; neither a failed write nor another boot can compound a partial cover.
func (a *App) ApplyRecurringBudgetCoverAt(destinationID string, shares map[string]money.Money, ref time.Time) error {
	return a.coverBudgetsAt(destinationID, shares, ref, true)
}

func (a *App) coverBudgetsAt(destinationID string, shares map[string]money.Money, ref time.Time, recurring bool) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if ref.IsZero() {
		return fmt.Errorf("cover effective date is required")
	}
	ref = dateutil.WallClockUTC(ref)
	current := make(map[string]domain.Budget)
	for _, b := range a.Budgets() {
		current[b.ID] = b
	}
	to, ok := current[destinationID]
	if !ok {
		return fmt.Errorf("appstate: destination budget %q not found", destinationID)
	}
	in, err := a.BudgetViewInput(ref)
	if err != nil {
		return err
	}
	to = to.WithBaseCurrency(in.Base)
	p := in.Period
	key := budgeting.PeriodKeyWithWeekStart(to.Period, ref, p.PayCycleAnchor, p.WeekStart)
	if recurring {
		if to.RecurringCover == nil {
			return fmt.Errorf("budget %q has no recurring cover", destinationID)
		}
		if to.RecurringCover.LastAppliedPeriod == key {
			return nil
		}
		rc := *to.RecurringCover
		rc.LastAppliedPeriod = key
		to.RecurringCover = &rc
	}
	ids := make([]string, 0, len(shares))
	for id := range shares {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	writes := make([]domain.Budget, 0, len(shares)+1)
	total := money.Zero(to.Limit.Currency)
	for _, id := range ids {
		if id == destinationID {
			return budgeting.ErrTransferSameBudget
		}
		amount := shares[id]
		if !amount.IsPositive() {
			return budgeting.ErrTransferNonPositive
		}
		from, ok := current[id]
		if !ok {
			return fmt.Errorf("appstate: source budget %q not found", id)
		}
		from = from.WithBaseCurrency(in.Base)
		if from.Limit.Currency != to.Limit.Currency || amount.Currency != to.Limit.Currency {
			return budgeting.ErrTransferCurrency
		}
		v := budgeting.EvaluateViewBudget(from, in)
		if !v.Valid {
			return fmt.Errorf("evaluate cover source %q: %w", id, v.Error)
		}
		remaining, err := v.Status.Remaining.Sub(amount)
		if err != nil {
			return fmt.Errorf("cover source %q room: %w", id, err)
		}
		if !remaining.IsPositive() {
			return fmt.Errorf("%w: %s would have nothing left", budgeting.ErrInsufficientSource, from.Name)
		}
		start, _ := budgeting.PeriodRangeAnchored(from.Period, ref, p.WeekStart, p.PayCycleAnchor)
		from, err = budgetWithBoost(from, start, amount.Neg())
		if err != nil {
			return err
		}
		writes = append(writes, from)
		total, err = total.Add(amount)
		if err != nil {
			return fmt.Errorf("cover total: %w", err)
		}
	}
	if total.IsPositive() {
		start, _ := budgeting.PeriodRangeAnchored(to.Period, ref, p.WeekStart, p.PayCycleAnchor)
		to, err = budgetWithBoost(to, start, total)
		if err != nil {
			return err
		}
		to.CoveredAt = ref
	}
	writes = append(writes, to)
	return a.PutBudgetsAtomicAt(writes, ref)
}

func budgetWithBoost(b domain.Budget, start time.Time, delta money.Money) (domain.Budget, error) {
	amount, err := money.New(b.PeriodBoost(start), b.Limit.Currency).Add(delta)
	if err != nil {
		return domain.Budget{}, fmt.Errorf("budget %q period boost: %w", b.ID, err)
	}
	boosts := make(map[string]int64, len(b.PeriodBoosts)+1)
	for key, value := range b.PeriodBoosts {
		boosts[key] = value
	}
	key := start.Format(time.DateOnly)
	if amount.Amount == 0 {
		delete(boosts, key)
	} else {
		boosts[key] = amount.Amount
	}
	b.PeriodBoosts = boosts
	return b, nil
}
