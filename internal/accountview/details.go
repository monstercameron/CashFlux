// SPDX-License-Identifier: MIT

package accountview

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

type historyPoint struct {
	date   time.Time
	id     string
	amount int64
}

// History prepares one account's immutable chronological balance index. Callers
// can request individual chart windows without filtering and sorting again.
// Its arithmetic preserves accountflow.BalanceSeries, including imported rows
// excluded from reports and transactions stored in a different currency.
type History struct {
	points []historyPoint
	sums   []int64 // opening balance, then the balance after each ordered point
	// overflowed is set when a running balance did not fit in int64 and wrapped.
	overflowed bool
}

// PrepareHistory copies and indexes an account's dated amounts once. Like
// accountflow.BalanceSeries, it keeps the legacy raw-minor-unit arithmetic, which
// wraps on overflow; Overflowed reports when that happened, and hosts must also
// verify the exact ledger balance before displaying its series.
func PrepareHistory(account domain.Account, transactions []domain.Transaction) *History {
	h := &History{points: make([]historyPoint, 0, len(transactions))}
	for _, txn := range transactions {
		if txn.AccountID == account.ID {
			h.points = append(h.points, historyPoint{txn.Date, txn.ID, txn.Amount.Amount})
		}
	}
	sort.SliceStable(h.points, func(i, j int) bool {
		if h.points[i].date.Equal(h.points[j].date) {
			return h.points[i].id < h.points[j].id
		}
		return h.points[i].date.Before(h.points[j].date)
	})
	h.sums = make([]int64, len(h.points)+1)
	h.sums[0] = account.OpeningBalance.Amount
	for i, point := range h.points {
		next, err := money.Add(h.sums[i], point.amount)
		if err != nil {
			h.overflowed = true
			next = h.sums[i] + point.amount
		}
		h.sums[i+1] = next
	}
	return h
}

// Overflowed reports whether any running balance exceeded the int64 range. The
// series values after that point are wrapped and must not be displayed.
func (h *History) Overflowed() bool { return h != nil && h.overflowed }

// Series returns the same end-of-day points as accountflow.BalanceSeries. Only
// the requested window is allocated; the prepared index is never modified.
func (h *History) Series(asOf time.Time, days int) []int64 {
	if days <= 0 {
		return nil
	}
	first := dateutil.DayStart(asOf).AddDate(0, 0, -(days - 1))
	out := make([]int64, days)
	for d := range out {
		end := first.AddDate(0, 0, d+1)
		i := sort.Search(len(h.points), func(i int) bool { return !h.points[i].date.Before(end) })
		out[d] = h.sums[i]
	}
	return out
}

// ProjectionInputs indexes recurring rows once for an entire account snapshot.
// It preserves ProjectAccount's existing occurrence and raw account-currency
// conventions; the prepared exact ledger balance is supplied separately.
type ProjectionInputs struct{ byAccount map[string][]domain.Recurring }

// ProjectionWindow holds one independently checked recurring projection.
// Err is nil for both valid projections and valid no-flow windows; Projection.Known
// distinguishes those cases from a forecast with drivers.
type ProjectionWindow struct {
	Projection acctproject.Projection
	Err        error
}

type projectionOccurrence struct {
	acctproject.Driver
	Currency string
}

// PrepareProjectionInputs copies the recurring rows that can move an account's
// cash into an immutable account index. Zero-amount rows, paused rows and
// services are skipped, as services.MonthlyTotal and safespend skip them: a
// service is never auto-posted and stays out of forecasts.
func PrepareProjectionInputs(recurring []domain.Recurring) ProjectionInputs {
	p := ProjectionInputs{byAccount: make(map[string][]domain.Recurring)}
	for _, row := range recurring {
		if !row.Amount.IsZero() && row.Active() && !row.Service {
			p.byAccount[row.AccountID] = append(p.byAccount[row.AccountID], row)
		}
	}
	return p
}

// Project expands only this account's recurring occurrences and applies them to
// balance exactly as given; ProjectFor normalises a liability's stored balance first. Occurrences are
// selected by calendar day, so a bill due at any time today is in the window
// whatever time of day asOf carries, and a bill whose next due date has already
// passed lands today, as acctproject.Project treats an overdue flow.
func (p ProjectionInputs) Project(accountID string, balance money.Money, asOf time.Time, horizonDays int) (acctproject.Projection, error) {
	drivers := p.occurrences(accountID, asOf, horizonDays)
	return projectWindow(balance, asOf, drivers, horizonDays)
}

// ProjectFor projects an account in the stated convention (positive is money you
// have, negative is money you owe): the stored balance of a liability is
// normalised through reconcile.Stated with the account's own LiabilitySign
// before the recurring flows are applied. A card holding +500 owed therefore
// starts at -500 and a -600 charge takes it to -1,100, where Project would start
// from the raw +500. An asset projects exactly as Project does.
func (p ProjectionInputs) ProjectFor(account domain.Account, balance money.Money, asOf time.Time, horizonDays int) (acctproject.Projection, error) {
	return p.Project(account.ID, statedStart(account, balance), asOf, horizonDays)
}

// ProjectPairFor is ProjectPair with the starting balance normalised as in
// ProjectFor.
func (p ProjectionInputs) ProjectPairFor(account domain.Account, balance money.Money, asOf time.Time, firstHorizonDays, secondHorizonDays int) (ProjectionWindow, ProjectionWindow) {
	return p.ProjectPair(account.ID, statedStart(account, balance), asOf, firstHorizonDays, secondHorizonDays)
}

func statedStart(account domain.Account, balance money.Money) money.Money {
	balance.Amount = reconcile.Stated(balance.Amount, reconcile.ConventionFor(account))
	return balance
}

// ProjectPair expands an account's recurring occurrences once through the longer
// requested horizon, then filters each window against its own last calendar day
// so occurrences past a shorter window's end do not leak in from the longer one.
// The two returned projections follow the order of the requested horizons.
func (p ProjectionInputs) ProjectPair(accountID string, balance money.Money, asOf time.Time, firstHorizonDays, secondHorizonDays int) (ProjectionWindow, ProjectionWindow) {
	maxHorizon := max(firstHorizonDays, secondHorizonDays)
	drivers := p.occurrences(accountID, asOf, maxHorizon)
	firstProjection, firstErr := projectWindow(balance, asOf, drivers, firstHorizonDays)
	secondProjection, secondErr := projectWindow(balance, asOf, drivers, secondHorizonDays)
	first := ProjectionWindow{Projection: firstProjection, Err: firstErr}
	second := ProjectionWindow{Projection: secondProjection, Err: secondErr}
	return first, second
}

// secondsPerDay converts the difference of two UTC-midnight Unix times to days.
// Subtracting Unix seconds, unlike time.Time.Sub, does not saturate for dates
// more than about 292 years apart, such as a zero NextDue.
const secondsPerDay = 24 * 60 * 60

// maxCadenceSteps bounds the stepping loops so a misconfigured cadence cannot spin.
const maxCadenceSteps = 5000

func (p ProjectionInputs) occurrences(accountID string, asOf time.Time, horizonDays int) []projectionOccurrence {
	if horizonDays <= 0 {
		return nil
	}
	startDay := dateutil.DayStart(asOf)
	endDay := startDay.AddDate(0, 0, horizonDays)
	var drivers []projectionOccurrence
	for _, row := range p.byAccount[accountID] {
		for _, due := range occurrenceDates(row, startDay, endDay) {
			drivers = append(drivers, projectionOccurrence{
				Driver:   acctproject.Driver{Label: row.Label, Date: due, Amount: row.Amount.Amount},
				Currency: row.Amount.Currency,
			})
		}
	}
	return drivers
}

// occurrenceDates lists the dates a recurring row falls due in [startDay, endDay],
// both UTC-midnight calendar days. A NextDue before startDay is overdue and is
// listed once, at its own date, which acctproject lands on the start day; the
// cadence is then fast-forwarded to its first occurrence on or after startDay, so
// a bill overdue by any length of time still projects its future occurrences.
// Month-based cadences step from the original day of the month, clamped per
// month, so a bill due on the 31st returns to the 31st after a short month.
func occurrenceDates(row domain.Recurring, startDay, endDay time.Time) []time.Time {
	due := row.NextDue
	anchorDay := due.Day()
	var out []time.Time
	if dateutil.DayStart(due).Before(startDay) {
		out = append(out, due)
		var ok bool
		if due, ok = firstOnOrAfter(row.Cadence, due, anchorDay, startDay); !ok {
			return out
		}
	}
	for i := 0; i < maxCadenceSteps && !dateutil.DayStart(due).After(endDay); i++ {
		out = append(out, due)
		next := stepCadence(row.Cadence, due, anchorDay)
		if !next.After(due) {
			break
		}
		due = next
	}
	return out
}

// firstOnOrAfter returns the first occurrence after due whose calendar day is on
// or after startDay. Fixed-length cadences jump there arithmetically; the rest
// step. ok is false when the cadence never reaches startDay.
func firstOnOrAfter(c domain.RecurringCadence, due time.Time, anchorDay int, startDay time.Time) (time.Time, bool) {
	if length := fixedCadenceDays(c); length > 0 {
		gap := int((startDay.Unix() - dateutil.DayStart(due).Unix()) / secondsPerDay)
		steps := (gap + length - 1) / length
		return due.AddDate(0, 0, steps*length), true
	}
	for i := 0; i < maxCadenceSteps; i++ {
		next := stepCadence(c, due, anchorDay)
		if !next.After(due) {
			return time.Time{}, false
		}
		due = next
		if !dateutil.DayStart(due).Before(startDay) {
			return due, true
		}
	}
	return time.Time{}, false
}

// fixedCadenceDays is the constant length in days of a day-based cadence, or 0
// for the cadences that follow the calendar.
func fixedCadenceDays(c domain.RecurringCadence) int {
	switch c {
	case domain.CadenceDaily:
		return 1
	case domain.CadenceWeekly:
		return 7
	case domain.CadenceBiweekly:
		return 14
	}
	return 0
}

// stepCadence advances one cadence step. Month-based cadences return to anchorDay,
// clamped to the destination month's last day.
func stepCadence(c domain.RecurringCadence, due time.Time, anchorDay int) time.Time {
	next := c.Next(due)
	if fixedCadenceDays(c) > 0 || c == domain.CadenceSemimonthly {
		return next
	}
	last := time.Date(next.Year(), next.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(next.Year(), next.Month(), min(anchorDay, last), next.Hour(), next.Minute(), next.Second(), next.Nanosecond(), next.Location())
}

func projectWindow(start money.Money, asOf time.Time, drivers []projectionOccurrence, horizonDays int) (acctproject.Projection, error) {
	if horizonDays <= 0 {
		return acctproject.Project(start.Amount, asOf, nil, horizonDays)
	}
	endDay := dateutil.DayStart(asOf).AddDate(0, 0, horizonDays)
	withinWindow := make([]acctproject.Driver, 0, len(drivers))
	for _, driver := range drivers {
		if !dateutil.DayStart(driver.Driver.Date).After(endDay) {
			if driver.Currency != start.Currency {
				return acctproject.Projection{}, fmt.Errorf("%w: recurring %q is %s, account is %s", money.ErrCurrencyMismatch, driver.Driver.Label, driver.Currency, start.Currency)
			}
			withinWindow = append(withinWindow, driver.Driver)
		}
	}
	return acctproject.Project(start.Amount, asOf, withinWindow, horizonDays)
}
