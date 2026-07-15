// SPDX-License-Identifier: MIT

// Package attention ranks the urgent, act-now signals on the dashboard into a
// single ordered digest: bills due soon, near/over budgets, stale account
// balances, overdue or high-priority to-dos, and the biggest spending spike.
// It is pure (no syscall/js): callers compute the raw signals with the existing
// logic packages and hand them in; this package decides which to include (per a
// Config), scores severity, orders by urgency, and caps the count. The widget
// formats the structured Items for display.
package attention

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/taskrecur"
)

// Severity ranks how urgent an item is; higher is more urgent.
type Severity int

const (
	// SeverityInfo is a noteworthy-but-not-pressing signal (e.g. a spending spike).
	SeverityInfo Severity = iota
	// SeverityWarning is something to handle soon (near budget, stale balance,
	// a bill due within the window, a high-priority open task).
	SeverityWarning
	// SeverityCritical needs attention now (over budget, a bill due in <=2 days
	// or today, an overdue task).
	SeverityCritical
)

// Kind identifies which signal an item came from, so the view can pick an icon
// and phrase the detail line.
type Kind string

const (
	KindBill     Kind = "bill"
	KindBudget   Kind = "budget"
	KindStale    Kind = "stale"
	KindTask     Kind = "task"
	KindSpending Kind = "spending"
)

// Item is one ranked attention signal. It carries structured data (not display
// strings) so the widget can localize and format at the edge; Label is the raw
// user-entered name (account/bill/budget/task) and is safe to pass through.
type Item struct {
	Kind     Kind
	Severity Severity
	Label    string            // entity name / task title (user data)
	Amount   money.Money       // bill min-payment, budget overage context (zero = none)
	Days     int               // bill: days until due; task: days overdue; stale: days since update
	Pct      int               // budget: spent as percent of limit
	Route    string            // in-app route for the deep link (e.g. "/todo")
	AnchorID string            // entity id to anchor-scroll to, when one exists
	Anomaly  *insights.Anomaly // set only for KindSpending, for the view to phrase
	when     time.Time         // deadline used to order by soonness; zero sorts last
}

// Config selects which sources to include and how aggressively to surface them.
// It is the typed form of the widget's gear/flip-panel settings.
type Config struct {
	Bills    bool
	Budgets  bool
	Stale    bool
	Tasks    bool
	Spending bool

	BillsWindowDays int      // a bill is surfaced when due within this many days
	MaxItems        int      // cap on returned items (<=0 means no cap)
	MinSeverity     Severity // drop items below this severity
}

// DefaultConfig is the all-on configuration matching the widget's schema defaults.
func DefaultConfig() Config {
	return Config{
		Bills: true, Budgets: true, Stale: true, Tasks: true, Spending: true,
		BillsWindowDays: 7, MaxItems: 5, MinSeverity: SeverityInfo,
	}
}

// Inputs holds the already-computed raw signals. Callers build these from the
// live store with the existing logic packages (bills.UpcomingAll,
// budgeting.EvaluateRollup, freshness.VisibleStaleAccounts, app.Tasks,
// insights.Detect) so this package stays free of store/UI concerns.
type Inputs struct {
	Now     time.Time
	Bills   []bills.Bill
	Budgets []budgeting.Status
	Stale   []domain.Account
	Tasks   []domain.Task
	Anomaly *insights.Anomaly
}

// Rank turns the raw inputs into an urgency-ordered digest under cfg: only
// enabled sources contribute, items below cfg.MinSeverity are dropped, the rest
// are ordered by severity (highest first) then soonest deadline, and the list is
// capped at cfg.MaxItems. Deterministic for a given input.
func Rank(in Inputs, cfg Config) []Item {
	var items []Item

	if cfg.Bills {
		window := cfg.BillsWindowDays
		if window <= 0 {
			window = 7
		}
		for _, b := range in.Bills {
			if b.DaysUntil > window {
				continue
			}
			sev := SeverityWarning
			if b.DaysUntil <= 2 {
				sev = SeverityCritical
			}
			items = append(items, Item{
				Kind: KindBill, Severity: sev, Label: b.Name, Amount: b.Amount,
				Days: b.DaysUntil, Route: "/accounts", when: b.DueDate,
			})
		}
	}

	if cfg.Budgets {
		for _, s := range in.Budgets {
			sev := SeverityInfo
			switch s.State {
			case budgeting.StateOver:
				sev = SeverityCritical
			case budgeting.StateNear:
				sev = SeverityWarning
			default:
				continue // on-track budgets aren't "attention"
			}
			label := s.Budget.Name
			items = append(items, Item{
				Kind: KindBudget, Severity: sev, Label: label, Pct: s.Percent,
				Route: "/budgets", AnchorID: s.Budget.ID,
			})
		}
	}

	if cfg.Stale {
		for _, a := range in.Stale {
			days := daysSince(a.BalanceAsOf, in.Now)
			items = append(items, Item{
				Kind: KindStale, Severity: SeverityWarning, Label: a.Name, Days: days,
				Route: "/accounts", AnchorID: a.ID,
			})
		}
	}

	if cfg.Tasks {
		today := dayStart(in.Now)
		for _, t := range in.Tasks {
			if t.Status != domain.StatusOpen {
				continue
			}
			overdue := !t.Due.IsZero() && t.Due.Before(today)
			high := t.Priority == domain.PriorityHigh
			// A task also surfaces once its reminder window opens (Due − lead ≤ now),
			// so a dated to-do is nudged before it tips into overdue.
			reminder := taskrecur.ReminderDue(t, in.Now)
			if !overdue && !high && !reminder {
				continue
			}
			sev := SeverityWarning
			days := 0
			when := time.Time{}
			if overdue {
				sev = SeverityCritical
				days = daysSince(t.Due, in.Now)
				when = t.Due
			}
			items = append(items, Item{
				Kind: KindTask, Severity: sev, Label: t.Title, Days: days,
				Route: "/todo", AnchorID: t.ID, when: when,
			})
		}
	}

	if cfg.Spending && in.Anomaly != nil {
		items = append(items, Item{
			Kind: KindSpending, Severity: SeverityInfo, Label: in.Anomaly.Category,
			Route: "/insights", Anomaly: in.Anomaly,
		})
	}

	// Drop anything below the configured floor.
	if cfg.MinSeverity > SeverityInfo {
		kept := items[:0]
		for _, it := range items {
			if it.Severity >= cfg.MinSeverity {
				kept = append(kept, it)
			}
		}
		items = kept
	}

	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Severity != b.Severity {
			return a.Severity > b.Severity // most urgent first
		}
		// Within a severity, soonest deadline first; items without a deadline
		// (zero when) sort after dated ones.
		az, bz := a.when.IsZero(), b.when.IsZero()
		if az != bz {
			return bz // a has a date, b doesn't → a first
		}
		if !az && !a.when.Equal(b.when) {
			return a.when.Before(b.when)
		}
		return false // stable: preserve source order otherwise
	})

	if cfg.MaxItems > 0 && len(items) > cfg.MaxItems {
		items = items[:cfg.MaxItems]
	}
	return items
}

// Counts returns the number of critical and warning items in a ranked list, for
// the compact "+N need attention" summary.
func Counts(items []Item) (critical, warning int) {
	for _, it := range items {
		switch it.Severity {
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warning++
		}
	}
	return critical, warning
}

// daysSince returns whole days from t to now (positive when t is in the past).
func daysSince(t, now time.Time) int {
	if t.IsZero() {
		return 0
	}
	return int(dayStart(now).Sub(dayStart(t)).Hours() / 24)
}

// dayStart truncates to local midnight so day math ignores the time of day.
func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
