// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountdetail"
	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// errAccountBalanceUnavailable marks a detail figure that cannot be shown because
// the account's exact ledger balance could not be computed.
var errAccountBalanceUnavailable = errors.New("account balance unavailable")

// accountDetailContext shares the list's immutable ledger snapshot with its rows.
// No history/projection work happens until an actual row disclosure is opened.
type accountDetailContext struct {
	app       *appstate.App
	snapshot  accountview.Snapshot
	recurring []domain.Recurring
	asOf      time.Time
	revision  string
	key       string
}

// accountDetailFacts holds the period flow, projections and valuation history
// prepared once per account and ledger revision.
type accountDetailFacts struct {
	Flow                 accountflow.Flow
	Projection           acctproject.Projection // 30-day summary shown by the projected-low line
	ProjectionErr        error
	ForwardProjection    acctproject.Projection // 90-day model used by the forward chart
	ForwardProjectionErr error
	Valuations           []domain.BalanceSnapshot
}

// accountRowDetails is everything one row's details panel shows, prepared on
// demand from the list's shared snapshot.
type accountRowDetails struct {
	accountDetailFacts
	// BalanceValid is false when the account's exact ledger balance is unavailable;
	// the panel then shows no history chart.
	BalanceValid bool
	// ShowFlow is set when the period flow was computed for this row.
	ShowFlow bool
	// Series is the balance series for the selected chart range.
	Series []int64
	// HasRange is set when the account has enough history for the range picker.
	HasRange bool
}

const accountProjectionHorizonDays = 30

var (
	accountHistoryCache          = newViewCache[*accountview.History](32)
	accountSeriesCache           = newViewCache[[]int64](48)
	accountDetailFactsCache      = newViewCache[*accountDetailFacts](32)
	accountProjectionInputsCache = newViewCache[accountview.ProjectionInputs](1)
)

// newAccountDetailContext dates the details at now. The prepared facts are day-granular: the period
// flow, the projections and the series select by calendar day, and a recurring bill due at any time
// today is in today's window whatever the time of day. The key therefore follows the calendar day
// in now's time zone, so every render within a day shares the facts and the next day starts fresh.
func newAccountDetailContext(app *appstate.App, snapshot accountview.Snapshot, rates currency.Rates, base string, now time.Time) *accountDetailContext {
	revision := revKey(app)
	return &accountDetailContext{app: app, snapshot: snapshot, recurring: app.Recurring(), asOf: now, revision: revision,
		key: fmt.Sprintf("%s|rates=%#v|base=%q|day=%s|zone=%q", revision, rates, base, dateutil.DayStart(now).Format(time.RFC3339), now.Location().String())}
}

func (c *accountDetailContext) rowKey(accountID string) string {
	return c.revision + "|account=" + strconv.Quote(accountID)
}

func (c *accountDetailContext) facts(account domain.Account) *accountDetailFacts {
	return memoByRev(accountDetailFactsCache, c.key+"|account="+strconv.Quote(account.ID), func() *accountDetailFacts {
		start, end := dateutil.MonthRange(c.asOf)
		facts := &accountDetailFacts{Flow: accountflow.PeriodFlow(account, c.snapshot.Transactions[account.ID], start, end)}
		// Only cash accounts have a projection; liabilities and valuation assets
		// show none.
		_, validBalance := c.snapshot.Balances[account.ID]
		if account.ShowsCashCurve() {
			if !validBalance {
				facts.ProjectionErr = errAccountBalanceUnavailable
				facts.ForwardProjectionErr = facts.ProjectionErr
			} else {
				inputs := memoByRev(accountProjectionInputsCache, c.revision, func() accountview.ProjectionInputs {
					return accountview.PrepareProjectionInputs(c.recurring)
				})
				balance := c.snapshot.Balances[account.ID]
				first, second := inputs.ProjectPairFor(account, balance, c.asOf, accountProjectionHorizonDays, forwardHorizonDays)
				facts.Projection, facts.ProjectionErr = first.Projection, first.Err
				facts.ForwardProjection, facts.ForwardProjectionErr = second.Projection, second.Err
			}
		}
		if account.Type.IsValuation() {
			facts.Valuations = c.app.BalanceHistory(account.ID)
		}
		return facts
	})
}

// details prepares the details panel content for one account and chart range.
func (c *accountDetailContext) details(account domain.Account, selectedRange string) accountRowDetails {
	series, hasRange := c.series(account, selectedRange)
	_, valid := c.snapshot.Balances[account.ID]
	return accountRowDetails{accountDetailFacts: *c.facts(account), BalanceValid: valid, ShowFlow: true, Series: series, HasRange: hasRange}
}

// series returns the balance series for the selected chart range and whether the
// account has enough history to offer the range picker. An account whose exact
// balance is unavailable has no series: the history engine keeps raw minor-unit
// arithmetic, so its output must not be shown as a plausible cash curve.
func (c *accountDetailContext) series(account domain.Account, selected string) (series []int64, hasRange bool) {
	if _, valid := c.snapshot.Balances[account.ID]; !valid {
		return nil, false
	}
	window := accountdetail.SelectWindow(selected, c.asOf, c.snapshot.Earliest[account.ID])
	rowKey := c.rowKey(account.ID)
	// History and series use native currency only; rates and presentation preferences
	// do not affect their values. The account and ledger fields are revision-bound.
	key := rowKey + "|day=" + dateutil.DayStart(c.asOf).Format(time.RFC3339) + "|zone=" + strconv.Quote(c.asOf.Location().String()) + "|days=" + strconv.Itoa(window.Days)
	series = memoByRev(accountSeriesCache, key, func() []int64 {
		history := memoByRev(accountHistoryCache, rowKey, func() *accountview.History {
			return accountview.PrepareHistory(account, c.snapshot.Transactions[account.ID])
		})
		return history.Series(c.asOf, window.Days)
	})
	return series, window.HasRange
}
