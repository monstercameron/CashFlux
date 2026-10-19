// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
)

func (a *App) budgetPreferences() (prefs.Prefs, time.Time, error) {
	p := prefs.Default()
	raw, ok, err := a.store.GetSettingKV("cashflux:prefs")
	if err != nil {
		return p, time.Time{}, fmt.Errorf("read budget preferences: %w", err)
	}
	if ok {
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return p, time.Time{}, fmt.Errorf("read budget preferences: %w", err)
		}
	}
	p = p.Normalize()
	var anchor time.Time
	if p.PayCycleAnchor != "" {
		var err error
		anchor, err = time.Parse(time.DateOnly, p.PayCycleAnchor)
		if err != nil {
			return p, anchor, fmt.Errorf("read budget pay-cycle anchor: %w", err)
		}
	}
	return p, anchor, nil
}

// BudgetViewInput returns a scoped immutable budget evaluation input, using the
// application clock and persisted week-start and pay-cycle preferences.
func (a *App) BudgetViewInput(ref time.Time) (budgeting.ViewInput, error) {
	p, anchor, err := a.budgetPreferences()
	if err != nil {
		return budgeting.ViewInput{}, err
	}
	settings := a.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	transactions := a.Transactions()
	ledger := budgeting.NewNettedLedgerWithLinks(transactions, a.TxnLinks())
	return budgeting.ViewInput{Ledger: &ledger,
		Period:       budgeting.ViewPeriod{Anchor: ref, Now: a.clock(), WeekStart: p.WeekStartWeekday(), PayCycleAnchor: anchor},
		Transactions: transactions, Categories: a.Categories(), Recurring: a.Recurring(),
		Rates: currency.Rates{Base: base, Rates: settings.FXRates}, Base: base, PoolRollover: p.BudgetRolloverLeftover,
	}, nil
}

func (a *App) prepareBudgetWrite(b domain.Budget, effective time.Time) (domain.Budget, error) {
	p, anchor, err := a.budgetPreferences()
	if err != nil {
		return domain.Budget{}, err
	}
	start, _ := budgeting.PeriodRangeAnchored(b.Period, effective, p.WeekStartWeekday(), anchor)
	var previous *domain.Budget
	for _, old := range a.Budgets() {
		if old.ID == b.ID {
			copy := old
			previous = &copy
			break
		}
	}
	history := make(map[string]int64)
	if previous != nil {
		if previous.Rollover && b.Rollover {
			b.RolloverStart = previous.RolloverStart
		}
		for key, value := range previous.LimitHistory {
			history[key] = value
		}
		if previous.Limit.Currency != b.Limit.Currency && len(history) > 0 {
			return domain.Budget{}, fmt.Errorf("budget %q: changing currency would reinterpret limit history", b.ID)
		}
		if previous.Limit != b.Limit {
			baseline, _ := budgeting.PreviousPeriodRangeAnchored(previous.Period, start, p.WeekStartWeekday(), anchor)
			if !previous.RolloverStart.IsZero() && previous.RolloverStart.Before(baseline) {
				baseline = previous.RolloverStart
			}
			key := baseline.Format(time.DateOnly)
			if len(history) == 0 {
				history[key] = previous.Limit.Amount
			}
			history[start.Format(time.DateOnly)] = b.Limit.Amount
		}
	} else {
		for key, value := range b.LimitHistory {
			history[key] = value
		}
	}
	// Every new budget records its first funded period, because envelope mode
	// can be inherited from household settings without the Rollover flag.
	if previous == nil || b.Rollover && !previous.Rollover {
		b.RolloverStart = start
		history[start.Format(time.DateOnly)] = b.Limit.Amount
	}
	b.LimitHistory = history
	return b, nil
}

// SetRolloverStart explicitly repairs the first funded period of an existing
// budget, including an envelope inheriting its household methodology. It never
// guesses from imported transactions or the clock.
func (a *App) SetRolloverStart(budgetID string, ref time.Time) error {
	if ref.IsZero() {
		return fmt.Errorf("rollover start is required")
	}
	p, anchor, err := a.budgetPreferences()
	if err != nil {
		return err
	}
	for _, b := range a.Budgets() {
		if b.ID == budgetID {
			b.RolloverStart, _ = budgeting.PeriodRangeAnchored(b.Period, ref, p.WeekStartWeekday(), anchor)
			return a.PutBudget(b)
		}
	}
	return fmt.Errorf("budget %q not found", budgetID)
}

// BudgetOverTaskTitle returns the translation key and arguments for a budget
// flag task. Task.Custom preserves these for screen-side translation; Title is
// retained as a readable fallback for older clients and exports.
func BudgetOverTaskTitle(b domain.Budget) (string, []any) {
	if b.IsSaving() {
		return "tasks.budgetSavingShortfall", []any{b.Name}
	}
	return "tasks.budgetOverLimit", []any{b.Name}
}

// PutBudgetWithHistory saves a budget with a permanent change effective in the
// app clock's period. Creation and editor paths use this history-aware boundary.
func (a *App) PutBudgetWithHistory(b domain.Budget) error { return a.PutBudgetAt(b, a.clock()) }

// PutBudgetAt records limit history from the budget's own period containing ref
// before saving. Use an explicit ref when editing a viewed past or planned period.
func (a *App) PutBudgetAt(b domain.Budget, ref time.Time) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	prepared, err := a.prepareBudgetWrite(b, ref)
	if err != nil {
		return err
	}
	return a.PutBudget(prepared)
}
