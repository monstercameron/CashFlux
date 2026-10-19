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
	now = dateutil.WallClockUTC(now)
	anchor := start
	if !now.Before(start) && now.Before(end) {
		anchor = now
	}
	return ViewPeriod{Anchor: anchor, Now: now, WeekStart: weekStart, PayCycleAnchor: payCycleAnchor}
}

// ViewInput contains the immutable ledger inputs for evaluating one budget.
type ViewInput struct {
	// Ledger is an explicit scope-local refund read model. When nil, the legacy
	// transaction/netting adapter is used for compatibility with older callers.
	Ledger                  *NettedLedger
	Period                  ViewPeriod
	Transactions            []domain.Transaction
	Categories              []domain.Category
	Recurring               []domain.Recurring
	Rates                   currency.Rates
	Base                    string
	PoolRollover, ShowPrior bool
}

// ViewIssue identifies a failed or informational step without hiding successfully evaluated figures.
type ViewIssue struct {
	Operation, BudgetID, Field string
	Kind, Message              string
	Err                        error
}

// ViewIssueRolloverStartAssumed identifies the compatibility fallback for
// legacy rollover budgets without a recorded first period.
const ViewIssueRolloverStartAssumed = "RolloverStartAssumed"

// ViewBudget holds unformatted figures and their provenance for a budget card.
type ViewBudget struct {
	CommitmentError                   *BudgetEvaluationError
	Error                             *BudgetEvaluationError
	PeriodStatus                      PeriodStatus
	Projection                        Pace
	FreeAfterCommitted                money.Money
	Rollover                          RolloverView
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
	RestPerDay                        int64
	Issues                            []ViewIssue
}

func (v *ViewBudget) issue(operation string, err error) {
	v.Issues = append(v.Issues, ViewIssue{Operation: operation, BudgetID: v.Budget.ID, Err: err})
}

func (v *ViewBudget) fail(operation string, err error) {
	v.Error = &BudgetEvaluationError{BudgetID: v.Budget.ID, Operation: operation, Err: err}
	v.Valid = false
	v.Status.Error = v.Error
	v.Status.State = StateUnavailable
	v.issue(operation, v.Error)
	v.Status.Issues = DeduplicateViewIssues(v.Issues)
}

// EvaluateViewBudget resolves carry, boosts, spending, smoothing, commitments and prior-period figures.
// The stored budget is never modified; Status.Limit carries the effective cap
// while Status.Budget retains the standing plan for editing and annual totals.
func EvaluateViewBudget(b domain.Budget, in ViewInput) ViewBudget {
	ledger := NewNettedLedgerWithLinks(in.Transactions, nil)
	if in.Ledger != nil {
		ledger = *in.Ledger
	} else {
		ledger = NewNettedLedger(in.Transactions)
	}
	in.Ledger = &ledger
	in.Transactions = ledger.transactions
	b = b.WithBaseCurrency(in.Base)
	p := in.Period
	p.Now = dateutil.WallClockUTC(p.Now)
	in.Period = p
	start, end := PeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
	calendar := periodStatus(start, end, p.Now, periodage.Of(start, end, p.Now).Early())
	v := ViewBudget{Budget: b, Start: start, End: end, PeriodStatus: calendar, Status: Status{Budget: b, Limit: b.Limit, PeriodStatus: calendar}, Covers: categorytree.DescendantsOfAll(in.Categories, b.TrackedCategoryIDs())}
	v.Covered = !b.CoveredAt.IsZero() && !b.CoveredAt.Before(start) && b.CoveredAt.Before(end)
	eval := v.effectiveBudget(in)
	if v.Error != nil {
		return v
	}
	if _, err := in.Rates.Convert(eval.Limit, in.Base); err != nil {
		v.fail("convert budget limit", err)
		return v
	}
	st, err := evaluateNettedWith(eval, in.Transactions, start, end, in.Rates, DefaultNearThreshold, func(id string) bool { return eval.TracksCategory(id) || v.Covers[id] })
	if err != nil {
		v.fail("evaluate budget", err)
		return v
	}
	smoothingBudget := b
	smoothingBudget.CategoryIDs = ScopeOf(b, v.Covers).CategoryIDs
	offset, items, err := SmoothingLandingItemsWithRates(smoothingBudget, in.Recurring, in.Transactions, start, end, in.Rates)
	if err != nil {
		v.fail("calculate smoothed budget spend", err)
		return v
	}
	if offset > 0 {
		st, err = ApplySmoothingOffsetChecked(st, offset, DefaultNearThreshold)
		if err != nil {
			v.fail("offset smoothed budget spend", err)
			return v
		}
	}
	v.Status, v.Valid, v.LandingItems = st, true, items
	// Preserve the standing configuration for editing and annual planning; the
	// evaluated cap lives only in Status.Limit.
	v.Status.Budget = b
	commitBudget := b
	commitBudget.CategoryIDs = ScopeOf(b, v.Covers).CategoryIDs
	v.Committed, err = CommittedWithRates(commitBudget, in.Recurring, in.Transactions, st.Remaining, start, end, in.Rates)
	if err == nil {
		v.FreeAfterCommitted, err = st.Remaining.Sub(v.Committed.Required)
	}
	if err != nil {
		v.CommitmentError = &BudgetEvaluationError{BudgetID: b.ID, Operation: "calculate committed budget", Err: err}
		v.issue(v.CommitmentError.Operation, v.CommitmentError)
		v.FreeAfterCommitted = money.Money{}
	}
	v.Early = periodage.Of(start, end, p.Now).Early()
	v.PeriodStatus = periodStatus(start, end, p.Now, v.Early)
	if v.PeriodStatus != PeriodEarly {
		v.Early = false
	}
	if in.ShowPrior || v.Early {
		v.previousPeriod(in)
	}
	postedRecurring, matchErr := matchedRecurringPosted(eval, in.Recurring, in.Transactions, start, end, st.Spent.Currency, in.Rates)
	if matchErr != nil {
		v.issue("match posted recurring spending", matchErr)
	}
	scheduledRecurring := v.Committed.Required
	if scheduledRecurring.Currency == "" {
		scheduledRecurring = money.Zero(st.Spent.Currency)
	}
	var pace Pace
	switch {
	case matchErr != nil:
		pace = Pace{Error: matchErr}
	case v.CommitmentError != nil:
		pace = Pace{Error: v.CommitmentError}
	default:
		pace = ProjectBudgetPace(st, postedRecurring, scheduledRecurring, start, end, p.Now)
	}
	v.Projection = pace
	if pace.Error != nil {
		v.issue("project budget pace", pace.Error)
	}
	if !pace.PeriodEnded && !pace.OnTrack && pace.Elapsed > 0 && pace.Elapsed < 1 && st.State != StateOver {
		v.PaceOver = pace.OverBy
	}
	if st.Remaining.Amount > 0 {
		days, left := int(end.Sub(start).Hours()/24), int(end.Sub(p.Now).Hours()/24)
		if left > 0 && left < days {
			v.RestPerDay, err = money.MulDiv(st.Remaining.Amount, 1, int64(left), money.RoundTowardZero)
			if err != nil {
				v.issue("calculate remaining budget per day", err)
			}
			amount, err := money.MulDiv(st.Remaining.Amount, int64(left), int64(days), money.RoundTowardZero)
			if err != nil {
				v.issue("prorate budget remainder", err)
			} else if amount > 0 && amount < st.Remaining.Amount {
				v.ProratedRest = amount
			}
		}
	}
	v.Status.CommitmentError = v.CommitmentError
	v.Status.FreeAfterCommitted = v.FreeAfterCommitted
	v.Status.PeriodStatus = v.PeriodStatus
	v.Status.Projection = v.Projection
	v.Status.Committed = v.Committed
	v.Status.Rollover = v.Rollover
	v.Issues = DeduplicateViewIssues(v.Issues)
	v.Status.Issues = v.Issues
	return v
}

func matchedRecurringPosted(budget domain.Budget, recurrings []domain.Recurring, posted []domain.Transaction, start, end time.Time, targetCurrency string, rates currency.Rates) (money.Money, error) {
	total := money.Zero(targetCurrency)
	for _, recurring := range recurrings {
		if !recurring.Active() || !recurring.Amount.IsNegative() || recurring.CategoryID == "" || !budget.TracksCategory(recurring.CategoryID) || recurring.Smooths() {
			continue
		}
		magnitude, err := money.Abs(recurring.Amount.Amount)
		if err != nil {
			return money.Money{}, fmt.Errorf("recurring %q magnitude: %w", recurring.ID, err)
		}
		contributions, err := postedContributions(budget, posted, recurring, magnitude, start, end, rates)
		if err != nil {
			return money.Money{}, err
		}
		for _, contribution := range contributions {
			converted, err := rates.Convert(contribution, targetCurrency)
			if err != nil {
				return money.Money{}, fmt.Errorf("convert posted recurring %q: %w", recurring.ID, err)
			}
			total, err = total.Add(converted)
			if err != nil {
				return money.Money{}, fmt.Errorf("sum posted recurring spend: %w", err)
			}
		}
	}
	return total, nil
}

func (v *ViewBudget) effectiveBudget(in ViewInput) domain.Budget {
	b, p := v.Budget, in.Period
	eval := b
	base, err := BudgetBaseAt(b, v.Start)
	if err != nil {
		v.fail("resolve budget base", err)
		return eval
	}
	eval.Limit = base
	if b.Rollover {
		chain, err := EvaluateRolloverView(b, in, v.Covers)
		if err != nil {
			v.fail("evaluate prior rollover period", err)
			return eval
		}
		v.Rollover = chain
		if chain.StartAssumed {
			v.Issues = append(v.Issues, ViewIssue{
				Operation: "evaluate prior rollover period",
				BudgetID:  b.ID,
				Kind:      ViewIssueRolloverStartAssumed,
				Message:   fmt.Sprintf("Carry is counted from %s. Set when this budget started rolling over to include earlier months.", chain.Start.Format("January 2006")),
			})
		}
		eval.Limit = chain.Current.EffectiveCap
		v.Carry, v.HasCarry = chain.Current.CarryIn, len(chain.Prior) > 0
		v.CarryDelta = v.Carry.Amount
	} else if in.PoolRollover {
		start, end := PreviousPeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
		prev, err := evaluateNettedWith(b, in.Transactions, start, end, in.Rates, DefaultNearThreshold, func(id string) bool { return b.TracksCategory(id) || v.Covers[id] })
		switch {
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
	if v.Boost != 0 && !b.Rollover {
		var err error
		eval.Limit, err = eval.Limit.Add(money.New(v.Boost, eval.Limit.Currency))
		if err != nil {
			v.fail("apply budget period boost", err)
			return eval
		}
	}
	// Preserve the effective cap even when current-period evaluation subsequently fails.
	v.Status.Limit = eval.Limit
	return eval
}

func (v *ViewBudget) previousPeriod(in ViewInput) {
	p := in.Period
	start, end := PreviousPeriodRangeAnchored(v.Budget.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
	eval := v.Budget
	base, err := BudgetBaseAt(eval, start)
	if err != nil {
		v.issue("resolve previous budget base", err)
		return
	}
	eval.Limit, err = base.Add(money.New(eval.PeriodBoosts[PeriodKeyWithWeekStart(eval.Period, start, p.PayCycleAnchor, p.WeekStart)], base.Currency))
	if err != nil {
		v.issue("apply previous budget boost", err)
		return
	}
	if len(v.Rollover.Prior) > 0 {
		eval.Limit = v.Rollover.Prior[len(v.Rollover.Prior)-1].EffectiveCap
	}
	prev, err := evaluateNettedWith(eval, in.Transactions, start, end, in.Rates, DefaultNearThreshold, func(id string) bool { return eval.TracksCategory(id) || v.Covers[id] })
	if err != nil {
		v.issue("evaluate prior budget spending", err)
		return
	}
	smoothingBudget := eval
	smoothingBudget.CategoryIDs = ScopeOf(eval, v.Covers).CategoryIDs
	offset, _, err := SmoothingLandingItemsWithRates(smoothingBudget, in.Recurring, in.Transactions, start, end, in.Rates)
	if err == nil {
		prev, err = ApplySmoothingOffsetChecked(prev, offset, DefaultNearThreshold)
	}
	if err != nil {
		v.issue("calculate previous smoothed spending", err)
		return
	}
	spent, err := PeriodEquivalent(prev.Spent, eval.Period, domain.PeriodMonthly)
	if err == nil {
		v.PriorSpentBase, err = currency.ConvertBetween(spent.Amount, spent.Currency, in.Base, in.Rates)
	}
	if err != nil {
		v.issue("convert previous budget spending", err)
		return
	}
	limit, err := PeriodEquivalent(prev.Limit, eval.Period, domain.PeriodMonthly)
	if err == nil {
		v.PriorLimitBase, err = currency.ConvertBetween(limit.Amount, limit.Currency, in.Base, in.Rates)
	}
	if err != nil {
		v.issue("convert previous budget limit", err)
		return
	}
	v.Prior, v.HasPrior = prev, true
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
	return SummarizeViewStatusesForPeriod(statuses, domain.PeriodMonthly, base, rates)
}

// SummarizeViewStatusesForPeriod reduces budget amounts to one currency and
// cadence. A failed budget stays in the input, contributes no figures, and is
// counted once in Excluded; healthy budgets still produce a usable subtotal.
func SummarizeViewStatusesForPeriod(statuses []Status, period domain.Period, base string, rates currency.Rates) (RollupSummary, []ViewIssue, error) {
	converted := make([]Status, 0, len(statuses))
	var issues []ViewIssue
	excluded := 0
	for _, st := range statuses {
		if st.Error != nil {
			excluded++
			issues = append(issues, ViewIssue{BudgetID: st.Budget.ID, Operation: st.Error.Operation, Err: st.Error})
			continue
		}
		next := st
		b := st.Budget
		b.Limit = st.Limit
		normalized, err := NormalizeBudgets([]domain.Budget{b}, period, rates)
		if err != nil {
			excluded++
			issues = append(issues, normalized.Issues...)
			continue
		}
		next.Limit = normalized.Total
		next.Limit, err = rates.Convert(next.Limit, base)
		if err != nil {
			excluded++
			issues = append(issues, ViewIssue{Operation: "convert normalized budget limit", BudgetID: st.Budget.ID, Err: err})
			continue
		}
		next.Spent, err = PeriodEquivalent(st.Spent, st.Budget.Period, period)
		if err == nil {
			next.Spent, err = rates.Convert(next.Spent, base)
		}
		if err == nil {
			// Round each independent amount once; derive the remainder so the
			// normalized figures reconcile exactly even on a half-cent boundary.
			next.Remaining, err = next.Limit.Sub(next.Spent)
		}
		if err != nil {
			excluded++
			issues = append(issues, ViewIssue{Operation: "convert budget rollup", BudgetID: st.Budget.ID, Err: err})
			continue
		}
		converted = append(converted, next)
	}
	summary, err := SummarizeRollupChecked(converted)
	if err != nil {
		return RollupSummary{}, issues, fmt.Errorf("summarize budget rollup: %w", err)
	}
	summary.Excluded = excluded
	return summary, DeduplicateViewIssues(issues), nil
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
