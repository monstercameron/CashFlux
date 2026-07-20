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
	Bucket    balancesheet.Bucket
}

// nwsView is everything both views render, computed once per render.
type nwsView struct {
	Base string
	Dec  int
	Now  time.Time

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

// nwsWindowMonths are the selectable window lengths, shortest first.
var nwsWindowMonths = []int{1, 6, 12, 24}

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

	v := nwsView{Base: base, Dec: currency.Decimals(base), Now: now, Months: months}
	curMonth := dateutil.MonthStart(now)
	v.Since = dateutil.AddMonths(curMonth, -(months - 1))
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
	cutoffs := make([]time.Time, 0, months+2)
	for k := 0; k <= months-1; k++ {
		cutoffs = append(cutoffs, dateutil.AddMonths(v.Since, k))
	}
	cutoffs = append(cutoffs, v.Until)
	v.Points, _ = balancesheet.Series(accounts, txns, cutoffs, rates)
	v.Labels = nwsLabels(cutoffs, months)

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
		row := nwsAcctRow{Acct: a, BalanceMinor: conv.Amount, Bucket: balancesheet.BucketOf(a)}
		row.SideMinor = absMinor(conv.Amount)
		row.MoveMinor = nwsAccountMove(a, txns, rates, base, v.Since, v.Until)
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

// nwsAccountMove is one account's signed net-worth contribution across the
// window, in the canonical convention: an asset contributes its balance delta,
// a liability contributes minus the change in what is owed.
func nwsAccountMove(a domain.Account, txns []domain.Transaction, rates currency.Rates, base string, since, until time.Time) int64 {
	balSince, balUntil := a.OpeningBalance.Amount, a.OpeningBalance.Amount
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
	}
	delta := balUntil - balSince
	if a.Class == domain.ClassLiability {
		delta = -(absMinor(balUntil) - absMinor(balSince))
	}
	conv, err := rates.Convert(money.New(delta, a.Currency), base)
	if err != nil {
		return 0
	}
	return conv.Amount
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
