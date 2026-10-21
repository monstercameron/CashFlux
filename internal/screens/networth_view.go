// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/attribution"
	"github.com/monstercameron/CashFlux/internal/balancesheet"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/networthview"
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
	// WindowKnown reports whether the selected-window amounts converted and
	// reduced without error. Current balance availability is tracked separately.
	WindowKnown bool
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
	Snapshot    ledger.NetWorthResult
	SnapshotErr error

	// Bridge decomposes the window; its EndMinor is the same net worth the hero
	// prints and its legs sum to it exactly, residual included.
	Bridge    attribution.Bridge
	BridgeErr error

	// Points is the monthly composition series across the window, with today
	// appended as the final point so the chart ends where the hero does.
	Points    []balancesheet.Point
	SeriesErr error
	// Labels captions Points, one per point (blank where thinned).
	Labels         []string
	PreparationErr error

	Health balancesheet.Health
	// CashMinor is the spendable (BucketCash) total; MonthlyExpenseMinor is
	// typical monthly spending over the trailing quarter (0 when unknown).
	CashMinor, MonthlyExpenseMinor int64
	CashKnown, MonthlyExpenseKnown bool
	HealthKnown, RunwayKnown       bool

	// Accounts holds EVERY non-archived account, biggest side-magnitude first.
	// Nothing is capped here: a view that hides rows must say so itself.
	Accounts  []nwsAcctRow
	AssetRows []nwsAcctRow
	DebtRows  []nwsAcctRow
	MoverRows []nwsAcctRow
}

// Assets returns the asset rows in display order.
func (v nwsView) Assets() []nwsAcctRow { return v.AssetRows }

// Liabilities returns the liability rows in display order.
func (v nwsView) Liabilities() []nwsAcctRow { return v.DebtRows }

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
	return v.MoverRows
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

	prepared, err := networthview.Prepare(networthview.Input{
		Accounts: accounts, Transactions: txns, Rates: rates, Months: months, Now: now,
		IsAdjustment: isBalanceAdjustment,
	})
	if err != nil {
		return nwsView{Base: base, Dec: currency.Decimals(base), Now: now, Months: months, Rates: rates, PreparationErr: err}
	}
	v := nwsView{
		Base: prepared.Base, Dec: currency.Decimals(prepared.Base), Now: now, Months: prepared.Months,
		Since: prepared.Since, Until: prepared.Until, Snapshot: prepared.Snapshot.NetWorthResult,
		SnapshotErr: prepared.SnapshotErr, Bridge: prepared.Bridge, BridgeErr: prepared.BridgeErr,
		Points: prepared.Points, SeriesErr: prepared.SeriesErr, Labels: nwsLabels(prepared.Cutoffs, prepared.SpanMonths),
		CashMinor: prepared.Cash.Amount, CashKnown: prepared.CashKnown,
		MonthlyExpenseMinor: prepared.Expense.Amount.Amount, MonthlyExpenseKnown: prepared.Expense.Known,
		Health: prepared.Health, HealthKnown: prepared.HealthKnown, RunwayKnown: prepared.RunwayKnown,
	}
	mapFacts := func(facts []networthview.AccountFact) []nwsAcctRow {
		rows := make([]nwsAcctRow, 0, len(facts))
		for _, fact := range facts {
			row := nwsAcctRow{
				Acct: fact.Account, BalanceMinor: fact.Window.BaseBalance.Amount, SideMinor: int64(fact.SideMagnitude),
				Bucket: fact.Bucket, AsOf: fact.AsOf, WindowKnown: fact.Window.WindowKnown(), Manual: fact.Window.Manual,
			}
			if row.WindowKnown {
				row.MoveMinor = fact.Window.Move.Amount
				row.StartMinor, row.EndMinor = fact.Window.Start.Amount, fact.Window.End.Amount
				row.FlowMinor, row.AdjMinor = fact.Window.Flow.Amount, fact.Window.Adjustment.Amount
			}
			rows = append(rows, row)
		}
		return rows
	}
	v.Accounts, v.AssetRows, v.DebtRows, v.MoverRows = mapFacts(prepared.Accounts), mapFacts(prepared.Assets), mapFacts(prepared.Liabilities), mapFacts(prepared.Movers)
	return v
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
