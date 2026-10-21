// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/acctseries"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

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

type accountDetailFacts struct {
	Flow              accountflow.Flow
	Projection        acctproject.Projection // 30-day summary shown by the AC13 projected-low line
	ForwardProjection acctproject.Projection // 90-day model used by the forward chart
	Valuations        []domain.BalanceSnapshot
}

const accountProjectionHorizonDays = 30

var (
	accountHistoryCache          = newViewCache[*accountview.History](32)
	accountSeriesCache           = newViewCache[[]int64](48)
	accountDetailFactsCache      = newViewCache[*accountDetailFacts](32)
	accountProjectionInputsCache = newViewCache[accountview.ProjectionInputs](1)
)

func newAccountDetailContext(app *appstate.App, snapshot accountview.Snapshot, rates currency.Rates, base string, now time.Time) *accountDetailContext {
	revision := revKey(app)
	return &accountDetailContext{app: app, snapshot: snapshot, recurring: app.Recurring(), asOf: now, revision: revision,
		key: fmt.Sprintf("%s|rates=%#v|base=%q|asof=%s|zone=%q", revision, rates, base, now.Format(time.RFC3339Nano), now.Location().String())}
}

func (c *accountDetailContext) rowKey(accountID string) string {
	return c.revision + "|account=" + strconv.Quote(accountID)
}

func (c *accountDetailContext) facts(account domain.Account) *accountDetailFacts {
	return memoByRev(accountDetailFactsCache, c.key+"|account="+strconv.Quote(account.ID), func() *accountDetailFacts {
		start, end := dateutil.MonthRange(c.asOf)
		facts := &accountDetailFacts{Flow: accountflow.PeriodFlow(account, c.snapshot.Transactions[account.ID], start, end)}
		// These account kinds never display a cash projection. Their exact balances
		// and valuation histories retain their existing independent semantics.
		_, validBalance := c.snapshot.Balances[account.ID]
		if validBalance && account.Class != domain.ClassLiability && !isValuationType(account.Type) {
			inputs := memoByRev(accountProjectionInputsCache, c.revision, func() accountview.ProjectionInputs {
				return accountview.PrepareProjectionInputs(c.recurring)
			})
			balance := c.snapshot.Balances[account.ID]
			facts.Projection, facts.ForwardProjection = inputs.ProjectPair(account.ID, balance, c.asOf, accountProjectionHorizonDays, forwardHorizonDays)
		}
		if isValuationType(account.Type) {
			facts.Valuations = c.app.BalanceHistory(account.ID)
		}
		return facts
	})
}

func (c *accountDetailContext) series(account domain.Account, selected string) (series []int64, hasRange bool) {
	// The historical engine preserves legacy native-amount arithmetic. An invalid
	// exact balance must never be presented as a plausible historical cash curve.
	if _, valid := c.snapshot.Balances[account.ID]; !valid {
		return nil, false
	}
	const allWindowCapDays = 366 * 20
	allDays := acctseries.AllDays(c.asOf, acctseries.Days90, allWindowCapDays, c.snapshot.Earliest[account.ID])
	hasRange = acctseries.HasRange(allDays)
	days := acctseries.Days90
	if hasRange {
		switch selected {
		case "12m":
			days = acctseries.Days12m
		case "all":
			days = allDays
		}
	}
	rowKey := c.rowKey(account.ID)
	// History and series use native currency only; rates/presentation preferences
	// do not affect their values. The account and ledger fields are revision-bound.
	key := rowKey + "|day=" + dateutil.DayStart(c.asOf).Format(time.RFC3339) + "|zone=" + strconv.Quote(c.asOf.Location().String()) + "|days=" + strconv.Itoa(days)
	series = memoByRev(accountSeriesCache, key, func() []int64 {
		history := memoByRev(accountHistoryCache, rowKey, func() *accountview.History {
			return accountview.PrepareHistory(account, c.snapshot.Transactions[account.ID])
		})
		return history.Series(c.asOf, days)
	})
	return series, hasRange
}
