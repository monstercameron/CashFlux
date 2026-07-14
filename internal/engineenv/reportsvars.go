// SPDX-License-Identifier: MIT

package engineenv

// This file exposes the reports-page figures as engine variables: the
// period-over-period deltas, spending statistics, payee concentration, and the
// cash burn/runway estimate the /reports surface derives — as report_* variables
// usable in any formula or dashboard widget. Everything is computed from the
// fundamental Data fields already fed to Vars (transactions, accounts, rates,
// the active period window, Now) via the pure internal/reports core — the SAME
// functions the reports screen renders — so a report_* figure always matches
// what the page shows for the same window.

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// ReportsVarNames are the fixed report variables addReportsVars exposes, in a
// stable order. Money figures are major units of the base currency; deltas are
// percents; report_runway_months is fractional months.
var ReportsVarNames = []string{
	"report_prev_income",      // income over the immediately preceding window of the same length
	"report_prev_spend",       // spending over the preceding window (positive)
	"report_prev_net",         // net cash flow over the preceding window
	"report_income_delta_pct", // income change vs the preceding window (%)
	"report_spend_delta_pct",  // spending change vs the preceding window (%; up = spending more)
	"report_avg_expense",      // average expense transaction this period
	"report_median_expense",   // median expense transaction this period
	"report_no_spend_days",    // elapsed days this period with zero spending
	"report_top_payee_spend",  // the single biggest payee's spending this period
	"report_top_payee_pct",    // that payee's share of total spending (%)
	"report_burn",             // average monthly spending over the last 6 full months
	"report_runway_months",    // months liquid cash lasts at that burn (0 = sustainable)
}

// reportsBurnMonths is the trailing full-month window the burn average spans —
// the current partial month is excluded so it doesn't understate spending
// (matching the reports screen's cash-runway stat).
const reportsBurnMonths = 6

func init() { Names = append(Names, ReportsVarNames...) }

// addReportsVars derives the report_* variables. The "previous" window is the
// same DURATION immediately before the active period (a close approximation of
// the screen's calendar-aware period shift; for month/quarter/year windows the
// figures agree in practice because transactions cluster inside the window).
// liquidMinor/liquidErr are computeAtoms' LiquidBalance result, threaded in so
// the full-transaction scan runs once per Vars call, not per helper.
func addReportsVars(out map[string]float64, d Data, major func(int64) float64, liquidMinor int64, liquidErr error) {
	start, end := d.PeriodStart, d.PeriodEnd
	if start.IsZero() || end.IsZero() {
		start, end = dateutil.MonthRange(d.Now)
	}
	ps, pe := start.Add(-end.Sub(start)), start

	cur, _ := reports.IncomeVsExpense(d.Transactions, start, end, d.Rates)
	prev, _ := reports.IncomeVsExpense(d.Transactions, ps, pe, d.Rates)
	out["report_prev_income"] = major(prev.Income)
	out["report_prev_spend"] = major(prev.Expense)
	out["report_prev_net"] = major(prev.Net())
	out["report_income_delta_pct"] = 0
	if pct, ok := ledger.PercentChange(cur.Income, prev.Income); ok {
		out["report_income_delta_pct"] = float64(pct)
	}
	out["report_spend_delta_pct"] = 0
	if pct, ok := ledger.PercentChange(cur.Expense, prev.Expense); ok {
		out["report_spend_delta_pct"] = float64(pct)
	}

	stats, _ := reports.SpendingStats(d.Transactions, start, end, d.Rates)
	out["report_avg_expense"] = major(stats.Average)
	out["report_median_expense"] = major(stats.Median)
	out["report_no_spend_days"] = float64(reports.NoSpendDays(d.Transactions, start, end, d.Now))

	// Payee concentration: the biggest payee's spend and its share of the total.
	out["report_top_payee_spend"] = 0
	out["report_top_payee_pct"] = 0
	if payees, err := reports.TopPayees(d.Transactions, start, end, d.Rates, 1); err == nil && len(payees) > 0 {
		top := payees[0].Amount
		if top < 0 {
			top = -top
		}
		out["report_top_payee_spend"] = major(top)
		if cur.Expense > 0 {
			out["report_top_payee_pct"] = float64(top*100) / float64(cur.Expense)
		}
	}

	// Burn + runway: average monthly spending over the trailing full months, and
	// how long liquid cash lasts at that pace. Sustainable (burn covered by
	// income or zero) reports 0 months — the "never depletes" convention the
	// plan_<slug>_runway variables use.
	curMonth := dateutil.MonthStart(d.Now)
	monthBounds := make([]time.Time, 0, reportsBurnMonths+1)
	for k := 0; k <= reportsBurnMonths; k++ {
		monthBounds = append(monthBounds, dateutil.AddMonths(curMonth, k-reportsBurnMonths))
	}
	flows, _ := reports.IncomeExpenseSeries(d.Transactions, monthBounds, d.Rates)
	burn := reports.AverageMonthlyExpense(flows)
	out["report_burn"] = major(burn)
	out["report_runway_months"] = 0
	if liquidErr == nil && burn > 0 {
		rw := reports.EstimateRunway(liquidMinor, burn)
		if !rw.Sustainable {
			out["report_runway_months"] = float64(rw.Months) + float64(rw.Days)/30
		}
	}
}
