// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// NetWorthPanelProps configures the NetWorthPanel component. The panel reads
// all required data directly from appstate, so the zero value is always correct.
type NetWorthPanelProps struct{}

// NetWorthPanel is the compact net-worth card embedded on the Reports
// "networth" tab (FEATURE_MAP §5.7b): the assets/liabilities/net-worth stat
// grid, the assets-vs-liabilities composition bar (R52), and the 6-month
// net-worth trend area chart (C217), plus a "View accounts" drill-through link
// (R52b). The standalone /networth page renders its own richer bento surface
// (NetWorth below); this panel stays the embeddable summary.
//
// Being a registered component (invoked via ui.CreateElement) it owns its own
// hook scope, making it safe to embed inside conditional switch/case branches
// in a parent — including the Reports networth tab — without violating the
// GWC hook-ordering rules (see CLAUDE.md §"CRITICAL gotchas").
//
// The NW trend always uses monthly buckets independent of the cash-flow period
// selector (C217): net worth is a cumulative point-in-time series and
// re-bucketing to weekly or quarterly makes no sense.
func NetWorthPanel(p NetWorthPanelProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	// Subscribe to the shared data-revision atom so the panel re-renders on
	// any mutation (recategorize, balance update, new transaction, etc.).
	_ = uistate.UseDataRevision().Get()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	accounts := app.Accounts()
	txns := app.Transactions()

	// No accounts → nothing to show; caller decides how to fill the space.
	if len(accounts) == 0 {
		return Fragment()
	}

	// Net-worth snapshot: assets, liabilities, net worth as of now.
	nwNet, nwAssets, nwLiab, _ := ledger.NetWorth(accounts, txns, rates)

	// trendBuckets is how many consecutive months the NW trend spans.
	const trendBuckets = 6
	// NW trend: always monthly, last trendBuckets months, full unscoped txns
	// so the household balance-sheet is always complete regardless of scope.
	curMonth := dateutil.MonthStart(time.Now())
	nwBounds := make([]time.Time, 0, trendBuckets+1)
	for k := 0; k <= trendBuckets; k++ {
		nwBounds = append(nwBounds, dateutil.AddMonths(curMonth, k-trendBuckets))
	}
	nwSeries, _ := ledger.NetWorthSeries(accounts, txns, nwBounds, rates)

	// x-axis labels: month abbreviation from nwBounds.
	nwLabels := make([]string, 0, len(nwSeries))
	for i := 0; i < len(nwSeries) && i < len(nwBounds); i++ {
		nwLabels = append(nwLabels, nwBounds[i].Format("Jan"))
	}

	// Convert to major units so the Y-axis ticks read "$14k" not "1400000" (C216).
	nwDiv := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		nwDiv *= 10
	}
	nw := make([]float64, len(nwSeries))
	for i, m := range nwSeries {
		nw[i] = float64(m.Amount) / nwDiv
	}

	// Per-point hover labels from the raw minor-unit series (C216 follow-up).
	nwValueLabels := make([]string, len(nwSeries))
	for i, m := range nwSeries {
		nwValueLabels[i] = fmtMoney(money.New(m.Amount, base))
	}

	// Net-worth change: signed delta over the most recent monthly step.
	var nwChange int64
	if n := len(nwSeries); n >= 2 {
		nwChange = nwSeries[n-1].Amount - nwSeries[n-2].Amount
	}

	// nwAbsI64 is a pure helper used only in this panel's composition-bar pairs.
	nwAbsI64 := func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	}

	// Assets-vs-liabilities composition bar (R52): visual balance-sheet split.
	// Assets green / liabilities red (semantic money tones, not the Tableau palette).
	// Shown only when there's something to compare.
	decimals := currency.Decimals(base)
	var nwCompBar ui.Node = Fragment()
	if nwAssets.Amount != 0 || nwLiab.Amount != 0 {
		compPairs := []struct {
			Label  string
			Amount int64
		}{
			{Label: uistate.T("accounts.assets"), Amount: nwAbsI64(nwAssets.Amount)},
			{Label: uistate.T("dashboard.liabilities"), Amount: nwAbsI64(nwLiab.Amount)},
		}
		compSpec := reportsBarSpec(compPairs, decimals)
		if len(compSpec.Series) > 0 && len(compSpec.Series[0].Points) == 2 {
			compSpec.Series[0].Points[0].Color = "#54b884" // assets (money-positive)
			compSpec.Series[0].Points[1].Color = "#d8716f" // liabilities (money-negative)
		}
		nwCompBar = uiw.Chart(uiw.ChartProps{
			Spec:           compSpec,
			Height:         "120px",
			Label:          uistate.T("reports.assetsVsLiabilities"),
			CurrencySymbol: currency.Symbol(base),
		})
	}

	// R-11/C218: NW composition + trend as a single headline card with an id
	// anchor so /networth can deep-link to this section.
	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("dashboard.netWorth"),
		Attrs: []any{Attr("id", "networth")},
		// R52(b): a nearby drill-down — net worth composes from accounts, so
		// link straight to /accounts to see (and adjust) what's behind the figure.
		HeaderAction: A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/accounts")),
			Attr("data-testid", "networth-drill"), uistate.T("reports.viewAccounts")),
		Body: Fragment(
			Div(css.Class("stat-grid"),
				stat(uistate.T("accounts.assets"), fmtMoney(nwAssets), "pos"),
				stat(uistate.T("dashboard.liabilities"), fmtMoney(nwLiab), "neg"),
				stat(uistate.T("dashboard.netWorth"), fmtMoney(nwNet), accentFor(nwNet)),
				If(len(nwSeries) >= 2, stat(uistate.T("reports.netWorthChange"), fmtMoney(money.New(nwChange, base)), accentFor(money.New(nwChange, base)))),
			),
			// R52: assets-vs-liabilities composition bar (visual balance-sheet split).
			nwCompBar,
			// C217: NW trend uses its own monthly labels, not the cash-flow period labels.
			If(len(nw) >= 2, Fragment(
				P(css.Class("muted"), uistate.T("reports.nwTrendMonthly", trendBuckets)),
				uiw.AreaChart(uiw.AreaChartProps{
					Values: nw,
					// Track the live theme accent instead of a hardcoded off-theme purple.
					Stroke:      chartLineColor(uistate.CurrentAccent()),
					GradientID:  "nw-reports",
					Label:       uistate.T("dashboard.netWorthTrend"),
					Labels:      nwLabels,
					ValueLabels: nwValueLabels,
				}),
			)),
		),
	})
}
