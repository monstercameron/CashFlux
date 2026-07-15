// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/emergencyfund"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-G21", g21EmergencyResize)
}

// essentialDriftPct is the drift, in percent, at which the derived essential
// month is considered to have moved enough from the target's basis to re-suggest
// a resize.
const essentialDriftPct = 10

// EssentialBasis derives the household's ESSENTIAL month (base-currency minor
// units) from the input: fixed recurring commitments plus the trailing average
// of essential-classified (fixed / non-monthly, NOT flex) categorized spending.
// It is exported so the goals surface and the essential_monthly atom size the
// fund off exactly the same honest figure the re-suggest flag uses.
//
// Fixed component: every recurring EXPENSE, normalized to a monthly cost via
// domain.Recurring.MonthlyEquivalent. Essential-spend component: a trailing
// per-month average (over trailingMonths whole months) of spend in categories
// classed ClassFixed or ClassNonMonthly — discretionary ClassFlex spend is
// deliberately excluded, since a bare-bones emergency month wouldn't fund it.
func (in Input) EssentialBasis() emergencyfund.Basis {
	var fixed int64
	for _, r := range in.Recurring {
		if r.Amount.Amount >= 0 {
			continue // income / zero — only commitments count
		}
		me := r.MonthlyEquivalent() // signed monthly-equivalent minor units
		if me > 0 {
			me = -me
		}
		fixed += in.toBaseMinor(-me, r.Amount.Currency)
	}

	// Classify categories so we can pick out essential (non-flex) spend.
	classOf := map[string]domain.CategoryClass{}
	for _, c := range in.Categories {
		classOf[c.ID] = c.ClassOf()
	}

	curStart := dateutil.MonthStart(in.Now)
	var essentialSpend int64
	for k := 1; k <= trailingMonths; k++ {
		s := dateutil.AddMonths(curStart, -k)
		e := dateutil.AddMonths(curStart, -k+1)
		for _, t := range in.Transactions {
			if t.IsTransfer() || !t.Amount.IsNegative() || t.Date.Before(s) || !t.Date.Before(e) {
				continue
			}
			cls, ok := classOf[t.CategoryID]
			if !ok {
				cls = domain.ClassFlex // unclassified defaults to discretionary
			}
			if cls == domain.ClassFlex {
				continue // discretionary is not part of a bare essential month
			}
			essentialSpend += in.toBaseMinor(-t.Amount.Amount, t.Amount.Currency)
		}
	}
	essentialSpend /= trailingMonths

	return emergencyfund.Basis{
		FixedMonthlyMinor:          fixed,
		EssentialSpendMonthlyMinor: essentialSpend,
		Currency:                   in.Base,
	}
}

// SMART-G21 — Emergency-fund resize (GL3 re-suggest). When the household has an
// emergency-fund goal whose target was derived from a stored essential-month
// basis, and the freshly derived essential month has drifted more than
// essentialDriftPct from that basis, this re-suggests updating the target. It is
// a preview-approve nudge (never auto-applies): the action carries the newly
// derived target so the goal surface can offer a one-tap "set as target". The
// insight key encodes the suggested level (3 or 6 months) so a dismissal is
// scoped to that specific suggestion — a later, different drift re-surfaces.
//
// It stays quiet when the goal was never auto-derived (EssentialBasisMinor == 0),
// since there is no basis to have drifted from — G11/G12 handle the un-sized case.
func g21EmergencyResize(in Input) []smart.Insight {
	g, ok := emergencyGoal(in.Goals)
	if !ok || g.EssentialBasisMinor <= 0 {
		return nil
	}
	basis := in.EssentialBasis()
	derived := basis.EssentialMonthlyMinor()
	if derived <= 0 {
		return nil
	}
	if !emergencyfund.DriftExceeds(g.EssentialBasisMinor, derived, essentialDriftPct) {
		return nil
	}

	// Preserve the horizon the existing target implies (round the stored target /
	// stored basis to the nearest supported level) so the re-suggest keeps the
	// user's chosen 3- or 6-month intent instead of silently switching horizons.
	level := impliedLevel(in.toBaseMinor(g.TargetAmount.Amount, g.TargetAmount.Currency), g.EssentialBasisMinor)
	sizing := emergencyfund.Size(basis)
	newTarget := sizing.TargetMinor(level)

	direction := "up"
	if derived < g.EssentialBasisMinor {
		direction = "down"
	}

	ins := smart.Insight{
		Feature: "SMART-G21",
		Page:    smart.PageGoals,
		Key:     "SMART-G21:" + g.ID + ":" + itoa64(int64(level)),
		Title:   "Your essential month has shifted — resize " + g.Name + "?",
		Detail: "Your essential month is now about " + in.hmoney(derived) + " (it has moved " + direction +
			" from " + in.hmoney(g.EssentialBasisMinor) + "). A " + itoa64(int64(level)) +
			"-month fund would target " + in.hmoney(newTarget) + ". Update the target when you're ready — nothing changes until you do.",
		Severity: smart.SeverityNudge,
	}.WithAmount(in.baseMoney(newTarget)).
		WithAction(smart.Action{
			Kind:        smart.ActionNavigate,
			Label:       "Review emergency fund",
			Route:       "/goals",
			RelatedType: "goal",
			RelatedID:   g.ID,
		})
	return []smart.Insight{ins}
}

// impliedLevel picks the emergency-fund horizon (3 or 6 months) that the stored
// target/basis ratio is closest to, defaulting to six when the basis is unusable.
func impliedLevel(targetMinor, basisMinor int64) emergencyfund.Level {
	if basisMinor <= 0 {
		return emergencyfund.LevelSix
	}
	months := float64(targetMinor) / float64(basisMinor)
	// Midpoint between 3 and 6 is 4.5.
	if months < 4.5 {
		return emergencyfund.LevelThree
	}
	return emergencyfund.LevelSix
}
