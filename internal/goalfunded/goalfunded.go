// SPDX-License-Identifier: MIT

// Package goalfunded separates spending that a goal paid for from spending that
// broke a budget (WF18).
//
// Two transactions can look identical in a ledger — same amount, same category,
// same day — and mean opposite things. One blew the month's grocery budget. The
// other came out of a fund saved for exactly this purchase, over months, on
// purpose. Both are real spending; only one is overspending.
//
// A budget that cannot tell them apart punishes somebody for executing the plan
// they made, which is the fastest way to teach them that the budget is wrong and
// can be ignored.
//
// # What this package does NOT do
//
// It does not hide the spending. Goal-funded money still left the account, still
// appears in the ledger, and still counts toward what was spent — it is
// separated, not deducted. A budget screen that quietly shrank its own totals
// would be a different and worse lie than the one being fixed.
package goalfunded

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Split is a period's spending, divided by what paid for it.
type Split struct {
	// OrdinaryMinor is spending measured against the budget in the usual way.
	OrdinaryMinor int64
	// FundedMinor is spending a goal paid for.
	FundedMinor int64
	// TotalMinor is both — what actually left the account.
	TotalMinor int64
	// Goals are the goal IDs that funded any of it, sorted, so a surface can name
	// them rather than saying an unattributed "a goal".
	Goals []string
}

// HasFunded reports whether any of the period's spending was goal-funded.
func (s Split) HasFunded() bool { return s.FundedMinor > 0 }

// OverBy reports how far past a limit the ORDINARY spending went, and whether
// the goal-funded part is what would otherwise have tipped it over.
//
// The second return is the sentence worth showing: "you would be $120 over, but
// $340 of this came from your holiday fund" is a completely different message
// from "$120 over", and it is the difference between a budget that understands
// the plan and one that only counts.
func (s Split) OverBy(limitMinor int64) (overMinor int64, rescuedByGoal bool) {
	overMinor, rescuedByGoal, _ = s.OverByChecked(limitMinor)
	return overMinor, rescuedByGoal
}

// OverByChecked reports overflow while measuring ordinary and total overspend.
func (s Split) OverByChecked(limitMinor int64) (overMinor int64, rescuedByGoal bool, err error) {
	if limitMinor <= 0 {
		return 0, false, nil
	}
	ordinaryOver, err := money.Sub(s.OrdinaryMinor, limitMinor)
	if err != nil {
		return 0, false, fmt.Errorf("calculate ordinary overspend: %w", err)
	}
	totalOver, err := money.Sub(s.TotalMinor, limitMinor)
	if err != nil {
		return 0, false, fmt.Errorf("calculate total overspend: %w", err)
	}
	if ordinaryOver < 0 {
		ordinaryOver = 0
	}
	return ordinaryOver, totalOver > 0 && ordinaryOver == 0, nil
}

// Summarize splits expense in [start, end) matching a category set.
//
// categoryIDs may be empty, which matches every category — the caller decides
// scope; this package only decides what paid.
func Summarize(txns []domain.Transaction, categoryIDs map[string]bool,
	start, end time.Time, rates currency.Rates) Split {
	split, _ := SummarizeChecked(txns, categoryIDs, start, end, rates)
	return split
}

// SummarizeChecked splits matching spend and reports arithmetic overflow.
func SummarizeChecked(txns []domain.Transaction, categoryIDs map[string]bool,
	start, end time.Time, rates currency.Rates) (Split, error) {

	var s Split
	goals := map[string]bool{}
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		if len(categoryIDs) > 0 && !categoryIDs[t.CategoryID] {
			continue
		}
		amt, err := rates.ToBase(t.Amount)
		if err != nil {
			continue
		}
		v, err := money.Abs(amt.Amount)
		if err != nil {
			return Split{}, fmt.Errorf("absolute transaction %q: %w", t.ID, err)
		}
		s.TotalMinor, err = money.Add(s.TotalMinor, v)
		if err != nil {
			return Split{}, fmt.Errorf("sum goal-funded spending: %w", err)
		}
		if id := strings.TrimSpace(t.GoalID); id != "" {
			s.FundedMinor, err = money.Add(s.FundedMinor, v)
			if err != nil {
				return Split{}, fmt.Errorf("sum goal-funded spending: %w", err)
			}
			goals[id] = true
			continue
		}
		s.OrdinaryMinor, err = money.Add(s.OrdinaryMinor, v)
		if err != nil {
			return Split{}, fmt.Errorf("sum ordinary spending: %w", err)
		}
	}
	for id := range goals {
		s.Goals = append(s.Goals, id)
	}
	sort.Strings(s.Goals)
	return s, nil
}

// Drawdown is what a goal has had spent against it.
type Drawdown struct {
	GoalID string
	// SpentMinor is what has been drawn out for purchases linked to this goal.
	SpentMinor int64
	// Count is how many purchases drew on it.
	Count int
}

// ByGoal totals the spending linked to each goal over a window.
//
// Used to reduce a goal's available balance: money saved for a thing and then
// spent on that thing is no longer saved, and a goal that still shows the full
// amount after the purchase is a goal lying about its own progress.
func ByGoal(txns []domain.Transaction, start, end time.Time, rates currency.Rates) []Drawdown {
	drawdowns, _ := ByGoalChecked(txns, start, end, rates)
	return drawdowns
}

// ByGoalChecked totals goal-linked spending and reports arithmetic overflow.
func ByGoalChecked(txns []domain.Transaction, start, end time.Time, rates currency.Rates) ([]Drawdown, error) {
	byID := map[string]*Drawdown{}
	for _, t := range txns {
		id := strings.TrimSpace(t.GoalID)
		if id == "" || !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		amt, err := rates.ToBase(t.Amount)
		if err != nil {
			continue
		}
		v, err := money.Abs(amt.Amount)
		if err != nil {
			return nil, fmt.Errorf("absolute transaction %q: %w", t.ID, err)
		}
		d, ok := byID[id]
		if !ok {
			d = &Drawdown{GoalID: id}
			byID[id] = d
		}
		d.SpentMinor, err = money.Add(d.SpentMinor, v)
		if err != nil {
			return nil, fmt.Errorf("sum drawdown for goal %q: %w", id, err)
		}
		d.Count++
	}
	out := make([]Drawdown, 0, len(byID))
	for _, d := range byID {
		out = append(out, *d)
	}
	// Largest drawdown first, ties by ID, so the list is stable and the goal that
	// lost the most reads first.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SpentMinor != out[j].SpentMinor {
			return out[i].SpentMinor > out[j].SpentMinor
		}
		return out[i].GoalID < out[j].GoalID
	})
	return out, nil
}

// RemainingMinor is a goal's balance after the purchases it funded.
//
// Never negative: a goal cannot have less than nothing set aside, and a negative
// balance would read as a debt the household does not owe. Spending more than a
// goal held is a real situation — it means the difference came from somewhere
// else — and the honest report is "nothing left", with the overspend visible in
// the budget where it actually landed.
func RemainingMinor(savedMinor int64, d Drawdown) int64 {
	left, err := RemainingMinorChecked(savedMinor, d)
	if err != nil {
		return 0
	}
	return left
}

// RemainingMinorChecked subtracts a drawdown and reports arithmetic overflow.
func RemainingMinorChecked(savedMinor int64, d Drawdown) (int64, error) {
	left, err := money.Sub(savedMinor, d.SpentMinor)
	if err != nil {
		return 0, fmt.Errorf("subtract goal drawdown: %w", err)
	}
	if left < 0 {
		return 0, nil
	}
	return left, nil
}
