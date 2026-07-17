// SPDX-License-Identifier: MIT

package reports

import "sort"

// This file holds the pure computations behind the Annual Review report: the
// year-level roll-ups (months in the red, seasonal extremes), the per-$100
// income breakdown that accompanies the money-flow diagram, and the trim-target
// suggestions the plan section turns into dollar-quantified actions. All logic
// is platform-independent and table-tested on native Go.

// MonthsNegative counts the periods whose net (income − expense) was negative —
// the "months in the red" figure the problem-spots section reports. Periods with
// zero activity (no income AND no expense) are skipped so an empty leading month
// doesn't read as overspending.
func MonthsNegative(flows []PeriodFlow) int {
	n := 0
	for _, f := range flows {
		if f.Income == 0 && f.Expense == 0 {
			continue
		}
		if f.Net() < 0 {
			n++
		}
	}
	return n
}

// SeasonalExtremes returns the indices of the highest- and lowest-spending
// periods in flows (by Expense), skipping zero-activity periods. ok is false
// when fewer than two periods carry any activity — a single month has no
// seasonality to report.
func SeasonalExtremes(flows []PeriodFlow) (hiIdx, loIdx int, ok bool) {
	return SeasonalExtremesSkipping(flows, -1)
}

// SeasonalExtremesSkipping is SeasonalExtremes with one period excluded from
// the ranking — the IN-PROGRESS month. Seventeen days of July always ranks as
// the year's "lightest month" against eleven complete months (QA CF-23), so
// the caller passes the partial period's index (or -1 for none) and it never
// competes.
func SeasonalExtremesSkipping(flows []PeriodFlow, skipIdx int) (hiIdx, loIdx int, ok bool) {
	hiIdx, loIdx = -1, -1
	active := 0
	for i, f := range flows {
		if i == skipIdx {
			continue
		}
		if f.Income == 0 && f.Expense == 0 {
			continue
		}
		active++
		if hiIdx == -1 || f.Expense > flows[hiIdx].Expense {
			hiIdx = i
		}
		if loIdx == -1 || f.Expense < flows[loIdx].Expense {
			loIdx = i
		}
	}
	if active < 2 || hiIdx == loIdx {
		return -1, -1, false
	}
	return hiIdx, loIdx, true
}

// Per100Row is one line of the "where each $100 of income went" table: a
// category's share of income expressed in whole dollars-per-hundred (Cents100
// carries the remainder ×100 for one-decimal display, e.g. 12.5 → Per100=12,
// Cents100=5).
type Per100Row struct {
	CategoryID  string
	AmountMinor int64
	Per100      int64 // whole units of each 100 income units
	Tenths      int64 // first decimal digit (0-9) for "12.5" style display
}

// Per100 expresses the top n spending categories (rows are SpendingByCategory
// output, absolute minor units) as shares of each 100 units of income. Rows
// beyond n are folded into a synthetic row with CategoryID "" appended LAST
// regardless of size (callers label it "everything else"). Income ≤ 0 or no
// spending yields nil.
func Per100(rows []CategorySpend, incomeMinor int64, n int) []Per100Row {
	if incomeMinor <= 0 || len(rows) == 0 || n <= 0 {
		return nil
	}
	abs := func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	}
	share := func(amt int64) (int64, int64) {
		// per-100 with one decimal: amt/income × 100 → ×10 for the tenths digit.
		scaled := amt * 1000 / incomeMinor
		return scaled / 10, scaled % 10
	}
	out := make([]Per100Row, 0, n+1)
	var restMinor int64
	for i, r := range rows {
		a := abs(r.Amount)
		if a == 0 {
			continue
		}
		if i < n {
			p, t := share(a)
			out = append(out, Per100Row{CategoryID: r.CategoryID, AmountMinor: a, Per100: p, Tenths: t})
		} else {
			restMinor += a
		}
	}
	if restMinor > 0 {
		p, t := share(restMinor)
		out = append(out, Per100Row{CategoryID: "", AmountMinor: restMinor, Per100: p, Tenths: t})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// TrimTarget is one plan suggestion: a category whose recent spending runs above
// its own yearly norm, with the concrete monthly trim that returning to the
// median would free up.
type TrimTarget struct {
	CategoryID       string
	MedianMinor      int64 // the category's median monthly spend across the window
	RecentAvgMinor   int64 // average of the most recent 3 buckets
	MonthlySaveMinor int64 // RecentAvg − Median (always > 0 for a returned target)
}

// TrimTargets scans per-category monthly series (CategoryTrends output) for
// categories whose recent 3-month average runs above their own median month —
// the "creep" a plan can reverse without inventing an arbitrary budget. Only
// categories whose recent average is at least minMonthlyMinor are considered
// (trimming a $6/mo category isn't a plan), and only the top n by monthly
// saving are returned, largest first. Series shorter than 4 buckets are skipped
// (a median needs history the recent window doesn't dominate).
func TrimTargets(trends []CategoryTrend, minMonthlyMinor int64, n int) []TrimTarget {
	var out []TrimTarget
	for _, tr := range trends {
		if len(tr.Spend) < 4 {
			continue
		}
		med := medianInt64(tr.Spend)
		if med <= 0 {
			// A mostly-zero category has no "norm" to return to — "hold X at $0/mo"
			// is not a plan, it's a scold. Skip; the watch list still surfaces it.
			continue
		}
		recent := tr.Spend[len(tr.Spend)-3:]
		var sum int64
		for _, v := range recent {
			sum += v
		}
		avg := sum / int64(len(recent))
		if avg < minMonthlyMinor {
			continue
		}
		if save := avg - med; save > 0 {
			out = append(out, TrimTarget{CategoryID: tr.CategoryID, MedianMinor: med, RecentAvgMinor: avg, MonthlySaveMinor: save})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].MonthlySaveMinor > out[j].MonthlySaveMinor })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// medianInt64 returns the median of vs (mean of the two middle values for an
// even count). vs is not modified.
func medianInt64(vs []int64) int64 {
	cp := append([]int64(nil), vs...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}
