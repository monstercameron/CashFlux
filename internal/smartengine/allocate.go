// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/payoff"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-AL1", al1SuggestedProfile)
	register("SMART-AL3", al3SmartReserve)
	register("SMART-AL5", al5OutcomePreview)
}

// SMART-AL5 — Allocation outcome preview. Previews the impact of allocating the
// monthly surplus to the highest-interest debt — how soon it clears — so the
// payoff of allocating is visible before doing it.
func al5OutcomePreview(in Input) []smart.Insight {
	surplus := in.monthlySurplusBase()
	if surplus <= 0 {
		return nil
	}
	debts := buildDebts(in)
	if len(debts) == 0 {
		return nil
	}
	plan, ok := payoff.BuildPlan(debts, surplus, payoff.Avalanche)
	if !ok || plan.Months <= 0 {
		return nil
	}
	target := highestAPRDebt(debts)
	ins := smart.Insight{
		Feature: "SMART-AL5",
		Page:    smart.PageAllocate,
		Key:     "SMART-AL5:" + in.Now.Format("2006-01"),
		Title:   "Allocating your surplus clears debt in " + plural(int64(plan.Months), "month"),
		Detail: "Putting your " + in.hmoney(surplus) + "/mo surplus toward " + target +
			" (highest-interest first) clears your debt in about " + plural(int64(plan.Months), "month") + ".",
		Severity: smart.SeverityInfo,
	}.WithAmount(in.baseMoney(surplus)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open allocate", Route: "/allocate"})
	return []smart.Insight{ins}
}

const (
	highAPRThreshold  = 10.0  // a liability at or above this APR is "high-interest"
	thinEmergencyMos  = 3     // fewer months than this of essentials is "thin"
	reserveMinMonthly = 50_00 // need this much essential spend to suggest a reserve
)

// SMART-AL1 — Auto-suggested profile. Recommends the allocation weight profile
// that best fits the user's current situation, with a one-line reason.
func al1SuggestedProfile(in Input) []smart.Insight {
	profile, why := suggestProfile(in)
	if profile == "" {
		return nil
	}
	ins := smart.Insight{
		Feature:  "SMART-AL1",
		Page:     smart.PageAllocate,
		Key:      "SMART-AL1:" + profile,
		Title:    "The \"" + profile + "\" profile fits your situation",
		Detail:   why + " Start from the " + profile + " profile, then adjust as you like.",
		Severity: smart.SeverityNudge,
	}.WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open allocate", Route: "/allocate"})
	return []smart.Insight{ins}
}

// SMART-AL3 — Smart reserve suggestion. Pre-fills the emergency-buffer reserve
// from real essential monthly spend × the target months.
func al3SmartReserve(in Input) []smart.Insight {
	essentials := in.avgMonthlyExpenseBase()
	if essentials < reserveMinMonthly {
		return nil
	}
	reserve := essentials * emergencyTargetMos
	ins := smart.Insight{
		Feature: "SMART-AL3",
		Page:    smart.PageAllocate,
		Key:     "SMART-AL3:" + in.Now.Format("2006-01"),
		Title:   "Suggested reserve: " + in.hmoney(reserve),
		Detail: "Holding back about " + in.hmoney(reserve) + " (" + itoa64(emergencyTargetMos) +
			" months of your roughly " + in.hmoney(essentials) +
			"/mo essentials) keeps a real buffer before allocating the rest.",
		Severity: smart.SeverityInfo,
	}.WithAmount(in.baseMoney(reserve)).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Open allocate", Route: "/allocate"})
	return []smart.Insight{ins}
}

// --- allocate-engine helpers ----------------------------------------------

// suggestProfile picks the allocation profile that best fits the data and a
// human reason. It returns "" when there's nothing to base a suggestion on.
func suggestProfile(in Input) (profile, why string) {
	// High-interest debt dominates everything — pay it down first.
	if hasHighAPRDebt(in) {
		return "debt", "You're carrying high-interest debt, which costs more than most savings earn."
	}
	// A thin emergency fund comes next.
	if covered, known := emergencyMonths(in); known && covered < thinEmergencyMos {
		return "safety", "Your emergency fund is on the thin side, so safety comes first."
	}
	// Otherwise, if there are active goals, lean into them.
	for _, g := range in.Goals {
		if !g.Archived {
			if c, _ := isGoalComplete(g); !c {
				return "goals", "Your debt and buffer look healthy, so you can push toward your goals."
			}
		}
	}
	return "balanced", "Your finances look steady, so a balanced mix works well."
}

// hasHighAPRDebt reports whether any non-archived liability carries a high APR
// and a non-zero balance.
func hasHighAPRDebt(in Input) bool {
	for _, a := range in.Accounts {
		if a.Archived || a.Class != domain.ClassLiability || a.InterestRateAPR < highAPRThreshold {
			continue
		}
		bal, err := ledger.Balance(a, in.Transactions)
		if err == nil && bal.Amount != 0 {
			return true
		}
	}
	return false
}

// emergencyMonths returns how many months of essentials a named emergency goal
// covers, and whether such a goal + spend baseline exist to judge it.
func emergencyMonths(in Input) (months float64, known bool) {
	g, ok := emergencyGoal(in.Goals)
	if !ok {
		return 0, false
	}
	essentials := in.avgMonthlyExpenseBase()
	if essentials <= 0 {
		return 0, false
	}
	current := in.toBaseMinor(g.CurrentAmount.Amount, g.CurrentAmount.Currency)
	return float64(current) / float64(essentials), true
}

// isGoalComplete is a thin wrapper so allocate.go doesn't import goals directly
// for a single call (goals is already imported by goals.go in this package).
func isGoalComplete(g domain.Goal) (bool, error) {
	if g.TargetAmount.Currency != g.CurrentAmount.Currency {
		return false, nil
	}
	return g.CurrentAmount.Amount >= g.TargetAmount.Amount, nil
}
