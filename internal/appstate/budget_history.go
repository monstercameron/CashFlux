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
	return a.budgetViewInput(ref, p, anchor), nil
}

func (a *App) budgetViewInput(ref time.Time, p prefs.Prefs, anchor time.Time) budgeting.ViewInput {
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
	}
}

// prepareBudgetWrite is the single permanent-limit policy used by individual
// and batch writes. Limit remains the editable standing amount; callers cannot
// replace existing history by supplying their own map. A permanent edit replaces
// all later scheduled amounts, so editing a past period also changes the future.
// A cadence change starts a fresh funding grid: history, carry and boosts from
// the old grid are not reinterpreted as amounts for the new cadence. Dated journal
// notes are retained. Existing-budget currency changes remain refused.
func prepareBudgetWrite(b domain.Budget, previous *domain.Budget, effective time.Time, weekStart time.Weekday, anchor time.Time) (domain.Budget, error) {
	if effective.IsZero() {
		return domain.Budget{}, fmt.Errorf("budget effective date is required")
	}
	start, _ := budgeting.PeriodRangeAnchored(b.Period, effective, weekStart, anchor)
	history := make(map[string]int64)
	if previous != nil {
		for key, value := range previous.LimitHistory {
			if _, err := time.Parse(time.DateOnly, key); err != nil {
				return domain.Budget{}, fmt.Errorf("budget %q limit history: %w", b.ID, err)
			}
			history[key] = value
		}
		if previous.Limit.Currency != b.Limit.Currency {
			return domain.Budget{}, fmt.Errorf("budget %q: changing currency would reinterpret limit history", b.ID)
		}
		if previous.Period != b.Period {
			b.RolloverStart = start
			b.LimitHistory = map[string]int64{start.Format(time.DateOnly): b.Limit.Amount}
			b.PeriodBoosts = nil
			b.CoveredAt = time.Time{}
			if b.RecurringCover != nil {
				rc := *b.RecurringCover
				rc.LastAppliedPeriod = ""
				b.RecurringCover = &rc
			}
			return b, nil
		}
		// Repair rows written by older clients whose standing limit drifted
		// away from their latest history entry, even on an unchanged re-save.
		latest := ""
		for key := range history {
			if key > latest {
				latest = key
			}
		}
		if previous.Limit != b.Limit || latest != "" && history[latest] != b.Limit.Amount {
			baseline, _ := budgeting.PreviousPeriodRangeAnchored(previous.Period, start, weekStart, anchor)
			if !previous.RolloverStart.IsZero() && previous.RolloverStart.Before(baseline) {
				baseline = previous.RolloverStart
			}
			prior, err := budgeting.BudgetBaseAt(*previous, baseline)
			if err != nil {
				return domain.Budget{}, err
			}
			// BudgetBaseAt extends the earliest entry backwards. Preserve that
			// implicit baseline when editing before the oldest recorded change.
			hasEarlier := false
			for key := range history {
				if key < start.Format(time.DateOnly) {
					hasEarlier = true
					break
				}
			}
			if !hasEarlier {
				history[baseline.Format(time.DateOnly)] = prior.Amount
			}
			for key := range history {
				if key >= start.Format(time.DateOnly) {
					delete(history, key)
				}
			}
			history[start.Format(time.DateOnly)] = b.Limit.Amount
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

// PutBudgetAt records a permanent change from the budget's period containing ref,
// replacing later limit changes. Earlier periods retain their recorded amounts.
// Use an explicit ref when editing a viewed past or planned period.
func (a *App) PutBudgetAt(b domain.Budget, ref time.Time) error {
	return a.putBudgetsAt([]domain.Budget{b}, ref, true)
}
