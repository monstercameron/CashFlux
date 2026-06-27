// SPDX-License-Identifier: MIT

package goals

import (
	"errors"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ErrNegativeSpend is returned by DrawDownFund when spendMinor is negative.
var ErrNegativeSpend = errors.New("goals: spendMinor must be non-negative")

// DrawDownFund reduces g.CurrentAmount by spendMinor (a positive expense
// magnitude in the goal's currency) and returns the updated Goal. The balance
// is floored at zero — it never goes negative — so a draw-down larger than the
// current balance simply empties the fund. The input Goal is not mutated.
//
// spendMinor must be ≥ 0. A currency mismatch between spendMinor and the
// goal's CurrentAmount is detected and returned as an error.
func DrawDownFund(g domain.Goal, spendMinor int64) (domain.Goal, error) {
	if spendMinor < 0 {
		return domain.Goal{}, ErrNegativeSpend
	}

	spend := money.New(spendMinor, g.CurrentAmount.Currency)

	// Detect a currency mismatch by comparing against CurrentAmount's currency.
	// (money.Sub would catch it too, but we get a cleaner error message here.)
	if spend.Currency != g.CurrentAmount.Currency {
		return domain.Goal{}, fmt.Errorf("goals: spend currency %q != fund currency %q",
			spend.Currency, g.CurrentAmount.Currency)
	}

	after, err := g.CurrentAmount.Sub(spend)
	if err != nil {
		return domain.Goal{}, fmt.Errorf("goals: draw-down subtraction: %w", err)
	}

	if after.IsNegative() {
		after = money.Zero(g.CurrentAmount.Currency)
	}

	g.CurrentAmount = after
	return g, nil
}

// fundAccrualPeriodKey returns the UTC year-month key used to guard monthly
// accruals, e.g. "2026-06". It is the canonical period marker stored in
// Goal.Custom["fundAccrualPeriod"].
func fundAccrualPeriodKey(now time.Time) string {
	return now.UTC().Format("2006-01")
}

// FundAccrualDue reports whether a sinking-fund goal is due for its monthly
// auto-accrual as of now, and how many minor units should be credited.
//
// Returns due=false when any of the following hold:
//   - the goal is not a sinking fund (IsSinkingFund is false)
//   - the goal is archived
//   - CurrentAmount is already at or above TargetAmount (fully funded)
//   - the goal has already been accrued this calendar month (Custom["fundAccrualPeriod"] matches the current UTC year-month)
//   - FundSetAsideMinor returns 0 (no TargetDate, deadline passed, or nothing remains)
//
// When due=true, amountMinor is the lesser of FundSetAsideMinor and the
// remaining balance to target, ensuring the fund never exceeds TargetAmount.
func FundAccrualDue(g domain.Goal, now time.Time) (due bool, amountMinor int64) {
	if !g.IsSinkingFund || g.Archived {
		return false, 0
	}

	// Already fully funded?
	if g.CurrentAmount.Amount >= g.TargetAmount.Amount && g.TargetAmount.Amount > 0 {
		return false, 0
	}

	// Already accrued this month?
	periodKey := fundAccrualPeriodKey(now)
	if marker, ok := g.Custom["fundAccrualPeriod"]; ok {
		if s, ok := marker.(string); ok && s == periodKey {
			return false, 0
		}
	}

	// How much is the monthly set-aside?
	setAside := FundSetAsideMinor(g, now)
	if setAside <= 0 {
		return false, 0
	}

	// Cap at the remaining balance so we never overshoot.
	remaining := g.TargetAmount.Amount - g.CurrentAmount.Amount
	if remaining <= 0 {
		return false, 0
	}
	if setAside > remaining {
		setAside = remaining
	}

	return true, setAside
}

// FundSetAsideMinor returns the monthly amount (in minor units of the goal's
// currency) that must be set aside to reach the goal by its TargetDate.
//
// It uses the same months calculation as MonthlyNeeded so the two functions
// agree on the schedule: whole months from now to TargetDate, rounding a
// partial final month up, minimum one. The per-month figure is then computed
// by budgeting.SinkingFundContribution (ceiling division so the full remaining
// balance is covered on or before the deadline).
//
// Returns 0 when:
//   - the goal has no TargetDate, or
//   - the TargetDate is not after now (deadline passed or is today), or
//   - nothing remains (goal is already funded).
func FundSetAsideMinor(g domain.Goal, now time.Time) int64 {
	if g.TargetDate.IsZero() || !g.TargetDate.After(now) {
		return 0
	}

	rem, err := Remaining(g)
	if err != nil || rem.IsZero() {
		return 0
	}

	// Mirror the months formula used by MonthlyNeeded exactly.
	months := (g.TargetDate.Year()-now.Year())*12 + int(g.TargetDate.Month()) - int(now.Month())
	if g.TargetDate.Day() > now.Day() {
		months++ // partial final month still counts
	}
	if months < 1 {
		months = 1
	}

	contribution := budgeting.SinkingFundContribution(rem, months)
	return contribution.Amount
}
