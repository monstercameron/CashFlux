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
			Feature: "SMART-B7",
			Page:    smart.PageBudgets,
			Key:     "SMART-B7:" + cat,
			Title:   name + " spending is seasonal",
			Detail: name + " ranged from " + mny(lo, in.Base).Format(2) + " to " + mny(hi, in.Base).Format(2) +
				"/mo across recent months. A month-specific budget fits it better than a flat number.",
			Severity: smart.SeverityNudge,
		}.WithAmount(mny(hi-lo, in.Base)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open budgets", Route: "/budgets", RelatedType: "category", RelatedID: cat}))
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
	liquid := totalLiquidBase(in)
	billsLeft := in.billsRestOfMonthBase()
	goalNeeds := in.goalMonthlyNeedsBase()
	safe := liquid - billsLeft - goalNeeds
	if liquid < safeToSpendFloorAb {
		return nil // nothing meaningful to report on an empty wallet
	}
	sev := smart.SeverityInfo
	title := mny(safe, in.Base).Format(2) + " is safe to spend"
	detail := "After the bills still due this month (" + mny(billsLeft, in.Base).Format(2) +
		") and your goal contributions (" + mny(goalNeeds, in.Base).Format(2) +
		"), about " + mny(safe, in.Base).Format(2) + " of your " + mny(liquid, in.Base).Format(2) +
		" liquid cash is genuinely free."
	if safe < 0 {
		sev = smart.SeverityWarn
		title = "Spending is tight this month"
		detail = "Your bills and goal contributions this month exceed liquid cash by " +
			mny(-safe, in.Base).Format(2) + " — hold off on discretionary spending."
	}
	ins := smart.Insight{
		Feature:  "SMART-B8",
		Page:     smart.PageBudgets,
		Key:      "SMART-B8:" + in.Now.Format("2006-01"),
		Title:    title,
		Detail:   detail,
		Severity: sev,
	}.WithAmount(mny(safe, in.Base))
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
		out = append(out, smart.Insight{
			Feature: "SMART-B9",
			Page:    smart.PageBudgets,
			Key:     "SMART-B9:" + b.ID + ":" + start.Format("2006-01-02"),
			Title:   name + " is on pace to go over",
			Detail: "At the current rate " + name + " is projected to finish about " +
				pace.OverBy.Format(2) + " over its " + st.Spent.Currency + " limit. Easing off now keeps it in budget.",
			Severity: smart.SeverityWarn,
		}.WithAmount(pace.OverBy).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open budgets",
				Route: "/budgets", RelatedType: "budget", RelatedID: b.ID}))
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
			Feature: "SMART-B10",
			Page:    smart.PageBudgets,
			Key:     "SMART-B10:" + catID,
			Title:   name + " has no budget yet",
			Detail: "You spend about " + mny(monthly, in.Base).Format(2) + "/mo on " + name +
				" with no budget covering it — adding one keeps it from slipping through.",
			Severity: smart.SeverityNudge,
		}.WithAmount(mny(monthly, in.Base)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Add a budget",
				Route: "/budgets", RelatedType: "category", RelatedID: catID}))
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
