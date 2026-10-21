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
}

// PrepareHistory copies only the account's dated amounts from a shared index.
// Like accountflow.BalanceSeries, it retains legacy unchecked history arithmetic;
// financial UI consumers must gate it on an independently valid ledger balance.
// PrepareHistory copies and indexes an account's rows once. Hosts must verify
// the exact ledger balance before displaying its legacy raw-minor-unit series.
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
		h.sums[i+1] = h.sums[i] + point.amount
	}
	return h
}

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

// PrepareProjectionInputs copies recurring rows into an immutable account index.
// PrepareProjectionInputs copies nonzero recurring rows into an account index.
func PrepareProjectionInputs(recurring []domain.Recurring) ProjectionInputs {
	p := ProjectionInputs{byAccount: make(map[string][]domain.Recurring)}
	for _, row := range recurring {
		if !row.Amount.IsZero() {
			p.byAccount[row.AccountID] = append(p.byAccount[row.AccountID], row)
		}
	}
	return p
}

// Project expands only this account's recurring occurrences. The exact asOf time
// is retained when selecting occurrences, matching App.ProjectAccount even for
// imported intraday dates and non-advancing cadences.
func (p ProjectionInputs) Project(accountID string, balance money.Money, asOf time.Time, horizonDays int) (acctproject.Projection, error) {
	drivers := p.occurrences(accountID, asOf, horizonDays)
	projection, err := projectWindow(balance, asOf, drivers, horizonDays)
	return projection, err
}

// ProjectPair expands an account's recurring occurrences once through the longer
// requested horizon, then filters each window against its exact timestamp boundary
// before acctproject canonicalizes dates to calendar days. This keeps intraday
// occurrences at the end of a shorter window from leaking in from a longer one.
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

func (p ProjectionInputs) occurrences(accountID string, asOf time.Time, horizonDays int) []projectionOccurrence {
	if horizonDays <= 0 {
		return nil
	}
	var drivers []projectionOccurrence
	end := asOf.AddDate(0, 0, horizonDays)
	for _, row := range p.byAccount[accountID] {
		due := row.NextDue
		for i := 0; i < 400 && !due.After(end); i++ {
			if !due.Before(asOf) {
				drivers = append(drivers, projectionOccurrence{
					Driver:   acctproject.Driver{Label: row.Label, Date: due, Amount: row.Amount.Amount},
					Currency: row.Amount.Currency,
				})
			}
			next := row.Cadence.Next(due)
			if !next.After(due) {
				break
			}
			due = next
		}
	}
	return drivers
}

func projectWindow(start money.Money, asOf time.Time, drivers []projectionOccurrence, horizonDays int) (acctproject.Projection, error) {
	if horizonDays <= 0 {
		return acctproject.Project(start.Amount, asOf, nil, horizonDays)
	}
	end := asOf.AddDate(0, 0, horizonDays)
	withinWindow := make([]acctproject.Driver, 0, len(drivers))
	for _, driver := range drivers {
		if !driver.Driver.Date.Before(asOf) && !driver.Driver.Date.After(end) {
			if driver.Currency != start.Currency {
				return acctproject.Projection{}, fmt.Errorf("%w: recurring %q is %s, account is %s", acctproject.ErrCurrencyMismatch, driver.Driver.Label, driver.Currency, start.Currency)
			}
			withinWindow = append(withinWindow, driver.Driver)
		}
	}
	return acctproject.Project(start.Amount, asOf, withinWindow, horizonDays)
}
