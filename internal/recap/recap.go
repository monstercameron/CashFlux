// SPDX-License-Identifier: MIT

// Package recap is the pure, client-side "month in review" core (CG-S1). It
// distills a month of activity into a single glanceable summary — spend vs the
// same span of last month, the top spending category, the biggest single
// expense, savings, and the net-worth change — so the dashboard (and the
// assistant) can narrate "here's your month" the way the leading budgeting apps
// do. Like the rest of internal/reports it is deterministic and has no
// syscall/js: amounts are base-currency minor units and it unit-tests on native
// Go.
package recap

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// MonthRecap is the distilled summary of one month, in base-currency minor
// units. Category / expense fields carry ids and raw descriptors; the UI
// resolves names and formats money. All comparisons are against the SAME span of
// the prior month (e.g. "the first 12 days") so a mid-month recap compares
// like-for-like instead of month-to-date against a full month.
type MonthRecap struct {
	Month    time.Time // first day of the recapped month (UTC)
	AsOf     time.Time // exclusive end of the current window
	Complete bool      // whole month elapsed (window is the full month)
	Base     string    // base currency code, for formatting at the edge

	Income      int64 // income over the current window
	Expense     int64 // expense over the current window (positive)
	Net         int64 // Income - Expense (positive = saved)
	SavingsRate int   // percent of income kept

	PrevExpense     int64 // expense over the same span of the prior month
	SpendDeltaPct   int64 // signed percent change in spend vs that prior span
	SpendDeltaKnown bool  // false when the prior span had zero spend (delta undefined)

	TopCategoryID     string // biggest spending category ("" = uncategorized)
	TopCategoryAmount int64

	MoverID      string // category that changed the most vs the prior span
	MoverDelta   int64  // signed change (current - prior) for that category
	MoverHasData bool

	BiggestExpenseDesc       string
	BiggestExpenseAmount     int64
	BiggestExpenseCategoryID string
	BiggestExpenseKnown      bool

	NetWorthStart int64 // net worth entering the month
	NetWorthEnd   int64 // net worth as of AsOf
	NetWorthDelta int64 // NetWorthEnd - NetWorthStart

	// TypicalNet is the median net over the same span of the trailing months, and
	// NetVsTypical is how far this month sits from it (EC-1).
	//
	// Distinct from the prior-month comparison above because "below last month"
	// and "below typical" are different claims, and only the second survives a
	// lumpy month: one bad prior month makes an ordinary month look like a
	// triumph, and one good one makes it look like a collapse.
	//
	// The MEDIAN, not the mean — a single month carrying a car repair or an annual
	// premium would drag a mean far enough that nothing afterwards reads as
	// unusual, which is the same reasoning as the clearing-window and habit
	// engines.
	TypicalNet    int64
	NetVsTypical  int64
	TypicalMonths int  // how many months the typical rests on
	TypicalKnown  bool // false when there is too little history to have a typical

	NoSpendDays int  // days in the window with no spending
	TxnCount    int  // non-transfer transactions in the window
	HasData     bool // any activity to recap
}

// TypicalMinMonths is how many complete prior months are needed before a
// "typical" exists.
//
// Three. Two months give a median that is just the midpoint of two numbers, and
// calling that "typical" would dress a coin-flip as a baseline.
const TypicalMinMonths = 3

// TypicalLookbackMonths is how far back the typical is read.
//
// Six. Far enough that one unusual month cannot define normal, near enough that
// a household whose circumstances changed — a new job, a new baby — is not
// measured against a life they no longer live.
const TypicalLookbackMonths = 6

// BelowTypical reports whether this month is materially worse than usual, and by
// how much. Callers get the pair so "we have no typical yet" cannot be mistaken
// for "you are exactly typical".
func (r MonthRecap) BelowTypical() (int64, bool) {
	if !r.TypicalKnown || r.NetVsTypical >= 0 {
		return 0, false
	}
	return -r.NetVsTypical, true
}

// Saved reports whether the month is net-positive (income exceeded spend).
func (r MonthRecap) Saved() bool { return r.Net > 0 }

// SpendDown reports whether spending fell versus the prior span (a "good" move).
func (r MonthRecap) SpendDown() bool { return r.SpendDeltaKnown && r.SpendDeltaPct < 0 }

// dayStart truncates t to midnight UTC.
// medianInt64 is the middle value, which resists the one month that carried a
// car repair.
func medianInt64(in []int64) int64 {
	v := append([]int64(nil), in...)
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	n := len(v)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// daysBetween is the whole-day count in the half-open span [a, b).
func daysBetween(a, b time.Time) int {
	if !b.After(a) {
		return 0
	}
	return int(b.Sub(a).Hours() / 24)
}

// Compute distills the month containing `now`. For the current month it recaps
// month-to-date and compares against the first equal-length span of the prior
// month; for a fully-elapsed month it compares the full month against the full
// prior month. Returns a zero-value recap with HasData=false when there is
// nothing in the window. Errors only propagate FX-conversion failures.
func Compute(now time.Time, txns []domain.Transaction, accounts []domain.Account, rates currency.Rates) (MonthRecap, error) {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := dateutil.AddMonths(monthStart, 1)

	// Current window: month start → the end of today (so today's activity
	// counts), capped at the month end for a fully-elapsed month.
	curEnd := dayStart(now).AddDate(0, 0, 1)
	complete := false
	if !curEnd.Before(monthEnd) {
		curEnd = monthEnd
		complete = true
	}

	// Prior comparable window: the same number of days into the previous month,
	// never bleeding into the current month.
	span := daysBetween(monthStart, curEnd)
	prevStart := dateutil.AddMonths(monthStart, -1)
	prevEnd := prevStart.AddDate(0, 0, span)
	if prevEnd.After(monthStart) {
		prevEnd = monthStart
	}

	rec := MonthRecap{Month: monthStart, AsOf: curEnd, Complete: complete, Base: rates.Base}

	cur, err := reports.IncomeVsExpense(txns, monthStart, curEnd, rates)
	if err != nil {
		return MonthRecap{}, err
	}
	prev, err := reports.IncomeVsExpense(txns, prevStart, prevEnd, rates)
	if err != nil {
		return MonthRecap{}, err
	}
	rec.Income = cur.Income
	rec.Expense = cur.Expense
	rec.Net = cur.Net()
	rec.SavingsRate = cur.SavingsRate()
	rec.PrevExpense = prev.Expense
	if pct, ok := ledger.PercentChange(cur.Expense, prev.Expense); ok {
		rec.SpendDeltaPct, rec.SpendDeltaKnown = pct, true
	}

	// EC-1: the same span of each trailing month, so a mid-month recap is compared
	// against the first N days of other months rather than against whole ones.
	var nets []int64
	for i := 1; i <= TypicalLookbackMonths; i++ {
		start := dateutil.AddMonths(monthStart, -i)
		end := start.AddDate(0, 0, span)
		if limit := dateutil.AddMonths(start, 1); end.After(limit) {
			end = limit
		}
		past, perr := reports.IncomeVsExpense(txns, start, end, rates)
		if perr != nil {
			return MonthRecap{}, perr
		}
		// A month with no activity at all is absence of data, not a net of zero;
		// counting it would drag the typical toward zero for every household that
		// started using the app recently.
		if past.Income == 0 && past.Expense == 0 {
			continue
		}
		nets = append(nets, past.Net())
	}
	if len(nets) >= TypicalMinMonths {
		rec.TypicalNet = medianInt64(nets)
		rec.TypicalMonths = len(nets)
		rec.TypicalKnown = true
		rec.NetVsTypical = rec.Net - rec.TypicalNet
	}

	rows, err := reports.SpendingByCategory(txns, monthStart, curEnd, true, prevStart, prevEnd, rates)
	if err != nil {
		return MonthRecap{}, err
	}
	if len(rows) > 0 && rows[0].Amount > 0 {
		rec.TopCategoryID = rows[0].CategoryID
		rec.TopCategoryAmount = rows[0].Amount
	}
	if movers := reports.TopMovers(rows, 1); len(movers) > 0 {
		rec.MoverID = movers[0].CategoryID
		rec.MoverDelta = movers[0].Amount - movers[0].Prior
		rec.MoverHasData = true
	}

	big, err := reports.LargestExpenses(txns, monthStart, curEnd, rates, 1)
	if err != nil {
		return MonthRecap{}, err
	}
	if len(big) > 0 {
		rec.BiggestExpenseDesc = big[0].Desc
		rec.BiggestExpenseAmount = big[0].Amount
		rec.BiggestExpenseCategoryID = big[0].CategoryID
		rec.BiggestExpenseKnown = true
	}

	series, err := ledger.NetWorthSeries(accounts, txns, []time.Time{monthStart, curEnd}, rates)
	if err != nil {
		return MonthRecap{}, err
	}
	if len(series) == 2 {
		rec.NetWorthStart = series[0].Amount
		rec.NetWorthEnd = series[1].Amount
		rec.NetWorthDelta = series[1].Amount - series[0].Amount
	}

	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		if dateutil.InRange(t.Date, monthStart, curEnd) {
			rec.TxnCount++
		}
	}
	rec.NoSpendDays = reports.NoSpendDays(txns, monthStart, curEnd, now)
	rec.HasData = rec.TxnCount > 0 || rec.Income != 0 || rec.Expense != 0

	return rec, nil
}
