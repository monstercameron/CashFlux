//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// trendBuckets is how many consecutive periods the cash-flow trend spans.
const trendBuckets = 6

// accentForRunway tones the cash-runway stat: a thin buffer (under three months)
// reads as a warning, a healthy one (six months or more) as positive.
func accentForRunway(months int) string {
	switch {
	case months < 3:
		return "neg"
	case months >= 6:
		return "pos"
	default:
		return ""
	}
}

// Reports is the read-only reporting screen (B21): for the period chosen in the
// top bar it shows income / expense / net, a plain-English summary, and spending
// by category compared to the prior period — all from the pure internal/reports
// core, so the figures match the rest of the app. Charts come in a follow-up.
func Reports() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()

	// The viewed period is the shared top-bar window; the comparison is the
	// immediately preceding window of the same length.
	w := uistate.UsePeriod().Get()
	cs, ce := w.Range()
	ps, pe := w.Shift(-1).Range()

	flow, _ := reports.IncomeVsExpense(txns, cs, ce, rates)
	rows, _ := reports.SpendingByCategory(txns, cs, ce, true, ps, pe, rates)

	// Cash-flow trend: net for each of the last trendBuckets periods of the
	// viewed resolution, ending with the current one.
	pr := uistate.UsePrefs().Get()
	weekStart := pr.WeekStartWeekday()
	startCur := period.Truncate(w.Res, w.From, weekStart)
	bounds := make([]time.Time, 0, trendBuckets+1)
	for k := 0; k <= trendBuckets; k++ {
		bounds = append(bounds, period.Step(w.Res, startCur, k-(trendBuckets-1)))
	}
	flows, _ := reports.IncomeExpenseSeries(txns, bounds, rates)
	netSeries := make([]float64, len(flows))
	for i, f := range flows {
		netSeries[i] = float64(f.Net())
	}

	// Net-worth trend: net worth as of each period boundary (cumulative, so it
	// reads the running total rather than per-period flow).
	accounts := app.Accounts()
	nwSeries, _ := ledger.NetWorthSeries(accounts, txns, bounds, rates)
	nw := make([]float64, len(nwSeries))
	for i, m := range nwSeries {
		nw[i] = float64(m.Amount)
	}
	// Net-worth composition (assets vs liabilities) as of now, for a breakdown card.
	nwNet, nwAssets, nwLiab, _ := ledger.NetWorth(accounts, txns, rates)

	// Savings-rate trend: percent of income kept per period.
	srInts, _ := reports.SavingsRateSeries(txns, bounds, rates)
	srSeries := make([]float64, len(srInts))
	for i, v := range srInts {
		srSeries[i] = float64(v)
	}

	// Cash runway (B21): how long spendable cash would last at the average burn
	// over the last six *full* months (the current partial month is excluded so it
	// doesn't understate spending). Liquid = cash-type accounts only.
	const runwayMonths = 6
	var liquid int64
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		switch a.Type {
		case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
			if bal, err := ledger.Balance(a, txns); err == nil {
				if conv, err := rates.Convert(bal, base); err == nil {
					liquid += conv.Amount
				}
			}
		}
	}
	curMonth := dateutil.MonthStart(time.Now())
	monthBounds := make([]time.Time, 0, runwayMonths+1)
	for k := 0; k <= runwayMonths; k++ {
		monthBounds = append(monthBounds, dateutil.AddMonths(curMonth, k-runwayMonths))
	}
	monthFlows, _ := reports.IncomeExpenseSeries(txns, monthBounds, rates)
	burn := reports.AverageMonthlyExpense(monthFlows)
	runway := reports.EstimateRunway(liquid, burn)

	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	nameOf := func(id string) string {
		if n := catName[id]; n != "" {
			return n
		}
		return uistate.T("reports.uncategorized")
	}
	fmtMinor := func(v int64) string { return fmtMoney(money.New(v, base)) }
	narrative := reports.SpendingNarrative(rows, true, fmtMinor, func(id string) string { return catName[id] })

	// Category rows are plain text (no interactive controls), so building them in
	// a loop is safe (no On* hooks involved).
	var rowNodes []ui.Node
	for _, r := range rows {
		if r.Amount == 0 && r.Prior == 0 {
			continue
		}
		delta := Fragment()
		if r.HasDelta && r.Amount != r.Prior {
			// Spending up is red (worse), down is green (better).
			tone, arrow := "text-down", "▲"
			if r.DeltaPct < 0 {
				tone, arrow = "text-up", "▼"
			}
			pct := r.DeltaPct
			if pct < 0 {
				pct = -pct
			}
			delta = Span(Class("row-meta "+tone), fmt.Sprintf("%s %d%%", arrow, pct))
		}
		rowNodes = append(rowNodes, Div(Class("row"),
			Div(Class("row-main"), Span(Class("row-desc"), nameOf(r.CategoryID))),
			delta,
			Span(Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	var catBody ui.Node
	if len(rowNodes) == 0 {
		catBody = P(Class("empty"), uistate.T("reports.empty"))
	} else {
		catBody = Div(Class("rows"), rowNodes)
	}

	// Top payees: where the money went by merchant/description this period.
	payees, _ := reports.TopPayees(txns, cs, ce, rates, 8)
	var payeeNodes []ui.Node
	for _, p := range payees {
		name := p.Name
		if name == "" {
			name = uistate.T("reports.noPayee")
		}
		payeeNodes = append(payeeNodes, Div(Class("row"),
			Div(Class("row-main"), Span(Class("row-desc"), name)),
			Span(Class("budget-amount"), fmtMinor(p.Amount)),
		))
	}

	// Biggest individual expenses this period.
	largest, _ := reports.LargestExpenses(txns, cs, ce, rates, 8)
	var largestNodes []ui.Node
	for _, e := range largest {
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		largestNodes = append(largestNodes, Div(Class("row"),
			Div(Class("row-main"),
				Span(Class("row-desc"), desc),
				Span(Class("row-meta"), pr.FormatDate(e.Date)),
			),
			Span(Class("budget-amount"), fmtMinor(e.Amount)),
		))
	}

	// Spending by member: the household "who spent what" view this period.
	memberSpend, _ := reports.SpendingByMember(txns, cs, ce, rates)
	memberName := make(map[string]string, len(app.Members()))
	for _, m := range app.Members() {
		memberName[m.ID] = m.Name
	}
	var memberNodes []ui.Node
	for _, ms := range memberSpend {
		name := memberName[ms.MemberID]
		if name == "" {
			name = uistate.T("reports.noMember")
		}
		memberNodes = append(memberNodes, Div(Class("row"),
			Div(Class("row-main"), Span(Class("row-desc"), name)),
			Span(Class("budget-amount"), fmtMinor(ms.Amount)),
		))
	}

	// Biggest deposits: the largest individual income transactions this period.
	bigIncome, _ := reports.LargestIncome(txns, cs, ce, rates, 8)
	var bigIncomeNodes []ui.Node
	for _, e := range bigIncome {
		desc := e.Desc
		if desc == "" {
			desc = nameOf(e.CategoryID)
		}
		bigIncomeNodes = append(bigIncomeNodes, Div(Class("row"),
			Div(Class("row-main"),
				Span(Class("row-desc"), desc),
				Span(Class("row-meta"), pr.FormatDate(e.Date)),
			),
			Span(Class("budget-amount"), fmtMinor(e.Amount)),
		))
	}

	// Income by source: where the money comes from this period.
	incomeRows, _ := reports.IncomeByCategory(txns, cs, ce, rates)
	var incomeNodes []ui.Node
	for _, r := range incomeRows {
		if r.Amount == 0 {
			continue
		}
		incomeNodes = append(incomeNodes, Div(Class("row"),
			Div(Class("row-main"), Span(Class("row-desc"), nameOf(r.CategoryID))),
			Span(Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	// Spending-by-weekday insight: which day money tends to leave.
	weekdayPeakLine := ""
	if wd, err := reports.SpendingByWeekday(txns, cs, ce, rates); err == nil {
		if d, ok := reports.PeakWeekday(wd); ok {
			weekdayPeakLine = uistate.T("reports.peakWeekday", d.String(), fmtMinor(wd[d]))
		}
	}

	net := money.New(flow.Net(), base)
	return Div(
		Div(Class("stat-grid"),
			stat(uistate.T("dashboard.income"), fmtMoney(money.New(flow.Income, base)), "pos"),
			stat(uistate.T("dashboard.spending"), fmtMoney(money.New(flow.Expense, base)), "neg"),
			stat(uistate.T("reports.net"), fmtMoney(net), accentFor(net)),
			stat(uistate.T("dashboard.savingsRate"), fmt.Sprintf("%d%%", flow.SavingsRate()), ""),
			If(burn > 0, stat(uistate.T("reports.runway"), uistate.T("reports.runwayMonths", runway.Months), accentForRunway(runway.Months))),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.byCategory")),
			P(Class("muted"), narrative),
			If(weekdayPeakLine != "", P(Class("muted"), weekdayPeakLine)),
			catBody,
			If(len(rowNodes) > 0, Div(Class("flex flex-wrap gap-2 py-1"),
				Button(Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("spending-by-category.csv", "text/csv", reports.CategoryCSV(rows, nameOf, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			)),
		),
		If(len(bigIncomeNodes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.biggestDeposits")),
			Div(Class("rows"), bigIncomeNodes),
		)),
		If(len(incomeNodes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.incomeBySource")),
			Div(Class("rows"), incomeNodes),
			Div(Class("flex flex-wrap gap-2 py-1"),
				Button(Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("income-by-source.csv", "text/csv", reports.CategoryCSV(incomeRows, nameOf, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			),
		)),
		If(len(payeeNodes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.topPayees")),
			Div(Class("rows"), payeeNodes),
		)),
		If(len(largestNodes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.biggestExpenses")),
			Div(Class("rows"), largestNodes),
		)),
		If(len(memberSpend) > 1, Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.byMember")),
			Div(Class("rows"), memberNodes),
			Div(Class("flex flex-wrap gap-2 py-1"),
				Button(Class("btn"), Type("button"), Title(uistate.T("reports.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					nm := func(id string) string {
						if n := memberName[id]; n != "" {
							return n
						}
						return uistate.T("reports.noMember")
					}
					downloadBytes("spending-by-member.csv", "text/csv", reports.MemberCSV(memberSpend, nm, csvAmount))
				}), uistate.T("reports.downloadCsv")),
			),
		)),
		If(len(netSeries) >= 2, Section(Class("card"),
			H2(Class("card-title"), uistate.T("dashboard.cashFlow")),
			P(Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
			uiw.AreaChart(uiw.AreaChartProps{Values: netSeries, GradientID: "cf-reports", Label: uistate.T("dashboard.cashFlow")}),
		)),
		If(len(accounts) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("dashboard.netWorth")),
			Div(Class("stat-grid"),
				stat(uistate.T("accounts.assets"), fmtMoney(nwAssets), "pos"),
				stat(uistate.T("dashboard.liabilities"), fmtMoney(nwLiab), "neg"),
				stat(uistate.T("dashboard.netWorth"), fmtMoney(nwNet), accentFor(nwNet)),
			),
		)),
		If(len(nw) >= 2, Section(Class("card"),
			H2(Class("card-title"), uistate.T("dashboard.netWorthTrend")),
			uiw.AreaChart(uiw.AreaChartProps{Values: nw, Stroke: "#7c83ff", GradientID: "nw-reports", Label: uistate.T("dashboard.netWorthTrend")}),
		)),
		If(len(srSeries) >= 2, Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.savingsTrend")),
			P(Class("muted"), uistate.T("reports.trendHint", trendBuckets)),
			uiw.AreaChart(uiw.AreaChartProps{Values: srSeries, GradientID: "sr-reports", Label: uistate.T("reports.savingsTrend")}),
		)),
	)
}
