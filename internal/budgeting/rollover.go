// SPDX-License-Identifier: MIT

package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
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

// CapCarryover clamps a carried-forward rollover balance to at most
// capPeriods × limit of surplus (BG5). capPeriods <= 0 means uncapped — the carry
// passes through unchanged, preserving the historical behavior. A non-positive
// carry (envelope debt) is never clamped: the cap limits an accumulating cushion,
// not a deficit, so "Dining starts the month down $32" still shows in full. The
// clamp keeps carry's currency.
func CapCarryover(carry, limit money.Money, capPeriods int) money.Money {
	if capPeriods <= 0 || carry.Amount <= 0 {
		return carry
	}
	ceiling := limit.Amount * int64(capPeriods)
	if carry.Amount > ceiling {
		return money.New(ceiling, carry.Currency)
	}
	return carry
}

// CappedCarryover advances one rollover period like Carryover, but first clamps
// the incoming surplus to the budget's RolloverCapPeriods ceiling (BG5). It is the
// single-step recurrence a capped rollover budget uses: feed last period's
// remaining and this period's limit to get the funds available, with any surplus
// beyond the cap dropped so it can't compound. prevRemaining and limit must share
// a currency.
func CappedCarryover(prevRemaining, limit money.Money, capPeriods int) (money.Money, error) {
	capped := CapCarryover(prevRemaining, limit, capPeriods)
	return capped.Add(limit)
}

// PreviousPeriodRange returns the budget period immediately before the period
// containing ref. It keeps rollover UI and tests on the same period math as
// Evaluate/EnvelopeAvailable.
func PreviousPeriodRange(period domain.Period, ref time.Time, weekStart time.Weekday) (start, end time.Time) {
	return PreviousPeriodRangeAnchored(period, ref, weekStart, time.Time{})
}

// PreviousPeriodRangeAnchored returns the period immediately before the
// anchored period containing ref. The anchor applies to biweekly periods.
func PreviousPeriodRangeAnchored(period domain.Period, ref time.Time, weekStart time.Weekday, anchor time.Time) (start, end time.Time) {
	curStart, _ := PeriodRangeAnchored(period, ref, weekStart, anchor)
	return PeriodRangeAnchored(period, curStart.Add(-time.Nanosecond), weekStart, anchor)
}

// PeriodKey returns the stable UTC calendar-date key for the period containing
// ref. Persisted boosts and recurring-period markers should all use this key.
func PeriodKey(period domain.Period, ref time.Time, anchor time.Time) string {
	return PeriodKeyWithWeekStart(period, ref, anchor, time.Monday)
}

// PeriodKeyWithWeekStart returns a stable UTC calendar-date key for the period
// containing ref, honoring the configured first day of the week.
func PeriodKeyWithWeekStart(period domain.Period, ref, anchor time.Time, weekStart time.Weekday) string {
	start, _ := PeriodRangeAnchored(period, ref, weekStart, anchor)
	return dateutil.FormatDate(start)
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
