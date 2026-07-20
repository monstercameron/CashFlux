// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/attribution"
	"github.com/monstercameron/CashFlux/internal/balancesheet"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// This file holds the /networth surface's SHARED render model. Glance and
// Detail are two readings of ONE computation: both views take their figures
// from the same nwsView, which is why no number can disagree between them.
// Pure assembly over the tested engines (ledger, attribution.BuildBridge,
// balancesheet) — no analysis lives here.

// nwsAcctRow is one account's place on the balance sheet: where it stands now
// and how far it moved across the selected window.
type nwsAcctRow struct {
	Acct domain.Account
	// BalanceMinor is the account's own signed balance in base currency.
	BalanceMinor int64
	// SideMinor is the magnitude this account contributes to its SIDE of the
	// sheet (assets add their balance; liabilities add what is owed), so a share
	// can be normalized WITHIN a side rather than against the whole sheet.
	SideMinor int64
	// MoveMinor is the account's signed net-worth contribution over the window.
	MoveMinor int64
	// StartMinor / EndMinor are the account's own balance at each end of the
	// window, so a drilldown can show what it opened and closed at rather than
	// only how far it travelled.
	StartMinor, EndMinor int64
	// FlowMinor / AdjMinor split MoveMinor into money that actually moved
	// through the account and balance the household simply asserted — the
	// difference between "you earned this" and "you re-valued this", which is
	// the single most useful thing a reader can learn about one account.
	FlowMinor, AdjMinor int64
	// AsOf is when this balance was last confirmed (zero = never), and Manual
	// says it was last set by hand rather than derived from transactions.
	AsOf   time.Time
	Manual bool
	Bucket balancesheet.Bucket
}

// nwsView is everything both views render, computed once per render.
type nwsView struct {
	Base string
	Dec  int
	Now  time.Time
	// Rates is the FX table every figure here passed through, kept so a
	// disclosure can describe the conversion without rebuilding it.
	Rates currency.Rates

	// Months is the selected window length in calendar months (1 = this month).
	Months       int
	Since, Until time.Time

	// Snapshot as of now, with the exclusions disclosed rather than silently
	// folded in (ledger.NetWorthExplained).
	Snapshot ledger.NetWorthResult

	// Bridge decomposes the window; its EndMinor is the same net worth the hero
	// prints and its legs sum to it exactly, residual included.
	Bridge attribution.Bridge

	// Points is the monthly composition series across the window, with today
	// appended as the final point so the chart ends where the hero does.
	Points []balancesheet.Point
	// Labels captions Points, one per point (blank where thinned).
	Labels []string

	Health balancesheet.Health
	// CashMinor is the spendable (BucketCash) total; MonthlyExpenseMinor is
	// typical monthly spending over the trailing quarter (0 when unknown).
	CashMinor, MonthlyExpenseMinor int64

	// Accounts holds EVERY non-archived account, biggest side-magnitude first.
	// Nothing is capped here: a view that hides rows must say so itself.
	Accounts []nwsAcctRow
}

// Assets returns the asset rows in display order.
func (v nwsView) Assets() []nwsAcctRow { return v.side(domain.ClassAsset) }

// Liabilities returns the liability rows in display order.
func (v nwsView) Liabilities() []nwsAcctRow { return v.side(domain.ClassLiability) }

func (v nwsView) side(c domain.AccountClass) []nwsAcctRow {
	out := make([]nwsAcctRow, 0, len(v.Accounts))
	for _, r := range v.Accounts {
		if r.Acct.Class == c {
			out = append(out, r)
		}
	}
	return out
}

// Latest is the composition point the hero describes (the final point).
func (v nwsView) Latest() balancesheet.Point {
	if len(v.Points) == 0 {
		return balancesheet.Point{}
	}
	return v.Points[len(v.Points)-1]
}

// Movers returns the accounts that actually moved over the window, largest
// absolute movement first. Every mover is returned — the caller decides how to
// present them, but it may not quietly drop any.
func (v nwsView) Movers() []nwsAcctRow {
	out := make([]nwsAcctRow, 0, len(v.Accounts))
	for _, r := range v.Accounts {
		if r.MoveMinor != 0 {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return absMinor(out[i].MoveMinor) > absMinor(out[j].MoveMinor)
	})
	return out
}

// nwsWindowMonths are the selectable window lengths, shortest first. Zero is
// "all time": the window opens at the household's earliest record rather than a
// fixed number of months back.
var nwsWindowMonths = []int{1, 6, 12, 24, 0}

// nwsAllTime is the sentinel window length meaning "since your records begin".
const nwsAllTime = 0

// computeNwsView builds the shared model for a window of `months` calendar
// months ending now. The window is half-open [Since, Until) in the app's
// canonical convention, so every engine reading it agrees to the cent.
func computeNwsView(app *appstate.App, months int, now time.Time) nwsView {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	accounts := app.Accounts()
	txns := app.Transactions()

	v := nwsView{Base: base, Dec: currency.Decimals(base), Now: now, Months: months, Rates: rates}
	curMonth := dateutil.MonthStart(now)
	if months == nwsAllTime {
		v.Since = dateutil.MonthStart(nwsEarliestRecord(accounts, txns, now))
	} else {
		v.Since = dateutil.AddMonths(curMonth, -(months - 1))
	}
	// Until is tomorrow so "strictly before the cutoff" includes everything
	// posted today — the hero must not lag the ledger by a day.
	v.Until = dateutil.DayStart(now).AddDate(0, 0, 1)

	v.Snapshot, _ = ledger.NetWorthExplained(accounts, txns, rates)

	adjDesc := uistate.T("accounts.balanceAdjustment")
	isAdj := func(t domain.Transaction) bool { return t.Desc == adjDesc }
	v.Bridge, _ = attribution.BuildBridge(attribution.Input{
		Accounts: accounts, Txns: txns, Rates: rates,
		Since: v.Since, Until: v.Until, IsAdjustment: isAdj,
	})

	// Composition series: one point per month boundary in the window, plus today.
	// An all-time window is thinned to a sane number of points — a household with
	// ten years of records does not need 120 of them to see a shape.
	span := months
	if span == nwsAllTime {
		span = nwsMonthsBetween(v.Since, curMonth) + 1
	}
	step := 1
	if span > 36 {
		step = (span + 35) / 36
	}
	cutoffs := make([]time.Time, 0, span/step+2)
	for k := 0; k <= span-1; k += step {
		cutoffs = append(cutoffs, dateutil.AddMonths(v.Since, k))
	}
	cutoffs = append(cutoffs, v.Until)
	v.Points, _ = balancesheet.Series(accounts, txns, cutoffs, rates)
	v.Labels = nwsLabels(cutoffs, span)

	// Per-account standing and movement over the same window.
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		bal, err := ledger.Balance(a, txns)
		if err != nil {
			continue
		}
		conv, err := rates.Convert(bal, base)
		if err != nil {
			continue // already disclosed by NetWorthExplained
		}
		row := nwsAcctRow{Acct: a, BalanceMinor: conv.Amount, Bucket: balancesheet.BucketOf(a), AsOf: a.BalanceAsOf}
		row.SideMinor = absMinor(conv.Amount)
		row.MoveMinor, row.StartMinor, row.EndMinor, row.FlowMinor, row.AdjMinor =
			nwsAccountWindow(a, txns, rates, base, v.Since, v.Until, isAdj)
		if kind, _ := ledger.BalanceProvenance(a.ID, txns, isAdj); kind == ledger.ProvenanceAdjusted || kind == ledger.ProvenanceOpening {
			row.Manual = true
		}
		v.Accounts = append(v.Accounts, row)
	}
	sort.SliceStable(v.Accounts, func(i, j int) bool {
		return v.Accounts[i].SideMinor > v.Accounts[j].SideMinor
	})

	latest := v.Latest()
	v.CashMinor = latest.Assets[balancesheet.BucketCash]
	v.MonthlyExpenseMinor = nwsTypicalMonthlyExpense(txns, rates, now)
	v.Health = balancesheet.Assess(latest.AssetsMinor, latest.LiabilitiesMinor, v.CashMinor, v.MonthlyExpenseMinor)
	return v
}

// nwsEarliestRecord is the oldest date the household has any record of: the
// first transaction, or today when there are none.
func nwsEarliestRecord(accounts []domain.Account, txns []domain.Transaction, now time.Time) time.Time {
	earliest := now
	for _, t := range txns {
		if !t.Date.IsZero() && t.Date.Before(earliest) {
			earliest = t.Date
		}
	}
	return earliest
}

// nwsMonthsBetween counts whole calendar months from a to b (never negative).
func nwsMonthsBetween(a, b time.Time) int {
	m := int(b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
	if m < 0 {
		return 0
	}
	return m
}

// nwsAccountWindow reads one account across the window: its signed net-worth
// contribution, its own balance at each end, and the split between money that
// moved through it and balance that was asserted by hand. All in the canonical
// convention — an asset contributes its balance delta, a liability contributes
// minus the change in what is owed — so these agree with the bridge to the cent.
func nwsAccountWindow(a domain.Account, txns []domain.Transaction, rates currency.Rates, base string,
	since, until time.Time, isAdj func(domain.Transaction) bool) (move, start, end, flow, adj int64) {

	balSince, balUntil := a.OpeningBalance.Amount, a.OpeningBalance.Amount
	var flowAcct, adjAcct int64
	for _, t := range txns {
		if t.AccountID != a.ID {
			continue
		}
		if t.Date.Before(since) {
			balSince += t.Amount.Amount
		}
		if t.Date.Before(until) {
			balUntil += t.Amount.Amount
		}
		if !dateutil.InRange(t.Date, since, until) {
			continue
		}
		if isAdj(t) {
			adjAcct += t.Amount.Amount
		} else {
			flowAcct += t.Amount.Amount
		}
	}
	factor := int64(1)
	delta := balUntil - balSince
	if a.Class == domain.ClassLiability {
		delta = -(absMinor(balUntil) - absMinor(balSince))
		if !(balSince < 0 || (balSince == 0 && balUntil < 0)) {
			factor = -1
		}
	}
	conv := func(minor int64) int64 {
		c, err := rates.Convert(money.New(minor, a.Currency), base)
		if err != nil {
			return 0
		}
		return c.Amount
	}
	return conv(delta), conv(balSince), conv(balUntil), conv(factor * flowAcct), conv(factor * adjAcct)
}

// nwsTypicalMonthlyExpense averages spending over the last three COMPLETE
// months, which is what makes "months of expenses on hand" a real figure rather
// than a reading of whatever this half-finished month happens to look like.
// Returns 0 when there is no spending history to average.
func nwsTypicalMonthlyExpense(txns []domain.Transaction, rates currency.Rates, now time.Time) int64 {
	const window = 3
	end := dateutil.MonthStart(now)
	start := dateutil.AddMonths(end, -window)
	_, expense, err := ledger.PeriodTotals(txns, start, end, rates)
	if err != nil || expense.Amount <= 0 {
		return 0
	}
	return expense.Amount / window
}

// nwsLabels captions the composition points: a month abbreviation per boundary
// and "Now" for the trailing point, thinned so a long window stays legible.
// Blank captions keep their slot so spacing stays even.
func nwsLabels(cutoffs []time.Time, months int) []string {
	out := make([]string, len(cutoffs))
	step := 1
	if n := len(cutoffs); n > 8 {
		step = (n + 7) / 8
	}
	format := "Jan"
	if months > 12 {
		format = "Jan 06"
	}
	for i := range cutoffs {
		switch {
		case i == len(cutoffs)-1:
			out[i] = uistate.T("nw.labelNow")
		case i%step == 0:
			out[i] = cutoffs[i].Format(format)
		}
	}
	return out
}
