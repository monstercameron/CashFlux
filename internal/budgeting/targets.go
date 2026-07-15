package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TargetNeed is the evaluated funding need for a budget's optional target (BG1):
// how much of the target is already funded and how much is still needed to fund
// this period. All amounts are in the status's normalized currency.
type TargetNeed struct {
	// Kind echoes the budget's target kind (domain.TargetNone when there is no target).
	Kind domain.TargetKind
	// Target is the target level being funded toward — the refill ceiling, the
	// per-period set-aside amount, or the by-date lump sum. Zero for TargetNone.
	Target money.Money
	// Funded is the amount currently available toward the target this period —
	// the budget's remaining (rollover-aware) balance. Zero for TargetNone.
	Funded money.Money
	// Needed is the amount still needed to fund the target this period, never
	// negative. Zero when the target is already met (or there is no target).
	Needed money.Money
}

// clampNonNeg returns m when it is positive, otherwise a zero of the same currency.
func clampNonNeg(m money.Money) money.Money {
	if m.IsNegative() {
		return money.Zero(m.Currency)
	}
	return m
}

// Needed computes how much is still needed to fund a budget's target this period,
// given the budget's evaluated Status (its Remaining is the rollover-aware funded
// balance) and the reference date `from`.
//
// The three shapes:
//   - TargetRefillUpTo: TargetAmount minus the funded balance, floored at zero.
//   - TargetSetAside:   the fixed TargetAmount every period, regardless of balance.
//   - TargetByDate:     the monthly contribution to reach TargetAmount by TargetDate.
//     A by-date target delegates accumulation to a linked goal (LinkedGoalID): when
//     the budget links a goal, pass that goal's pace via (linkedMonthly, hasLinked=true)
//     and it is used verbatim. Without a linked goal the pace is derived inline from
//     the remaining-to-target and whole months until TargetDate (rounding a partial
//     month up, minimum one), mirroring goals.MonthlyNeeded.
//
// A budget with no target (TargetNone) yields a zero need.
func Needed(budget domain.Budget, status Status, from time.Time, linkedMonthly money.Money, hasLinked bool) TargetNeed {
	cur := status.Remaining.Currency
	if cur == "" {
		cur = budget.TargetAmount.Currency
	}
	target := money.New(budget.TargetAmount.Amount, cur)
	funded := clampNonNeg(money.New(status.Remaining.Amount, cur))

	need := TargetNeed{Kind: budget.TargetKind, Target: target, Funded: funded, Needed: money.Zero(cur)}
	if !budget.HasTarget() {
		need.Target = money.Zero(cur)
		need.Funded = money.Zero(cur)
		return need
	}

	switch budget.TargetKind {
	case domain.TargetRefillUpTo:
		diff, err := target.Sub(funded)
		if err == nil {
			need.Needed = clampNonNeg(diff)
		}
	case domain.TargetSetAside:
		need.Needed = clampNonNeg(target)
	case domain.TargetByDate:
		if budget.LinkedGoalID != "" && hasLinked {
			need.Needed = clampNonNeg(money.New(linkedMonthly.Amount, cur))
			return need
		}
		need.Needed = byDateMonthly(target, funded, from, budget.TargetDate)
	}
	return need
}

// byDateMonthly derives the per-month contribution to accumulate `target` by
// `deadline`, crediting what is already `funded`. It returns zero when the target
// is already met or the deadline is not in the future. Whole months are counted
// from `from`, rounding a partial final month up (minimum one month), matching the
// goals-package MonthlyNeeded convention so a linked and an inline by-date target
// pace the same way.
func byDateMonthly(target, funded money.Money, from, deadline time.Time) money.Money {
	cur := target.Currency
	if deadline.IsZero() || !deadline.After(from) {
		return money.Zero(cur)
	}
	rem, err := target.Sub(funded)
	if err != nil || !rem.IsPositive() {
		return money.Zero(cur)
	}
	months := (deadline.Year()-from.Year())*12 + int(deadline.Month()) - int(from.Month())
	if deadline.Day() > from.Day() {
		months++ // a partial final month still needs a contribution
	}
	if months < 1 {
		months = 1
	}
	per := (rem.Amount + int64(months) - 1) / int64(months) // ceil division
	return money.New(per, cur)
}
