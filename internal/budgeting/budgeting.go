// SPDX-License-Identifier: MIT

// Package budgeting computes spending against budgets: how much has been spent
// in a budget's category over a period (scope-aware), how much remains, and
// whether the budget is on track, near its limit, or over.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// DefaultNearThreshold is the fraction of the limit at which a budget is "near"
// (but not yet over).
const DefaultNearThreshold = 0.8

// State summarizes how a budget is tracking.
type State string

const (
	StateOK   State = "ok"
	StateNear State = "near"
	StateOver State = "over"
)

// Status is the evaluated state of a budget for a period.
type Status struct {
	Budget    domain.Budget
	Spent     money.Money
	Remaining money.Money
	// Limit is the budget's cap in the evaluation currency (R-LEAK). Callers used
	// to re-derive it as Spent+Remaining, which is arithmetically right and wrong
	// as a practice: it puts domain arithmetic in view code and it would break
	// silently the day Remaining is ever clamped. Reading a field cannot.
	Limit   money.Money
	Percent int // spent as a percent of the limit (may exceed 100)
	State   State
}

// normalizedLimit returns the budget's limit, defaulting an empty currency to
// the rate table's base currency.
func normalizedLimit(budget domain.Budget, rates currency.Rates) money.Money {
	limit := budget.Limit
	if limit.Currency == "" {
		return money.New(limit.Amount, rates.Base)
	}
	return limit
}

// matchesScope reports whether a transaction is in scope for the budget
// independent of its category and owner: it must be an expense within
// [start, end). The per-category test and the individual-budget owner test are
// applied separately by spentCovered — the owner test is per split LINE (XC10),
// so a shared transaction can still contribute the lines an individual budget
// owns even when a different member paid.
func matchesScope(budget domain.Budget, t domain.Transaction, start, end time.Time) bool {
	if !inScopeDirection(budget, t) {
		return false
	}
	return dateutil.InRange(t.Date, start, end)
}

// inScopeDirection decides whether a transaction is the KIND of movement a
// budget measures.
//
// A spending budget counts expenses: negative, and not a transfer. Excluding
// transfers is right there — money moved between your own accounts was not
// spent, and counting it would double-report it against the category.
//
// A saving budget counts contributions, which are also outflows — money leaving
// the spending pool — but it must INCLUDE transfers, because the ordinary way to
// put $500 into investments is a transfer from checking to the brokerage. That
// is the answer to C539, and it falls out of the direction rather than needing a
// separate flag: a transfer is not spending, but it IS saving. Only the leg that
// leaves counts, so the matching pair cannot be counted twice.
func inScopeDirection(budget domain.Budget, t domain.Transaction) bool {
	if budget.IsSaving() {
		return t.Amount.IsNegative()
	}
	return t.IsExpense()
}

// ownsScope reports whether spend attributed to member counts against the budget
// under its scope: a household budget counts everyone; an individual budget
// counts only its owner (XC10 resolves member per split line before calling this).
func ownsScope(budget domain.Budget, member string) bool {
	return budget.Scope != domain.ScopeIndividual || member == budget.OwnerID
}

// spentCovered sums spend against the budget for transactions whose category
// passes covers, in the budget's limit currency. For a split transaction (C58)
// each split line is attributed to its own category — only the lines whose
// category passes covers count, and never the whole-transaction category — so a
// grocery receipt split into produce/household lands in the right budgets without
// double-counting.
func spentCovered(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, covers func(string) bool) (money.Money, error) {
	limit := normalizedLimit(budget, rates)
	all = nettedForSpending(all) // XC2: fold refund-pair netting into read-model spend
	total := money.Zero(limit.Currency)
	add := func(amt money.Money) error {
		conv, err := rates.Convert(amt.Abs(), limit.Currency)
		if err != nil {
			return err
		}
		total, err = total.Add(conv)
		return err
	}
	for _, t := range all {
		// TXC-1: a transaction excluded from reports doesn't count toward budget
		// spend either (reimbursements, cash-back, one-offs) — but still affects the
		// account balance elsewhere.
		if !t.CountsInReports() {
			continue
		}
		if !matchesScope(budget, t, start, end) {
			continue
		}
		// Cross-category tag tracking: a transaction carrying one of the budget's tracked
		// tags counts in FULL and counts ONCE — regardless of how many of those tags it
		// carries, and taking priority over category/split matching so a charge that matches
		// both a tracked tag and a tracked category is never double-counted.
		if budget.TracksAnyTag(t.Tags) {
			if !ownsScope(budget, t.MemberID) {
				continue
			}
			if err := add(t.Amount); err != nil {
				return money.Money{}, err
			}
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				if !covers(s.CategoryID) {
					continue
				}
				// XC10: attribute the line to its effective owner (the line's own
				// MemberID, or the transaction's payer when the line carries none).
				// An individual budget counts only the lines it owns — so a shared
				// receipt paid by A can still feed B's personal budget line.
				if !ownsScope(budget, s.LineOwner(t.MemberID)) {
					continue
				}
				if err := add(s.Amount); err != nil {
					return money.Money{}, err
				}
			}
			continue
		}
		if !ownsScope(budget, t.MemberID) {
			continue
		}
		if !covers(t.CategoryID) {
			continue
		}
		if err := add(t.Amount); err != nil {
			return money.Money{}, err
		}
	}
	return total, nil
}

// Spent returns the total spent against a budget within [start, end), in the
// budget's limit currency (the budget's tracked categories only).
func Spent(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates) (money.Money, error) {
	return spentCovered(budget, all, start, end, rates, budget.TracksCategory)
}

// evaluateWith builds the Status using the given category-cover predicate.
func evaluateWith(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64, covers func(string) bool) (Status, error) {
	limit := normalizedLimit(budget, rates)
	spent, err := spentCovered(budget, all, start, end, rates, covers)
	if err != nil {
		return Status{}, err
	}
	remaining, err := limit.Sub(spent)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Budget:    budget,
		Spent:     spent,
		Remaining: remaining,
		Limit:     limit,
		Percent:   percent(spent, limit),
		State:     classifyDirected(budget.Direction, spent, limit, nearThreshold),
	}, nil
}

// classifyDirected applies the budget's direction to the raw figures.
//
// Spending and saving are the same arithmetic read two opposite ways. Under a
// spending cap, being under the number is healthy and passing it is the failure.
// Under a saving target the reading inverts: reaching the number is the SUCCESS,
// and the state worth flagging is falling short of it. Everything downstream —
// the near/over counts, the issues rail, the bar's tone — keys off State, so
// inverting here is what makes one flag mean "needs attention" in both worlds
// instead of meaning two different things.
func classifyDirected(dir domain.BudgetDirection, contributed, target money.Money, nearThreshold float64) State {
	if !dir.IsSaving() {
		return classify(contributed, target, nearThreshold)
	}
	if target.Amount <= 0 {
		return StateOK // nothing asked for, nothing to fall short of
	}
	switch {
	case contributed.Amount >= target.Amount:
		return StateOK // funded — the goal, not a breach
	case float64(contributed.Amount) >= nearThreshold*float64(target.Amount):
		return StateNear // nearly there
	}
	// Short of the target with the period's money already committed elsewhere is
	// the savings equivalent of overspending: it is the state that needs a
	// decision, so it carries the same flag the rest of the app already counts.
	return StateOver
}

// Evaluate returns the full Status for a budget over [start, end), counting only
// the budget's own category. nearThreshold is the fraction of the limit
// considered "near"; pass DefaultNearThreshold for the standard 80%.
func Evaluate(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64) (Status, error) {
	return evaluateWith(budget, all, start, end, rates, nearThreshold, budget.TracksCategory)
}

// EvaluateRollup is like Evaluate but the budget also counts spend in any
// category in covers — typically the budget's category plus its descendants
// (from categorytree.Descendants) — so a parent-category budget includes its
// sub-categories' spend (D5). An empty covers falls back to the budget's own
// category.
func EvaluateRollup(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64, covers map[string]bool) (Status, error) {
	return evaluateWith(budget, all, start, end, rates, nearThreshold, func(id string) bool {
		return budget.TracksCategory(id) || covers[id]
	})
}

// epochMonday is the Monday of an arbitrary fixed week used to establish the
// fortnightly grid for biweekly periods. Any Monday will do; 2006-01-02 is the
// reference date in Go's time package (which is itself a Monday).
var epochMonday = time.Date(2006, time.January, 2, 0, 0, 0, 0, time.UTC)

// PeriodRange returns the half-open [start, end) range for the budget period of
// kind p that contains ref. weekStart sets the first day of the week for weekly
// and biweekly periods. An unknown period falls back to monthly.
func PeriodRange(p domain.Period, ref time.Time, weekStart time.Weekday) (start, end time.Time) {
	switch p {
	case domain.PeriodWeekly:
		start = dateutil.WeekStart(ref, weekStart)
		return start, start.AddDate(0, 0, 7)

	case domain.PeriodBiweekly:
		// Snap the reference date to the start of its 7-day week, then determine
		// which fortnight of the stable 14-day grid it falls in. The grid is
		// anchored to epochMonday, shifted by weekStart so that every biweekly
		// window boundary aligns with a week-start boundary.
		anchor := epochMonday
		if weekStart != time.Monday {
			// Shift the epoch anchor by the offset from Monday to weekStart.
			offset := int(weekStart) - int(time.Monday)
			if offset < 0 {
				offset += 7
			}
			anchor = anchor.AddDate(0, 0, offset)
		}
		// Use UTC-normalised day counts to avoid DST ambiguity at boundaries.
		refDay := ref.In(time.UTC).Truncate(24 * time.Hour)
		ancDay := anchor.In(time.UTC)
		diff := int(refDay.Sub(ancDay).Hours()) / 24
		if diff < 0 {
			// Go's integer division truncates toward zero; adjust for negative diff.
			diff = diff - 13
		}
		fortnight := (diff / 14) * 14
		start = ancDay.AddDate(0, 0, fortnight).In(ref.Location())
		return start, start.AddDate(0, 0, 14)

	case domain.PeriodSemimonthly:
		// First half: [1st, 16th); second half: [16th, 1st of next month).
		y, m, d := ref.Date()
		loc := ref.Location()
		if d <= 15 {
			start = time.Date(y, m, 1, 0, 0, 0, 0, loc)
			return start, time.Date(y, m, 16, 0, 0, 0, 0, loc)
		}
		start = time.Date(y, m, 16, 0, 0, 0, 0, loc)
		return start, time.Date(y, m+1, 1, 0, 0, 0, 0, loc)

	case domain.PeriodQuarterly:
		y, m, _ := ref.Date()
		qm := ((int(m)-1)/3)*3 + 1
		start = time.Date(y, time.Month(qm), 1, 0, 0, 0, 0, ref.Location())
		return start, dateutil.AddMonths(start, 3)

	case domain.PeriodYearly:
		y := ref.Year()
		start = time.Date(y, time.January, 1, 0, 0, 0, 0, ref.Location())
		return start, time.Date(y+1, time.January, 1, 0, 0, 0, 0, ref.Location())

	default:
		return dateutil.MonthRange(ref)
	}
}

// PeriodRangeAnchored is like PeriodRange but for PeriodBiweekly it snaps the
// 14-day grid to a user-supplied anchor date (a known payday) instead of the
// internal epoch. This aligns every-two-weeks budget windows to the user's
// actual pay cycle.
//
// For every period other than PeriodBiweekly the anchor is ignored and the call
// delegates to PeriodRange. For PeriodBiweekly, if anchor is the zero time the
// function likewise falls back to PeriodRange (preserving the default epoch
// behavior for users who have not configured a pay-cycle anchor).
//
// The biweekly math mirrors PeriodRange: UTC day-normalised counts avoid DST
// ambiguity at window boundaries.
func PeriodRangeAnchored(p domain.Period, ref time.Time, weekStart time.Weekday, anchor time.Time) (start, end time.Time) {
	if p != domain.PeriodBiweekly || anchor.IsZero() {
		return PeriodRange(p, ref, weekStart)
	}
	// Normalise both dates to UTC midnight so DST transitions inside a fortnight
	// never shift the boundary by an hour.
	refDay := ref.In(time.UTC).Truncate(24 * time.Hour)
	ancDay := anchor.In(time.UTC).Truncate(24 * time.Hour)
	diff := int(refDay.Sub(ancDay).Hours()) / 24
	if diff < 0 {
		// Integer division truncates toward zero for negative values; adjust so
		// the result floors to the start of the fortnight that contains ref.
		diff = diff - 13
	}
	fortnight := (diff / 14) * 14
	start = ancDay.AddDate(0, 0, fortnight).In(ref.Location())
	return start, start.AddDate(0, 0, 14)
}

// EvaluateAll evaluates a set of budgets over the same period.
func EvaluateAll(budgets []domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64) ([]Status, error) {
	out := make([]Status, 0, len(budgets))
	for _, b := range budgets {
		s, err := Evaluate(b, all, start, end, rates, nearThreshold)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func percent(spent, limit money.Money) int {
	if limit.Amount <= 0 {
		if spent.Amount > 0 {
			return 100
		}
		return 0
	}
	return int(spent.Amount * 100 / limit.Amount)
}

func classify(spent, limit money.Money, nearThreshold float64) State {
	if limit.Amount <= 0 {
		if spent.Amount > 0 {
			return StateOver
		}
		return StateOK
	}
	// Strictly past the limit is over; landing EXACTLY on the limit is "near"
	// (remaining is $0.00, nothing is overspent). The old >= classification
	// forced cover flows to move one cent MORE than the displayed shortfall to
	// clear the flag — the QA L1 "covered $49.64 by moving $49.65" discrepancy.
	if spent.Amount > limit.Amount {
		return StateOver
	}
	if float64(spent.Amount) >= nearThreshold*float64(limit.Amount) {
		return StateNear
	}
	return StateOK
}

// IsDuplicateBudget reports whether adding a budget for the given (categoryID,
// period, ownerID) triple would create a second live budget with the same scope.
// The "one budget per category per period per owner" rule prevents ambiguous
// spend attribution: two budgets competing for the same category + period + owner
// would both accrue the same transactions, making their totals misleading.
//
// It ignores the existing budget whose ID matches excludeID (pass "" to check
// against all). Pass the ID of the budget being edited to allow a save of its
// own unchanged triple.
func IsDuplicateBudget(existing []domain.Budget, categoryID, period, ownerID, excludeID string) bool {
	for _, b := range existing {
		if b.ID == excludeID {
			continue
		}
		if b.CategoryID == categoryID && string(b.Period) == period && b.OwnerID == ownerID {
			return true
		}
	}
	return false
}

// ─── R-LEAK: the rollup summary, out of view code ────────────────────────────

// RollupSummary is the household-level read over a set of budget statuses: how
// many are in trouble, and how the money splits between spending and saving.
type RollupSummary struct {
	// OverCount and NearCount are how many budgets are past or approaching their
	// number. They key off State, so a SAVING budget's "over" means it exceeded
	// its target — a success — exactly as classifyDirected intends.
	OverCount, NearCount int
	// SpentMinor and LimitMinor cover the SPENDING budgets only.
	SpentMinor, LimitMinor int64
	// OverMinor is how far past their caps the over-budget ones are, as a
	// positive magnitude.
	OverMinor int64
	// SavingsBudgetedMinor and SavingsMovedMinor cover the SAVING budgets.
	SavingsBudgetedMinor, SavingsMovedMinor int64
}

// SummarizeRollup reduces evaluated statuses to the household read (R-LEAK).
//
// It lives here rather than in the budgets screen because it is domain
// aggregation, not display: the saving-vs-spending split in particular is a
// judgement about what the figures MEAN, and it was previously made in view
// code where no test could reach it.
//
// A SAVING budget is a contribution target, not a spending cap, so it is kept
// out of the spending totals entirely (C538). Folding its limit into "budgeted"
// would claim the household plans to spend money it plans to set aside, and
// folding its contributions into "spent" would do the same to the other figure.
func SummarizeRollup(statuses []Status) RollupSummary {
	var s RollupSummary
	for _, st := range statuses {
		switch st.State {
		case StateOver:
			s.OverCount++
			if st.Remaining.IsNegative() {
				s.OverMinor += -st.Remaining.Amount
			}
		case StateNear:
			s.NearCount++
		}
		if st.Budget.IsSaving() {
			s.SavingsBudgetedMinor += st.Limit.Amount
			s.SavingsMovedMinor += st.Spent.Amount
			continue
		}
		s.SpentMinor += st.Spent.Amount
		s.LimitMinor += st.Limit.Amount
	}
	return s
}
