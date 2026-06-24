// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-G1", g1SuggestedContribution)
	register("SMART-G5", g5GoalConflict)
	register("SMART-G6", g6MilestoneNudge)
	register("SMART-G11", g11EmergencyFund)
	register("SMART-G13", g13Windfall)
}

const (
	windfallFactor  = 1.5    // a deposit this many× average monthly income is a windfall
	windfallRecent  = 35     // only flag windfalls in this recent window (days)
	windfallMinBase = 200_00 // ignore small "windfalls" under $200
)

// SMART-G13 — Windfall routing. Detects an unusually large recent income deposit
// (a bonus, tax refund) and suggests routing it to goals or debt rather than
// letting it drift into spending.
func g13Windfall(in Input) []smart.Insight {
	avgIncome, _ := in.trailingMonthly()
	cut := in.Now.AddDate(0, 0, -windfallRecent)
	var best domain.Transaction
	var bestBase int64
	for _, t := range in.Transactions {
		if !t.IsIncome() || t.Date.Before(cut) || t.Date.After(in.Now) {
			continue
		}
		base := in.toBaseMinor(t.Amount.Amount, t.Amount.Currency)
		if base > bestBase {
			best, bestBase = t, base
		}
	}
	if bestBase < windfallMinBase {
		return nil
	}
	// A windfall is large relative to the usual monthly income (or simply large
	// when there's no income history to compare against).
	if avgIncome > 0 && float64(bestBase) < float64(avgIncome)*windfallFactor {
		return nil
	}
	ins := smart.Insight{
		Feature: "SMART-G13",
		Page:    smart.PageGoals,
		Key:     "SMART-G13:" + best.ID,
		Title:   "You received a large deposit of " + in.baseMoney(bestBase).Format(2),
		Detail: "A " + in.baseMoney(bestBase).Format(2) + " deposit on " + best.Date.Format("Jan 2") +
			" stands out from your usual income. Routing some of it to a goal or to debt now keeps it from drifting into spending.",
		Severity: smart.SeverityNudge,
	}.WithAmount(in.baseMoney(bestBase)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open goals", Route: "/goals"})
	return []smart.Insight{ins}
}

const (
	trailingMonths      = 3  // months of history for surplus / essentials baselines
	almostTherePct      = 75 // celebrate goals at or above this completion
	emergencyTargetMos  = 6  // recommended months of essentials in the emergency fund
	emergencyMinMonthly = 50_00
)

// SMART-G1 — Suggested contribution amount. For each active goal with a deadline,
// computes the monthly contribution needed and checks it against monthly surplus.
func g1SuggestedContribution(in Input) []smart.Insight {
	surplus := in.monthlySurplusBase()
	var out []smart.Insight
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, in.Now)
		if err != nil || !ok {
			continue
		}
		neededBase := in.toBaseMinor(needed.Amount, needed.Currency)
		detail := "Saving " + needed.Format(2) + "/mo reaches " + g.Name + " by " + g.TargetDate.Format("Jan 2006") + "."
		if surplus > 0 {
			if neededBase <= surplus {
				detail += " That fits within your roughly " + in.baseMoney(surplus).Format(2) + "/mo of slack."
			} else {
				detail += " That's above your roughly " + in.baseMoney(surplus).Format(2) + "/mo of slack — consider a later date."
			}
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-G1",
			Page:     smart.PageGoals,
			Key:      "SMART-G1:" + g.ID,
			Title:    "Save " + needed.Format(2) + "/mo for " + g.Name,
			Detail:   detail,
			Severity: smart.SeverityNudge,
		}.WithAmount(needed).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open goal",
				Route: "/goals", RelatedType: "goal", RelatedID: g.ID}))
	}
	return out
}

// SMART-G5 — Trade-off / conflict detection. Flags when the active goals
// collectively demand more monthly contribution than the surplus allows.
func g5GoalConflict(in Input) []smart.Insight {
	surplus := in.monthlySurplusBase()
	if surplus <= 0 {
		return nil // can't compute a meaningful shortfall without positive surplus
	}
	var totalNeeded int64
	var n int
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, in.Now)
		if err != nil || !ok {
			continue
		}
		totalNeeded += in.toBaseMinor(needed.Amount, needed.Currency)
		n++
	}
	if n < 2 || totalNeeded <= surplus {
		return nil
	}
	shortfall := totalNeeded - surplus
	ins := smart.Insight{
		Feature: "SMART-G5",
		Page:    smart.PageGoals,
		Key:     "SMART-G5:" + in.Now.Format("2006-01"),
		Title:   "Your goals need more than you free up each month",
		Detail: plural(int64(n), "active goal") + " together need about " + in.baseMoney(totalNeeded).Format(2) +
			"/mo, but you free up roughly " + in.baseMoney(surplus).Format(2) + "/mo. Extend a deadline or trim about " +
			in.baseMoney(shortfall).Format(2) + "/mo.",
		Severity: smart.SeverityWarn,
	}.WithAmount(in.baseMoney(shortfall)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review goals", Route: "/goals"})
	return []smart.Insight{ins}
}

// SMART-G6 — Milestone celebration & nudges. Acknowledges goals that are nearly
// done or just completed.
func g6MilestoneNudge(in Input) []smart.Insight {
	var out []smart.Insight
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		complete, err := goals.IsComplete(g)
		if err != nil {
			continue
		}
		pct := goals.Percent(g)
		if complete {
			out = append(out, smart.Insight{
				Feature:  "SMART-G6",
				Page:     smart.PageGoals,
				Key:      "SMART-G6:done:" + g.ID,
				Title:    "You reached " + g.Name + "! 🎉",
				Detail:   g.Name + " is fully funded at " + g.TargetAmount.Format(2) + ". Time to set the next one.",
				Severity: smart.SeverityInfo,
			}.WithAmount(g.TargetAmount).
				WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open goal", Route: "/goals", RelatedType: "goal", RelatedID: g.ID}))
			continue
		}
		if pct >= almostTherePct {
			rem, err := goals.Remaining(g)
			if err != nil {
				continue
			}
			out = append(out, smart.Insight{
				Feature:  "SMART-G6",
				Page:     smart.PageGoals,
				Key:      "SMART-G6:near:" + g.ID,
				Title:    g.Name + " is " + itoa64(int64(pct)) + "% there",
				Detail:   "Just " + rem.Format(2) + " left on " + g.Name + " — the finish line is close.",
				Severity: smart.SeverityInfo,
			}.WithAmount(rem).
				WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open goal", Route: "/goals", RelatedType: "goal", RelatedID: g.ID}))
		}
	}
	return out
}

// SMART-G11 — Emergency-fund adequacy check. Measures a named emergency-fund goal
// against actual monthly essential spending and flags the gap.
func g11EmergencyFund(in Input) []smart.Insight {
	g, ok := emergencyGoal(in.Goals)
	if !ok {
		return nil
	}
	essentials := in.avgMonthlyExpenseBase()
	if essentials < emergencyMinMonthly {
		return nil // too little spend history to judge adequacy
	}
	current := in.toBaseMinor(g.CurrentAmount.Amount, g.CurrentAmount.Currency)
	covered := float64(current) / float64(essentials)
	if covered >= emergencyTargetMos {
		return nil
	}
	gap := essentials*emergencyTargetMos - current
	ins := smart.Insight{
		Feature: "SMART-G11",
		Page:    smart.PageGoals,
		Key:     "SMART-G11:" + g.ID,
		Title:   "Emergency fund covers " + fmtMonths(covered) + " of essentials",
		Detail: g.Name + " holds " + in.baseMoney(current).Format(2) + " against about " +
			in.baseMoney(essentials).Format(2) + "/mo of essentials. Most aim for " + itoa64(emergencyTargetMos) +
			" months — another " + in.baseMoney(gap).Format(2) + " gets there.",
		Severity: smart.SeverityNudge,
	}.WithAmount(in.baseMoney(gap)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open goal", Route: "/goals", RelatedType: "goal", RelatedID: g.ID})
	return []smart.Insight{ins}
}

// --- goal-engine helpers --------------------------------------------------

// emergencyGoal finds an active goal whose name marks it as an emergency fund.
func emergencyGoal(gs []domain.Goal) (domain.Goal, bool) {
	for _, g := range gs {
		if g.Archived {
			continue
		}
		n := strings.ToLower(g.Name)
		if strings.Contains(n, "emergency") || strings.Contains(n, "rainy") {
			return g, true
		}
	}
	return domain.Goal{}, false
}

// trailingMonthly returns average monthly income and expense magnitude (base
// minor units) over the prior `trailingMonths` whole months.
func (in Input) trailingMonthly() (income, expense int64) {
	curStart := dateutil.MonthStart(in.Now)
	var inc, exp int64
	for k := 1; k <= trailingMonths; k++ {
		s := dateutil.AddMonths(curStart, -k)
		e := dateutil.AddMonths(curStart, -k+1)
		for _, t := range in.Transactions {
			if t.IsTransfer() || t.Date.Before(s) || !t.Date.Before(e) {
				continue
			}
			base := in.toBaseMinor(t.Amount.Amount, t.Amount.Currency)
			if t.Amount.IsPositive() {
				inc += base
			} else {
				exp += -base
			}
		}
	}
	return inc / trailingMonths, exp / trailingMonths
}

// monthlySurplusBase is average monthly income minus expense over the baseline.
func (in Input) monthlySurplusBase() int64 {
	inc, exp := in.trailingMonthly()
	return inc - exp
}

// avgMonthlyExpenseBase is average monthly expense magnitude over the baseline.
func (in Input) avgMonthlyExpenseBase() int64 {
	_, exp := in.trailingMonthly()
	return exp
}

// fmtMonths renders a months-covered float like 3.2 as "3.2 months".
func fmtMonths(v float64) string {
	whole := int64(v)
	tenths := int64(v*10) % 10
	return itoa64(whole) + "." + itoa64(tenths) + " months"
}
