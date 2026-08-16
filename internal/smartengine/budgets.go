// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-B7", b7Seasonal)
	register("SMART-B8", b8SafeToSpend)
	register("SMART-B9", b9PacingNudge)
	register("SMART-B10", b10UncoveredSpending)
}

const (
	seasonalMonths    = 6     // look back this many months for seasonality
	seasonalMinMonths = 3     // need this many active months to judge
	seasonalRatio     = 2     // peak ≥ this × the trough to call it seasonal
	seasonalMinSwing  = 50_00 // and the peak-trough gap must be meaningful
)

// SMART-B7 — Seasonal budget adjustment. Detects categories whose monthly spend
// swings widely across the year and suggests month-specific budgets instead of a
// flat number.
func b7Seasonal(in Input) []smart.Insight {
	curStart := dateutil.MonthStart(in.Now)
	// category -> per-month spend (base minor), only counting months with spend.
	byCat := map[string][]int64{}
	for k := 1; k <= seasonalMonths; k++ {
		s := dateutil.AddMonths(curStart, -k)
		e := dateutil.AddMonths(curStart, -k+1)
		month := map[string]int64{}
		// TODO(splits): sums the whole-transaction category only, so these insights can
		// disagree with the splits-aware budget page for split transactions. See the
		// split contract in domain/category_split.go; fix = mirror the HasSplits branch
		// of budgeting.spentCovered / reports.categoryTotals.
		for _, t := range in.Transactions {
			if t.IsTransfer() || !t.Amount.IsNegative() || t.CategoryID == "" {
				continue
			}
			if t.Date.Before(s) || !t.Date.Before(e) {
				continue
			}
			month[t.CategoryID] += in.toBaseMinor(-t.Amount.Amount, t.Amount.Currency)
		}
		for cat, v := range month {
			byCat[cat] = append(byCat[cat], v)
		}
	}
	names := categoryNames(in.Categories)
	var out []smart.Insight
	for cat, vals := range byCat {
		if len(vals) < seasonalMinMonths {
			continue
		}
		lo, hi := vals[0], vals[0]
		for _, v := range vals {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		if lo <= 0 || hi < lo*seasonalRatio || hi-lo < seasonalMinSwing {
			continue
		}
		name := names[cat]
		if name == "" {
			continue
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-B7",
			Page:     smart.PageBudgets,
			Key:      "SMART-B7:" + cat,
			Severity: smart.SeverityNudge,
		}.
			WithTitle("smart.b7.title", "%s spending is seasonal", name).
			WithDetail("smart.b7.detail", "%s ranged from %s to %s/mo across recent months. A month-specific budget fits it better than a flat number.", name, hmoneyc(lo, in.Base), hmoneyc(hi, in.Base)).
			WithAmount(mny(hi-lo, in.Base)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/budgets", RelatedType: "category", RelatedID: cat}.
				WithLabel("smart.b7.action", "Open budgets")))
	}
	return out
}

const (
	pacingMinElapsed   = 0.20  // ignore pace projections before this much of the period
	pacingNearBudget   = 0.80  // "near" threshold for budget evaluation
	uncoveredMinMonth  = 75_00 // a category needs this much monthly spend to nudge a budget
	safeToSpendFloorAb = 1_00  // only surface safe-to-spend when there's meaningful cash
)

// SMART-B8 — Safe-to-spend indicator. One glanceable number: liquid cash minus
// the bills still due this month and this month's remaining goal contributions.
func b8SafeToSpend(in Input) []smart.Insight {
	// A brand-new dataset has no accounts at all — "your spendable cash is
	// $0.00" is an alarming non-fact there, not a warning. Say nothing until
	// the user has any account to be low ON (C356).
	if len(in.Accounts) == 0 {
		return nil
	}
	liquid := totalLiquidBase(in)
	billsLeft := in.billsRestOfMonthBase()
	goalNeeds := in.goalMonthlyNeedsBase()
	safe := liquid - billsLeft - goalNeeds
	if liquid < safeToSpendFloorAb {
		// Surface a low-balance warning rather than silently returning nothing:
		// a near-empty wallet is itself the key fact the user should see.
		return []smart.Insight{smart.Insight{
			Feature:  "SMART-B8",
			Page:     smart.PageBudgets,
			Key:      "SMART-B8:" + in.Now.Format("2006-01"),
			Severity: smart.SeverityWarn,
		}.
			WithTitle("smart.b8.lowTitle", "Liquid cash is very low").
			WithDetail("smart.b8.lowDetail",
				"Your spendable cash is %s — well below the level needed to cover upcoming bills and goal contributions.",
				hmoneyc(liquid, in.Base))}
	}
	sev := smart.SeverityInfo
	titleKey, titleFmt := "smart.b8.title", "%s is safe to spend"
	titleArgs := []any{hmoneyc(safe, in.Base)}
	detailKey := "smart.b8.detail"
	detailFmt := "After the bills still due this month (%s) and your goal contributions (%s), about %s of your %s liquid cash is genuinely free."
	detailArgs := []any{hmoneyc(billsLeft, in.Base), hmoneyc(goalNeeds, in.Base),
		hmoneyc(safe, in.Base), hmoneyc(liquid, in.Base)}
	if safe < 0 {
		sev = smart.SeverityWarn
		titleKey, titleFmt, titleArgs = "smart.b8.tightTitle", "Spending is tight this month", nil
		detailKey = "smart.b8.tightDetail"
		detailFmt = "Your bills and goal contributions this month exceed liquid cash by %s — hold off on discretionary spending."
		detailArgs = []any{hmoneyc(-safe, in.Base)}
	}
	ins := smart.Insight{
		Feature:  "SMART-B8",
		Page:     smart.PageBudgets,
		Key:      "SMART-B8:" + in.Now.Format("2006-01"),
		Severity: sev,
	}.
		WithTitle(titleKey, titleFmt, titleArgs...).
		WithDetail(detailKey, detailFmt, detailArgs...).
		WithAmount(mny(safe, in.Base))
	return []smart.Insight{ins}
}

// SMART-B9 — Budget pacing nudges. Flags budgets projected to overspend by the
// end of their period and the per-week trim to get back on track.
func b9PacingNudge(in Input) []smart.Insight {
	var out []smart.Insight
	for _, b := range in.Budgets {
		start, end := budgeting.PeriodRange(b.Period, in.Now, in.WeekStart)
		st, err := budgeting.Evaluate(b, in.Transactions, start, end, in.Rates, pacingNearBudget)
		if err != nil {
			continue
		}
		pace := budgeting.ProjectPace(st, start, end, in.Now)
		if pace.OnTrack || pace.Elapsed < pacingMinElapsed {
			continue
		}
		name := budgetName(b)
		over := hmoneyc(pace.OverBy.Amount, pace.OverBy.Currency)
		out = append(out, smart.Insight{
			Feature:  "SMART-B9",
			Page:     smart.PageBudgets,
			Key:      "SMART-B9:" + b.ID + ":" + start.Format("2006-01-02"),
			Severity: smart.SeverityWarn,
		}.
			WithTitle("smart.b9.title", "%s is on pace to exceed its budget", name).
			WithDetail("smart.b9.detail", "%s is projected to exceed budget by %s this period. Slowing spending now would keep it closer to plan.", name, over).
			WithAmount(pace.OverBy).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/budgets", RelatedType: "budget", RelatedID: b.ID}.
				WithLabel("smart.b9.action", "Open budgets")))
	}
	return out
}

// SMART-B10 — Uncovered-spending finder. Surfaces categories with real recurring
// spend that no budget covers yet.
func b10UncoveredSpending(in Input) []smart.Insight {
	covered := budgetedCategories(in.Budgets)
	names := categoryNames(in.Categories)
	byCat := in.trailingExpenseByCategory()
	var out []smart.Insight
	for catID, monthly := range byCat {
		if catID == "" || covered[catID] || monthly < uncoveredMinMonth {
			continue
		}
		name := names[catID]
		if name == "" {
			continue
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-B10",
			Page:     smart.PageBudgets,
			Key:      "SMART-B10:" + catID,
			Severity: smart.SeverityNudge,
		}.
			WithTitle("smart.b10.title", "%s has no budget yet", name).
			WithDetail("smart.b10.detail", "You spend about %s/mo on %s with no budget covering it — adding one keeps it from slipping through.", hmoneyc(monthly, in.Base), name).
			WithAmount(mny(monthly, in.Base)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/budgets", RelatedType: "category", RelatedID: catID}.
				WithLabel("smart.b10.action", "Add a budget")))
	}
	return out
}

// --- budget-engine helpers ------------------------------------------------

// budgetName returns a display label for a budget.
func budgetName(b domain.Budget) string {
	if b.Name != "" {
		return b.Name
	}
	return "This budget"
}

// budgetedCategories is the set of category ids that already have a budget.
func budgetedCategories(bs []domain.Budget) map[string]bool {
	m := map[string]bool{}
	for _, b := range bs {
		if b.CategoryID != "" {
			m[b.CategoryID] = true
		}
	}
	return m
}

// trailingExpenseByCategory returns average monthly expense per category (base
// minor units) over the trailing baseline window.
func (in Input) trailingExpenseByCategory() map[string]int64 {
	curStart := dateutil.MonthStart(in.Now)
	sum := map[string]int64{}
	for k := 1; k <= trailingMonths; k++ {
		s := dateutil.AddMonths(curStart, -k)
		e := dateutil.AddMonths(curStart, -k+1)
		for _, t := range in.Transactions {
			if t.IsTransfer() || !t.Amount.IsNegative() || t.Date.Before(s) || !t.Date.Before(e) {
				continue
			}
			sum[t.CategoryID] += in.toBaseMinor(-t.Amount.Amount, t.Amount.Currency)
		}
	}
	for k := range sum {
		sum[k] /= trailingMonths
	}
	return sum
}

// billsRestOfMonthBase sums bills due between now and month-end, in base units.
func (in Input) billsRestOfMonthBase() int64 {
	_, monthEnd := dateutil.MonthRange(in.Now)
	var total int64
	for _, b := range bills.UpcomingAll(in.Accounts, in.Recurring, in.Now) {
		if b.DueDate.After(monthEnd) {
			continue
		}
		total += in.toBaseMinor(b.Amount.Amount, b.Amount.Currency)
	}
	return total
}

// goalMonthlyNeedsBase sums each active goal's required monthly contribution.
func (in Input) goalMonthlyNeedsBase() int64 {
	var total int64
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, in.Now)
		if err != nil || !ok {
			continue
		}
		total += in.toBaseMinor(needed.Amount, needed.Currency)
	}
	return total
}
