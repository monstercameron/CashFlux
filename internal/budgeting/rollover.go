package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Carryover returns the funds available for a period under rollover budgeting
// (B26): any balance carried from the previous period (prevRemaining, which is
// negative when the previous period was overspent) plus this period's limit. It
// is the per-period building block of envelope rollover (D6) and the value a
// "carried over $X" badge reports. Both amounts must share a currency.
//
// Unlike EnvelopeAvailable, which re-derives the running balance from the full
// transaction history, Carryover is the single-step recurrence: feed it last
// period's Status.Remaining and this period's limit to advance one period.
func Carryover(prevRemaining, limit money.Money) (money.Money, error) {
	return prevRemaining.Add(limit)
}

// PreviousPeriodRange returns the budget period immediately before the period
// containing ref. It keeps rollover UI and tests on the same period math as
// Evaluate/EnvelopeAvailable.
func PreviousPeriodRange(period domain.Period, ref time.Time, weekStart time.Weekday) (start, end time.Time) {
	curStart, _ := PeriodRange(period, ref, weekStart)
	return PeriodRange(period, curStart.Add(-time.Nanosecond), weekStart)
}

// SinkingFundContribution returns the amount to set aside each period to reach
// target in exactly periods contributions (a sinking fund: saving steadily
// toward a known future expense). It rounds up — ceiling division — so the goal
// is met on or before the final period rather than falling a few minor units
// short. A non-positive periods means "fund it all now" and returns the whole
// target; a non-positive target returns zero.
func SinkingFundContribution(target money.Money, periods int) money.Money {
	if target.Amount <= 0 {
		return money.Zero(target.Currency)
	}
	if periods <= 0 {
		return target
	}
	per := target.Amount / int64(periods)
	if target.Amount%int64(periods) != 0 {
		per++ // ceil so the full target is reached by the deadline
	}
	return money.New(per, target.Currency)
}

// SinkingFundAccrued returns the balance saved after contributing the per-period
// amount for made periods (contribution × made). The result never exceeds
// target, so the ceiling-rounded final contribution from SinkingFundContribution
// doesn't overshoot the goal. A non-positive made returns zero. It fails on a
// currency mismatch or if the multiplication would overflow int64.
func SinkingFundAccrued(contribution, target money.Money, made int) (money.Money, error) {
	if contribution.Currency != target.Currency {
		return money.Money{}, fmt.Errorf("%w: %q vs %q", money.ErrCurrencyMismatch, contribution.Currency, target.Currency)
	}
	if made <= 0 {
		return money.Zero(contribution.Currency), nil
	}
	accrued := contribution.Amount * int64(made)
	if accrued/int64(made) != contribution.Amount {
		return money.Money{}, fmt.Errorf("%w: %d * %d", money.ErrOverflow, contribution.Amount, made)
	}
	if accrued > target.Amount {
		accrued = target.Amount
	}
	return money.New(accrued, contribution.Currency), nil
}

// SinkingFundProgress returns how far a sinking fund has accrued toward its
// target, as a percent (0–100, capped). A non-positive target reports 100 when
// anything is saved and 0 otherwise.
func SinkingFundProgress(accrued, target money.Money) int {
	p := percent(accrued, target)
	if p > 100 {
		return 100
	}
	if p < 0 {
		return 0
	}
	return p
}
