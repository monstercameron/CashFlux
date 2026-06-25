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
