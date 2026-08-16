// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/payoff"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-G1", g1SuggestedContribution)
	register("SMART-G5", g5GoalConflict)
	register("SMART-G6", g6MilestoneNudge)
	register("SMART-G11", g11EmergencyFund)
	register("SMART-G12", g12SuggestGoals)
	register("SMART-G8", g8GoalImpact)
	register("SMART-G3", g3AllocateSurplus)
	register("SMART-G13", g13Windfall)
	register("SMART-G14", g14LinkAccount)
	register("SMART-G15", g15DebtStrategy)
	register("SMART-G2", g2Forecast)
	register("SMART-G10", g10WhatIf)
	register("SMART-G17", g17AutoContribute)
	register("SMART-G18", g18Feasibility)
	register("SMART-G19", g19BorrowWarning)
	register("SMART-G20", g20Shared)
}

// SMART-G2 — Goal completion forecast. Projects a deadline goal's finish date if
// the user put their whole spare surplus toward it, and warns when even that
// lands after the planned date.
func g2Forecast(in Input) []smart.Insight {
	surplus := in.monthlySurplusBase()
	if surplus <= 0 {
		return nil
	}
	var out []smart.Insight
	for _, g := range in.Goals {
		if g.Archived || g.TargetDate.IsZero() {
			continue
		}
		if c, err := goals.IsComplete(g); err != nil || c {
			continue
		}
		finish, ok, err := goals.Project(g, mny(surplus, g.TargetAmount.Currency), in.Now)
		if err != nil || !ok || !finish.After(g.TargetDate) {
			continue // on track or ahead at this pace — no warning
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-G2",
			Page:     smart.PageGoals,
			Key:      "SMART-G2:" + g.ID,
			Severity: smart.SeverityWarn,
		}.
			WithTitle("smart.g2.title", "%s is likely to finish late", g.Name).
			WithDetail("smart.g2.detail", "Even putting your full %s/mo surplus toward %s, it lands around %s — later than the planned %s.", in.hmoney(surplus), g.Name, finish.Format("Jan 2006"), g.TargetDate.Format("Jan 2006")).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
				WithLabel("smart.g2.action", "Open goal")))
	}
	return out
}

const g10ExtraStep = 100_00 // the illustrative "add $100/mo" what-if

// SMART-G10 — "What if" goal simulator. Shows how much sooner a deadline goal
// finishes if the user adds a fixed extra contribution each month.
func g10WhatIf(in Input) []smart.Insight {
	var out []smart.Insight
	for _, g := range in.Goals {
		if g.Archived || g.TargetDate.IsZero() {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, in.Now)
		if err != nil || !ok {
			continue
		}
		cur := needed.Amount
		base, ok1, _ := goals.Project(g, mny(cur, needed.Currency), in.Now)
		faster, ok2, _ := goals.Project(g, mny(cur+g10ExtraStep, needed.Currency), in.Now)
		if !ok1 || !ok2 {
			continue
		}
		months := monthsBetween(faster, base)
		if months < 1 {
			continue
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-G10",
			Page:     smart.PageGoals,
			Key:      "SMART-G10:" + g.ID,
			Severity: smart.SeverityInfo,
		}.
			WithTitle("smart.g10.title", "Add %s/mo to finish %s sooner", hmoneyc(g10ExtraStep, in.Base), g.Name).
			WithDetail("smart.g10.detail", "Contributing %s/mo beyond the planned pace brings %s in about %s sooner.", hmoneyc(g10ExtraStep, in.Base), g.Name, plural(months, "month")).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
				WithLabel("smart.g10.action", "Open goal")))
	}
	return out
}

// SMART-G20 — Shared / household goal contributions. For a shared goal in a
// multi-member household, nudges tracking who contributes what.
func g20Shared(in Input) []smart.Insight {
	if len(in.Members) < 2 {
		return nil
	}
	var out []smart.Insight
	for _, g := range in.Goals {
		if g.Archived || g.Scope != domain.ScopeShared {
			continue
		}
		if c, err := goals.IsComplete(g); err != nil || c {
			continue
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-G20",
			Page:     smart.PageGoals,
			Key:      "SMART-G20:" + g.ID,
			Severity: smart.SeverityInfo,
		}.
			WithTitle("smart.g20.title", "%s is a shared goal", g.Name).
			WithDetail("smart.g20.detail", "%s is shared across your %s. Logging contributions by member keeps each person's share clear.", g.Name, plural(int64(len(in.Members)), "household member")).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
				WithLabel("smart.g20.action", "Open goal")))
	}
	return out
}

// SMART-G17 — Recurring auto-contribution scheduling. For a goal with a deadline
// and a detected payday, nudges the user to automate a standing monthly
// contribution via a pay-yourself-first workflow.
//
// When the goal has a linked destination account (goal.AccountID is set), the
// action is ActionAutomateGoal, which creates a real scheduled transfer workflow
// in the app (C186 / C256). When the goal has no linked account yet, the action
// degrades to ActionNavigate → /goals so the user can link one first.
func g17AutoContribute(in Input) []smart.Insight {
	payday, ok := recentPayday(in)
	if !ok {
		return nil
	}
	var out []smart.Insight
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, in.Now)
		if err != nil || !ok {
			continue
		}

		var action smart.Action
		if g.AccountID != "" {
			// Goal has a destination account: offer the one-tap automate action.
			action = smart.Action{
				Kind:              smart.ActionAutomateGoal,
				GoalID:            g.ID,
				GoalMonthlyAmount: needed.Amount,
				RelatedType:       "goal",
				RelatedID:         g.ID,
			}.
				WithLabel("smart.g20.action2", "Set up automatic transfer")
		} else {
			// No linked account yet: navigate so the user can link one first.
			action = smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
				WithLabel("smart.g20.action3", "Open goal")
		}

		out = append(out, smart.Insight{
			Feature:  "SMART-G17",
			Page:     smart.PageGoals,
			Key:      "SMART-G17:" + g.ID,
			Severity: smart.SeverityInfo,
		}.
			WithTitle("smart.g17.title", "Automate %s: %s each payday", g.Name, hm(needed)).
			WithDetail("smart.g17.detail", "Setting a standing rule to move %s to %s on each payday (around the %s) keeps it on track without remembering it every month.", hm(needed), g.Name, ordinalDay(payday)).
			WithAmount(needed).
			WithAction(action))
	}
	return out
}

const g19MinShortfall = 50_00 // ignore small differences between goal and balance

// SMART-G19 — Borrow-from-goal warning. For a goal linked to an account, warns
// when the account's balance has fallen below the goal's recorded progress — a
// sign funds were pulled out, setting the goal back.
func g19BorrowWarning(in Input) []smart.Insight {
	balByAccount := map[string]int64{}
	for _, a := range in.Accounts {
		if a.Archived {
			continue
		}
		if bal, err := ledger.Balance(a, in.Transactions); err == nil {
			balByAccount[a.ID] = in.toBaseMinor(bal.Amount, a.Currency)
		}
	}
	var out []smart.Insight
	for _, g := range in.Goals {
		if g.Archived || g.AccountID == "" {
			continue
		}
		bal, ok := balByAccount[g.AccountID]
		if !ok {
			continue
		}
		cur := in.toBaseMinor(g.CurrentAmount.Amount, g.CurrentAmount.Currency)
		if cur-bal < g19MinShortfall {
			continue
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-G19",
			Page:     smart.PageGoals,
			Key:      "SMART-G19:" + g.ID,
			Severity: smart.SeverityWarn,
		}.
			WithTitle("smart.g19.title", "%s's linked account is below its saved amount", g.Name).
			WithDetail("smart.g19.detail", "%s shows %s saved, but its linked account holds only %s — if you borrowed from it, the goal is set back by %s.", g.Name, hm(g.CurrentAmount), in.hmoney(bal), in.hmoney(cur-bal)).
			WithAmount(in.baseMoney(cur-bal)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
				WithLabel("smart.g19.action", "Open goal")))
	}
	return out
}

// SMART-G14 — Goal-linked account binding. Suggests linking a goal to an account
// so its progress tracks the real balance automatically instead of by hand.
func g14LinkAccount(in Input) []smart.Insight {
	hasAsset := false
	for _, a := range in.Accounts {
		if !a.Archived && a.Class == domain.ClassAsset {
			hasAsset = true
			break
		}
	}
	if !hasAsset {
		return nil
	}
	var out []smart.Insight
	for _, g := range in.Goals {
		if g.Archived || g.AccountID != "" {
			continue
		}
		if c, err := goals.IsComplete(g); err == nil && c {
			continue
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-G14",
			Page:     smart.PageGoals,
			Key:      "SMART-G14:" + g.ID,
			Severity: smart.SeverityInfo,
		}.
			WithTitle("smart.g14.title", "Link %s to an account", g.Name).
			WithDetail("smart.g14.detail", "Linking %s to a savings account lets its progress track the real balance automatically, instead of updating it by hand.", g.Name).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
				WithLabel("smart.g14.action", "Open goal")))
	}
	return out
}

const g3MinSurplus = 50_00 // only nudge to allocate a meaningful leftover

// SMART-G3 — Auto-allocate surplus. When there's a recurring monthly surplus and
// active goals, nudges the user to route the leftover into their goals.
func g3AllocateSurplus(in Input) []smart.Insight {
	surplus := in.monthlySurplusBase()
	if surplus < g3MinSurplus {
		return nil
	}
	var n int
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		if c, err := goals.IsComplete(g); err == nil && !c {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	ins := smart.Insight{
		Feature:  "SMART-G3",
		Page:     smart.PageGoals,
		Key:      "SMART-G3:" + in.Now.Format("2006-01"),
		Severity: smart.SeverityNudge,
	}.
		WithTitle("smart.g3.title", "You're freeing up about %s/mo", in.hmoney(surplus)).
		WithDetail("smart.g3.detail", "After income and spending you have roughly %s/mo left over. Routing it across your %s would reach them sooner.", in.hmoney(surplus), plural(int64(n), "active goal")).
		WithAmount(in.baseMoney(surplus)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals"}.
			WithLabel("smart.g3.action", "Open goals"))
	return []smart.Insight{ins}
}

// SMART-G15 — Debt-payoff strategy optimizer. Compares avalanche (highest APR
// first) against snowball (smallest balance first) and surfaces the interest
// saved by avalanche, naming where to send the extra payment.
func g15DebtStrategy(in Input) []smart.Insight {
	debts := buildDebts(in)
	if len(debts) < 2 {
		return nil // with one debt both strategies are identical
	}
	extra := payoff.SuggestedExtra(debts)
	ava, ok1 := payoff.BuildPlan(debts, extra, payoff.Avalanche)
	sno, ok2 := payoff.BuildPlan(debts, extra, payoff.Snowball)
	if !ok1 || !ok2 {
		return nil
	}
	saved := sno.TotalInterest - ava.TotalInterest
	if saved <= 0 {
		return nil // no avalanche advantage (a tie) — nothing to recommend
	}
	target := highestAPRDebt(debts)
	ins := smart.Insight{
		Feature:  "SMART-G15",
		Page:     smart.PageGoals,
		Key:      "SMART-G15:" + in.Now.Format("2006-01"),
		Severity: smart.SeverityNudge,
	}.
		WithTitle("smart.g15.title", "Avalanche could save %s in interest", in.hmoney(saved)).
		WithDetail("smart.g15.detail", "Paying your highest-interest debt (%s) first instead of the smallest balance saves about %s in interest over the payoff.", target, in.hmoney(saved)).
		WithAmount(in.baseMoney(saved)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/planning"}.
			WithLabel("smart.g15.action", "Open planning"))
	return []smart.Insight{ins}
}

const g8MinExpense = 100_00 // only translate sizable purchases into goal terms

// SMART-G8 — Goal-impact preview. Expresses the month's biggest expense in terms
// of a goal's saving pace, making the trade-off tangible.
func g8GoalImpact(in Input) []smart.Insight {
	// Pick the most demanding deadline goal as the reference pace.
	var refGoal domain.Goal
	var pace int64
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, in.Now)
		if err != nil || !ok {
			continue
		}
		p := in.toBaseMinor(needed.Amount, needed.Currency)
		if p > pace {
			refGoal, pace = g, p
		}
	}
	if pace <= 0 {
		return nil
	}
	// The biggest recent expense.
	cut := in.Now.AddDate(0, 0, -spikeRecentDays)
	var big domain.Transaction
	var bigMag int64
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() || t.Date.Before(cut) || t.Date.After(in.Now) {
			continue
		}
		mag := in.toBaseMinor(-t.Amount.Amount, t.Amount.Currency)
		if mag > bigMag {
			big, bigMag = t, mag
		}
	}
	if bigMag < g8MinExpense {
		return nil
	}
	weeklyPace := pace * 12 / 52
	if weeklyPace <= 0 {
		return nil
	}
	weeks := (bigMag + weeklyPace/2) / weeklyPace
	if weeks < 1 {
		weeks = 1
	}
	ins := smart.Insight{
		Feature:  "SMART-G8",
		Page:     smart.PageGoals,
		Key:      "SMART-G8:" + big.ID,
		Severity: smart.SeverityInfo,
	}.
		WithTitle("smart.g8.title", "That %s is %s of %s", in.hmoney(bigMag), plural(weeks, "week"), refGoal.Name).
		WithDetail("smart.g8.detail", "%s on %s (%s) equals about %s of your %s saving pace.", txnLabel(big), big.Date.Format("Jan 2"), in.hmoney(bigMag), plural(weeks, "week"), refGoal.Name).
		WithAmount(in.baseMoney(bigMag)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: refGoal.ID}.
			WithLabel("smart.g8.action", "Open goal"))
	return []smart.Insight{ins}
}

const emergencyStartMonths = 3 // a sensible starting emergency-fund target

// SMART-G12 — Auto-create suggested goals. When the user has no emergency fund
// and enough spend history to size one, suggests starting one.
func g12SuggestGoals(in Input) []smart.Insight {
	if _, ok := emergencyGoal(in.Goals); ok {
		return nil // already has one — G11 tracks its adequacy instead
	}
	essentials := in.avgMonthlyExpenseBase()
	if essentials < emergencyMinMonthly {
		return nil
	}
	target := essentials * emergencyStartMonths
	ins := smart.Insight{
		Feature:  "SMART-G12",
		Page:     smart.PageGoals,
		Key:      "SMART-G12:emergency",
		Severity: smart.SeverityNudge,
	}.
		WithTitle("smart.g12.title", "Consider starting an emergency fund").
		WithDetail("smart.g12.detail", "You don't have an emergency fund yet. A common starting target is %s — about %s months of your roughly %s/mo essentials.", in.hmoney(target), itoa64(emergencyStartMonths), in.hmoney(essentials)).
		WithAmount(in.baseMoney(target)).
		WithAction(smart.Action{
			Kind:         smart.ActionCreateGoal,
			GoalName:     "Emergency Fund",
			GoalTarget:   target,
			GoalCurrency: in.Base,
		}.
			WithLabel("smart.g12.action", "Create goal"))
	return []smart.Insight{ins}
}

// SMART-G18 — Goal feasibility traffic-light. Flags each deadline goal whose
// required monthly contribution exceeds a fair share of the available surplus —
// the "red light" that its deadline is unrealistic at the current pace.
func g18Feasibility(in Input) []smart.Insight {
	surplus := in.monthlySurplusBase()
	type dg struct {
		g      domain.Goal
		needed int64
	}
	var deadlined []dg
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, in.Now)
		if err != nil || !ok {
			continue
		}
		deadlined = append(deadlined, dg{g: g, needed: in.toBaseMinor(needed.Amount, needed.Currency)})
	}
	if len(deadlined) == 0 {
		return nil
	}
	fair := int64(0)
	if surplus > 0 {
		fair = surplus / int64(len(deadlined))
	}
	var out []smart.Insight
	for _, d := range deadlined {
		if d.needed <= fair { // green — comfortably affordable
			continue
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-G18",
			Page:     smart.PageGoals,
			Key:      "SMART-G18:" + d.g.ID,
			Severity: smart.SeverityWarn,
		}.
			WithTitle("smart.g18.title", "%s's deadline looks tight", d.g.Name).
			WithDetail("smart.g18.detail", "%s needs %s/mo to hit %s, but only about %s/mo is realistically free for it — consider extending the date or trimming elsewhere.", d.g.Name, in.hmoney(d.needed), d.g.TargetDate.Format("Jan 2006"), in.hmoney(fair)).
			WithAmount(in.baseMoney(d.needed)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: d.g.ID}.
				WithLabel("smart.g18.action", "Open goal")))
	}
	return out
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
		Feature:  "SMART-G13",
		Page:     smart.PageGoals,
		Key:      "SMART-G13:" + best.ID,
		Severity: smart.SeverityNudge,
	}.
		WithTitle("smart.g13.title", "You received a large deposit of %s", in.hmoney(bestBase)).
		WithDetail("smart.g13.detail", "A %s deposit on %s stands out from your usual income. Routing some of it to a goal or to debt now keeps it from drifting into spending.", in.hmoney(bestBase), best.Date.Format("Jan 2")).
		WithAmount(in.baseMoney(bestBase)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals"}.
			WithLabel("smart.g13.action", "Open goals"))
	return []smart.Insight{ins}
}

const (
	trailingMonths      = 3  // months of history for surplus / essentials baselines
	almostTherePct      = 75 // celebrate goals at or above this completion
	emergencyTargetMos  = 6  // recommended months of essentials in the emergency fund
	emergencyMinMonthly = 50_00
)

// goalDeadlineFairShare returns the household's monthly surplus (base minor) and the
// fair-share slice of it per deadlined goal — the SINGLE affordability model every
// goal insight and the Goals card badge key off, so no two surfaces ever frame
// "available money" differently (total slack in one place, fair share in another,
// which read as a contradiction). n is the count of active, incomplete, dated goals
// sharing the surplus.
func (in Input) goalDeadlineFairShare() (surplus, fair int64, n int) {
	surplus = in.monthlySurplusBase()
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		if _, ok, err := goals.MonthlyNeeded(g, in.Now); err == nil && ok {
			n++
		}
	}
	if surplus > 0 && n > 0 {
		fair = surplus / int64(n)
	}
	return surplus, fair, n
}

// SMART-G1 — Suggested contribution amount. For each active goal with a deadline,
// computes the monthly contribution needed and frames it against the goal's FAIR
// SHARE of the slack — the same model SMART-G18 and the Goals card use — so the app
// gives one consistent answer instead of "fits within your $X slack" here while the
// feasibility note says only a fraction is realistically free.
func g1SuggestedContribution(in Input) []smart.Insight {
	surplus, fair, n := in.goalDeadlineFairShare()
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
		detailKey := "smart.g1.detail"
		detailFmt := "Saving %s/mo reaches %s by %s."
		detailArgs := []any{hm(needed), g.Name, g.TargetDate.Format("Jan 2006")}
		if surplus > 0 {
			switch goals.AssessHealth(neededBase, surplus, n) {
			case goals.HealthOnTrack:
				detailKey, detailFmt = "smart.g1.detailOnTrack", detailFmt+" That fits within its roughly %s/mo share of your %s/mo of slack."
				detailArgs = append(detailArgs, in.hmoney(fair), in.hmoney(surplus))
			case goals.HealthWatch:
				detailKey, detailFmt = "smart.g1.detailWatch", detailFmt+" That's more than its roughly %s/mo fair share of your %s/mo slack — it would crowd your other goals, so consider a later date."
				detailArgs = append(detailArgs, in.hmoney(fair), in.hmoney(surplus))
			case goals.HealthAtRisk:
				detailKey, detailFmt = "smart.g1.detailAtRisk", detailFmt+" That's above your entire %s/mo of slack — consider a later date."
				detailArgs = append(detailArgs, in.hmoney(surplus))
			}
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-G1",
			Page:     smart.PageGoals,
			Key:      "SMART-G1:" + g.ID,
			Severity: smart.SeverityNudge,
		}.
			WithTitle("smart.g1.title", "Save %s/mo for %s", hm(needed), g.Name).
			WithDetail(detailKey, detailFmt, detailArgs...).
			WithAmount(needed).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
				WithLabel("smart.g1.action", "Open goal")))
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
		Feature:  "SMART-G5",
		Page:     smart.PageGoals,
		Key:      "SMART-G5:" + in.Now.Format("2006-01"),
		Severity: smart.SeverityWarn,
	}.
		WithTitle("smart.g5.title", "Your goals are overcommitted by %s/mo", in.hmoney(shortfall)).
		WithDetail("smart.g5.detail", "You have about %s/mo of slack, but your %s require %s/mo. Extend a deadline or trim a goal to close the gap.", in.hmoney(surplus), plural(int64(n), "active goal"), in.hmoney(totalNeeded)).
		WithAmount(in.baseMoney(shortfall)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals"}.
			WithLabel("smart.g5.action", "Review goals"))
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
				Severity: smart.SeverityInfo,
			}.
				WithTitle("smart.g6.title", "You reached %s! 🎉", g.Name).
				WithDetail("smart.g6.detail", "%s is fully funded at %s. Time to set the next one.", g.Name, hm(g.TargetAmount)).
				WithAmount(g.TargetAmount).
				WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
					WithLabel("smart.g6.action", "Open goal")))
			continue
		}
		if pct >= almostTherePct {
			rem, err := goals.CoveredRemaining(g)
			if err != nil {
				continue
			}
			out = append(out, smart.Insight{
				Feature:  "SMART-G6",
				Page:     smart.PageGoals,
				Key:      "SMART-G6:near:" + g.ID,
				Severity: smart.SeverityInfo,
			}.
				WithTitle("smart.g6.title2", "%s is %s%% there", g.Name, itoa64(int64(pct))).
				WithDetail("smart.g6.detail2", "Just %s left on %s — the finish line is close.", hm(rem), g.Name).
				WithAmount(rem).
				WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
					WithLabel("smart.g6.action2", "Open goal")))
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
		Feature:  "SMART-G11",
		Page:     smart.PageGoals,
		Key:      "SMART-G11:" + g.ID,
		Severity: smart.SeverityNudge,
	}.
		WithTitle("smart.g11.title", "Emergency fund covers %s of essentials", fmtMonths(covered)).
		WithDetail("smart.g11.detail", "%s holds %s against about %s/mo of essentials. Most aim for %s months — another %s gets there.", g.Name, in.hmoney(current), in.hmoney(essentials), itoa64(emergencyTargetMos), in.hmoney(gap)).
		WithAmount(in.baseMoney(gap)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/goals", RelatedType: "goal", RelatedID: g.ID}.
			WithLabel("smart.g11.action", "Open goal"))
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
// minor units) over the prior `trailingMonths` whole months. It delegates to the
// shared cashflow definition so the assistant's "available monthly money" is the
// exact same figure the Goals pace verdict and other surfaces use — no surface
// computes its own surplus and disagrees.
func (in Input) trailingMonthly() (income, expense int64) {
	return cashflow.TrailingMonthly(in.Transactions, in.Rates, in.Base, in.Now, trailingMonths)
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
