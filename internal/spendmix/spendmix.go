// SPDX-License-Identifier: MIT

// Package spendmix splits spending into what a household is COMMITTED to and
// what it actually chooses, and reports how far budgets landed from plan
// (FP-T3b).
//
// The reports already said what was spent and on what. Neither answers the
// question people are actually asking when they look at a bad month, which is
// "how much of this could I have done anything about". Rent and insurance are
// spending, but they are not decisions — and a $4,200 month that is 80%
// commitments is a completely different situation from the same $4,200 at 30%,
// while the category chart renders them identically.
//
// The classification is not invented here: `domain.CategoryClass` has always
// carried it (fixed / non-monthly / flex), and until now nothing read it outside
// the budgeting screens.
package spendmix

import (
	"math"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Mix is a period's spending split by how much choice it involved.
type Mix struct {
	// FixedMinor is spending on commitments — rent, insurance, loan payments.
	FixedMinor int64
	// NonMonthlyMinor is irregular spending the household smooths into a monthly
	// set-aside: an annual premium, a car service. Kept SEPARATE from both other
	// buckets, because it is neither a monthly commitment nor a free choice —
	// folding it into "fixed" would overstate how trapped the month was, and
	// folding it into "flex" would suggest it could simply be skipped.
	NonMonthlyMinor int64
	// FlexMinor is day-to-day discretionary spending.
	FlexMinor int64
	// TotalMinor is all three.
	TotalMinor int64
	// UncategorizedMinor is expense with no category, counted in the total but
	// attributed to none of the buckets.
	//
	// Reported separately rather than defaulted into flex. A category-less
	// transaction is unknown, and calling unknown spending "discretionary" is the
	// flattering guess — it tells the reader they had more room than they did.
	UncategorizedMinor int64
}

// CommittedMinor is fixed plus non-monthly: the part of a month that is spoken
// for before any choices are made.
func (m Mix) CommittedMinor() int64 { return m.FixedMinor + m.NonMonthlyMinor }

// FlexSharePct is the share of spending that was actually discretionary.
//
// Reports ok=false on a period with no spending, because "0% of nothing was
// discretionary" is a sentence about a month that did not happen.
func (m Mix) FlexSharePct() (float64, bool) {
	if m.TotalMinor <= 0 {
		return 0, false
	}
	return float64(m.FlexMinor) / float64(m.TotalMinor) * 100, true
}

// Summarize splits expense in [start, end) by category class.
func Summarize(txns []domain.Transaction, cats []domain.Category,
	start, end time.Time, rates currency.Rates) Mix {

	classOf := make(map[string]domain.CategoryClass, len(cats))
	for _, c := range cats {
		classOf[c.ID] = c.ClassOf()
	}
	var m Mix
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		amt, err := rates.ToBase(t.Amount)
		if err != nil {
			continue
		}
		v := amt.Amount
		if v < 0 {
			v = -v
		}
		m.TotalMinor += v
		if t.CategoryID == "" {
			m.UncategorizedMinor += v
			continue
		}
		switch classOf[t.CategoryID] {
		case domain.ClassFixed:
			m.FixedMinor += v
		case domain.ClassNonMonthly:
			m.NonMonthlyMinor += v
		default:
			m.FlexMinor += v
		}
	}
	return m
}

// Variance is one budget's distance from plan.
type Variance struct {
	BudgetID   string
	CategoryID string
	Label      string
	LimitMinor int64
	SpentMinor int64
	// DeltaMinor is spent minus limit: POSITIVE is over, negative is under.
	//
	// The sign convention is stated because it is the one people get backwards,
	// and because "over" is the direction that costs money — making it the
	// positive one keeps the biggest problems at the top of a descending sort
	// without a second rule.
	DeltaMinor int64
	// PctOfLimit is spending as a percentage of the limit, and Known is false for
	// a zero limit — a percentage of nothing is not a large percentage, it is not
	// a percentage.
	PctOfLimit float64
	Known      bool
}

// Over reports whether this budget was exceeded.
func (v Variance) Over() bool { return v.DeltaMinor > 0 }

// Report is the budget-variance view.
type Report struct {
	// Rows are sorted by how far from plan they landed, largest overspend first,
	// so the first row is the one worth reading.
	Rows []Variance
	// OverMinor and UnderMinor are the totals on each side, kept apart rather
	// than netted: $300 over on groceries and $300 under on travel is not a month
	// that went to plan, and a net zero would say it was.
	OverMinor, UnderMinor int64
	// PlannedMinor and ActualMinor are the totals across every budget with a
	// limit.
	PlannedMinor, ActualMinor int64
}

// NetMinor is what the netting would have said, offered explicitly for a caller
// that genuinely wants it rather than produced by default.
func (r Report) NetMinor() int64 { return r.ActualMinor - r.PlannedMinor }

// Compare builds the variance report from already-evaluated budget statuses.
//
// It takes the statuses rather than re-deriving spend so this view can never
// disagree with the budgets screen about what was spent — two independent
// computations of the same figure eventually differ, and the reader has no way
// to know which one is lying.
func Compare(limits, spends map[string]int64, labels map[string]string,
	categoryOf map[string]string) Report {

	var r Report
	for id, limit := range limits {
		if limit <= 0 {
			// A budget with no limit has nothing to be over or under. Skipped
			// rather than reported as a 0% variance, which would read as on-plan.
			continue
		}
		spent := spends[id]
		v := Variance{
			BudgetID: id, CategoryID: categoryOf[id], Label: labels[id],
			LimitMinor: limit, SpentMinor: spent, DeltaMinor: spent - limit,
			PctOfLimit: float64(spent) / float64(limit) * 100, Known: true,
		}
		r.Rows = append(r.Rows, v)
		r.PlannedMinor += limit
		r.ActualMinor += spent
		if v.DeltaMinor > 0 {
			r.OverMinor += v.DeltaMinor
		} else {
			r.UnderMinor += -v.DeltaMinor
		}
	}
	sort.SliceStable(r.Rows, func(i, j int) bool {
		if r.Rows[i].DeltaMinor != r.Rows[j].DeltaMinor {
			return r.Rows[i].DeltaMinor > r.Rows[j].DeltaMinor
		}
		return r.Rows[i].BudgetID < r.Rows[j].BudgetID
	})
	return r
}

// OnPlanTolerancePct is how far from a limit still counts as on plan.
//
// Five percent, because a budget hit to the dollar is not a thing that happens,
// and flagging a $2 overspend on a $400 grocery budget as a miss trains people
// to ignore the flag entirely.
const OnPlanTolerancePct = 5.0

// OnPlan reports whether a variance is close enough to plan to be left alone.
func (v Variance) OnPlan() bool {
	if !v.Known {
		return false
	}
	return math.Abs(v.PctOfLimit-100) <= OnPlanTolerancePct
}
