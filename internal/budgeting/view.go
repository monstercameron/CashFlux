// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/periodage"
)

// ViewPeriod holds the resolved dates shared by all steps of a budget view.
type ViewPeriod struct {
	Anchor, Now, PayCycleAnchor time.Time
	WeekStart                   time.Weekday
}

// ResolveViewPeriod anchors a live window to now and any other window to its start.
func ResolveViewPeriod(start, end, now time.Time, weekStart time.Weekday, payCycleAnchor time.Time) ViewPeriod {
	anchor := start
	if !now.Before(start) && now.Before(end) {
		anchor = now
	}
	return ViewPeriod{Anchor: anchor, Now: now, WeekStart: weekStart, PayCycleAnchor: payCycleAnchor}
}

// ViewInput contains the immutable ledger inputs for evaluating one budget.
type ViewInput struct {
	Period                  ViewPeriod
	Transactions            []domain.Transaction
	Categories              []domain.Category
	Recurring               []domain.Recurring
	Rates                   currency.Rates
	Base                    string
	PoolRollover, ShowPrior bool
}

// ViewIssue identifies a failed step without hiding the successfully evaluated steps.
type ViewIssue struct {
	Operation, BudgetID, Field string
	Err                        error
}

// ViewBudget holds unformatted figures and their provenance for a budget card.
type ViewBudget struct {
	Budget                            domain.Budget
	Start, End                        time.Time
	Status                            Status
	Valid                             bool
	Covers                            map[string]bool
	Carry                             money.Money
	HasCarry                          bool
	CarryDelta, Boost, PooledRollover int64
	Covered, Early                    bool
	Committed                         CommittedResult
	LandingItems                      []CommittedItem
	Prior                             Status
	HasPrior                          bool
	PriorSpentBase, PriorLimitBase    int64
	PaceOver                          money.Money
	ProratedRest                      int64
	Issues                            []ViewIssue
}

func (v *ViewBudget) issue(operation string, err error) {
	v.Issues = append(v.Issues, ViewIssue{Operation: operation, BudgetID: v.Budget.ID, Err: err})
}

// EvaluateViewBudget resolves carry, boosts, spending, smoothing, commitments and prior-period figures.
// The stored budget is never modified; Status.Budget carries the effective evaluation cap.
func EvaluateViewBudget(b domain.Budget, in ViewInput) ViewBudget {
	p := in.Period
	start, end := PeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
	v := ViewBudget{Budget: b, Start: start, End: end, Covers: categorytree.DescendantsOfAll(in.Categories, b.TrackedCategoryIDs())}
	v.Covered = !b.CoveredAt.IsZero() && !b.CoveredAt.Before(start) && b.CoveredAt.Before(end)
	eval := v.effectiveBudget(in)
	st, err := EvaluateRollup(eval, in.Transactions, start, end, in.Rates, DefaultNearThreshold, v.Covers)
	if err != nil {
		v.issue("evaluate budget", err)
		return v
	}
	offset, items := SmoothingLandingItems(b, in.Recurring, in.Transactions, start, end)
	if offset > 0 {
		st = ApplySmoothingOffset(st, offset, DefaultNearThreshold)
	}
	v.Status, v.Valid, v.LandingItems = st, true, items
	v.Committed = CommittedResult{Committed: money.Zero(st.Spent.Currency), Free: st.Remaining}
	if st.Remaining.IsPositive() {
		v.Committed, err = CommittedWithRates(b, in.Recurring, in.Transactions, st.Remaining, start, end, in.Rates)
		if err != nil {
			v.issue("calculate committed budget", err)
		}
	}
	v.Early = periodage.Of(start, end, p.Now).Early()
	if in.ShowPrior || v.Early {
		v.previousPeriod(in)
	}
	pace := ProjectPace(st, start, end, p.Now)
	if !pace.OnTrack && pace.Elapsed > 0 && pace.Elapsed < 1 && st.State != StateOver {
		v.PaceOver = pace.OverBy
	}
	if st.Remaining.Amount > 0 {
		days, left := int(end.Sub(start).Hours()/24), int(end.Sub(p.Now).Hours()/24)
		if left > 0 && left < days {
			if amount := st.Remaining.Amount * int64(left) / int64(days); amount > 0 && amount < st.Remaining.Amount {
				v.ProratedRest = amount
			}
		}
	}
	return v
}

func (v *ViewBudget) effectiveBudget(in ViewInput) domain.Budget {
	b, p := v.Budget, in.Period
	eval := b
	if b.Rollover || in.PoolRollover {
		start, end := PreviousPeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
		prev, err := EvaluateRollup(b, in.Transactions, start, end, in.Rates, DefaultNearThreshold, v.Covers)
		switch {
		case b.Rollover && err != nil:
			v.issue("evaluate prior rollover period", err)
		case b.Rollover:
			effective, err := CappedCarryover(prev.Remaining, b.Limit, b.RolloverCapPeriods)
			if err != nil {
				v.issue("cap budget rollover", err)
			} else {
				eval.Limit = effective
				v.CarryDelta = effective.Amount - b.Limit.Amount
			}
			v.Carry, v.HasCarry = prev.Remaining, true
		case err != nil:
			v.issue("evaluate pooled rollover", err)
		case prev.Remaining.Amount > 0:
			amount, err := currency.ConvertBetween(prev.Remaining.Amount, prev.Remaining.Currency, in.Base, in.Rates)
			if err != nil {
				v.issue("convert pooled rollover", err)
			} else {
				v.PooledRollover = amount
			}
		}
	}
	v.Boost = eval.PeriodBoosts[PeriodKeyWithWeekStart(b.Period, p.Anchor, p.PayCycleAnchor, p.WeekStart)]
	if v.Boost != 0 {
		eval.Limit = money.New(eval.Limit.Amount+v.Boost, eval.Limit.Currency)
	}
	// Preserve the effective cap even when current-period evaluation subsequently fails.
	v.Status.Limit = eval.Limit
	return eval
}

func (v *ViewBudget) previousPeriod(in ViewInput) {
	p := in.Period
	start, end := PreviousPeriodRangeAnchored(v.Budget.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
	prev, err := EvaluateRollup(v.Budget, in.Transactions, start, end, in.Rates, DefaultNearThreshold, v.Covers)
	if err != nil {
		v.issue("evaluate prior budget spending", err)
		return
	}
	v.Prior, v.HasPrior = prev, true
	v.PriorSpentBase, err = currency.ConvertBetween(prev.Spent.Amount, prev.Spent.Currency, in.Base, in.Rates)
	if err != nil {
		v.issue("convert previous budget spending", err)
	}
	v.PriorLimitBase, err = currency.ConvertBetween(prev.Limit.Amount, prev.Limit.Currency, in.Base, in.Rates)
	if err != nil {
		v.issue("convert previous budget limit", err)
	}
}

// ViewBudgets selects member-owned and shared budgets without changing store order.
func ViewBudgets(all []domain.Budget, memberID string) []domain.Budget {
	selected := make([]domain.Budget, 0, len(all))
	for _, b := range all {
		if memberID == "" || b.OwnerID == memberID || b.OwnerID == domain.GroupOwnerID {
			selected = append(selected, b)
		}
	}
	return selected
}

// NeedsViewAttention is the shared over-or-near predicate for the count and list filter.
func NeedsViewAttention(status Status) bool {
	return status.State == StateOver || status.State == StateNear
}

// ViewAttention selects the exact set counted by the attention chip, preserving order.
func ViewAttention(statuses []Status) []Status {
	selected := make([]Status, 0, len(statuses))
	for _, status := range statuses {
		if NeedsViewAttention(status) {
			selected = append(selected, status)
		}
	}
	return selected
}

// SortViewStatuses applies stable health-first ordering, then descending percent used.
// A projected overspend shares the near-limit rank but is not added to ViewAttention.
func SortViewStatuses(statuses []Status, paceOver map[string]bool) {
	rank := func(s Status) int {
		if s.State == StateOver {
			return 0
		}
		if s.State == StateNear || paceOver[s.Budget.ID] {
			return 1
		}
		return 2
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		a, b := rank(statuses[i]), rank(statuses[j])
		if a != b {
			return a < b
		}
		return statuses[i].Percent > statuses[j].Percent
	})
}

// SummarizeViewStatuses converts complete statuses to base before the checked roll-up.
// An unconvertible budget is omitted as a whole, with a diagnostic for the caller.
func SummarizeViewStatuses(statuses []Status, base string, rates currency.Rates) (RollupSummary, []ViewIssue, error) {
	converted := make([]Status, 0, len(statuses))
	var issues []ViewIssue
	for _, st := range statuses {
		next := st
		for label, value := range map[string]*money.Money{"spent": &next.Spent, "limit": &next.Limit, "remaining": &next.Remaining} {
			amount, err := currency.ConvertBetween(value.Amount, value.Currency, base, rates)
			if err != nil {
				issues = append(issues, ViewIssue{Operation: "convert budget rollup", BudgetID: st.Budget.ID, Field: label, Err: err})
				next.Budget.ID = ""
				break
			}
			*value = money.New(amount, base)
		}
		if next.Budget.ID != "" {
			converted = append(converted, next)
		}
	}
	summary, err := SummarizeRollupChecked(converted)
	if err != nil {
		return RollupSummary{}, issues, fmt.Errorf("summarize budget rollup: %w", err)
	}
	return summary, issues, nil
}

// ViewIncomeBasis is the saved or draft monthly income configuration.
type ViewIncomeBasis struct {
	Mode               string
	PaycheckMin, Fixed int64
	CategoryIDs        []string
	AverageMonths      int
}

// ResolveViewIncome uses complete months before the view anchor, preserving legacy precedence for an unset mode.
func ResolveViewIncome(basis ViewIncomeBasis, txns []domain.Transaction, categories []domain.Category, p ViewPeriod, base string, rates currency.Rates) (int64, error) {
	monthStart, _ := PeriodRange(domain.PeriodMonthly, p.Anchor, p.WeekStart)
	switch basis.Mode {
	case IncomeModeAll, IncomeModePaychecks, IncomeModeFixed, IncomeModeCategories:
		sources := NewIncomeSources(categories, basis.CategoryIDs)
		return AveragedIncomeChecked(basis.Mode, basis.PaycheckMin, basis.Fixed, sources, txns, monthStart, basis.AverageMonths, base, rates)
	default:
		return IncomeForBudgetsChecked(basis.Fixed, txns, dateutil.AddMonths(monthStart, -1), monthStart, base, rates)
	}
}
