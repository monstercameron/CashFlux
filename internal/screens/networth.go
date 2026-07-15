// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
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

// nwTile wraps a tile body in the shared Widget chrome at an explicit bento
// column placement ("1 / span 4" full-width, "span 2" for a half-width pair).
func nwTile(tid, col string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: tid, Title: "", GridColumn: col, Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// nwSection wraps a tile body with a serif section title + optional header
// action, reusing the debt-section chrome so /networth matches the other
// redesigned surfaces.
func nwSection(sid, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if sid != "" {
		args = append(args, Attr("id", sid))
	}
	if title != "" {
		args = append(args, Div(css.Class("debt-section-head"),
			H2(css.Class("debt-section-title"), title),
			If(action != nil, action),
		))
	}
	args = append(args, body)
	return Div(args...)
}

// nwTypeBucketLabelKey maps an asset account type to its composition bucket's
// i18n key — the SAME buckets the networth_* engine variables use, so the tile
// and the variables always agree.
func nwTypeBucketLabelKey(t domain.AccountType) string {
	switch t {
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
		return "nw.bucketCash"
	case domain.TypeInvestment, domain.TypeRetirement, domain.TypeCrypto:
		return "nw.bucketInvested"
	case domain.TypeProperty, domain.TypeVehicle:
		return "nw.bucketProperty"
	}
	return "nw.bucketOther"
}

// nwLiabilityBucketLabelKey maps a liability account type to its bucket's i18n key.
func nwLiabilityBucketLabelKey(t domain.AccountType) string {
	switch t {
	case domain.TypeCreditCard, domain.TypeLineOfCredit:
		return "nw.bucketCredit"
	case domain.TypeMortgage:
		return "nw.bucketMortgage"
	}
	return "nw.bucketLoans"
}

// NetWorth is the dedicated /networth screen (FEATURE_MAP §5.7a), a widgetized
// bento surface over the household balance sheet: a hero tile (the net figure
// in the display serif with a month-to-date delta pill and figure chips), a
// toolbar (trend horizon + report metrics + drills), the trend tile (accent
// area chart with an insight takeaway), the "what you own / what you owe"
// composition pair (the SAME buckets as the networth_* engine variables), and
// the per-account contribution rows. All figures come from the pure ledger
// core; the same figures are exposed as networth_* variables
// (engineenv.addNetWorthVars) for formulas and dashboard widgets.
func NetWorth() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	_ = uistate.UsePrefs().Get() // re-render when the accent/theme changes

	// Trend horizon (6/12/24 months), persisted so the page reopens as it was read.
	cfg := uistate.NetWorthConfigGet()
	horizon := ui.UseState(fmt.Sprintf("%d", cfg.TrendMonths))
	showFormulas := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))
	ui.UseEffect(func() func() {
		months := 6
		fmt.Sscanf(horizon.Get(), "%d", &months)
		uistate.SetNetWorthConfig(uistate.NetWorthConfig{TrendMonths: months})
		return nil
	}, horizon.Get())

	if len(app.Accounts()) == 0 {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{
			Message:   uistate.T("reports.emptyNetWorth"),
			CTALabel:  uistate.T("accounts.addFirst"),
			AddTarget: "account",
		})
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	accounts := app.Accounts()
	txns := app.Transactions()
	accent := chartLineColor(uistate.CurrentAccent())
	pr := uistate.LoadPrefs()

	// Snapshot with explainability: a rate-less account is EXCLUDED and disclosed,
	// never silently zeroed (C79 honesty).
	nwRes, _ := ledger.NetWorthExplained(accounts, txns, rates)
	nwNet, nwAssets, nwLiab := nwRes.Net, nwRes.Assets, nwRes.Liabilities

	// Month-to-date change (matches the networth_change engine variable).
	curMonth := dateutil.MonthStart(time.Now())
	mtdBounds := []time.Time{curMonth, time.Now().AddDate(0, 0, 1)}
	var mtdChange int64
	if s, err := ledger.NetWorthSeries(accounts, txns, mtdBounds, rates); err == nil && len(s) == 2 {
		mtdChange = s[1].Amount - s[0].Amount
	}

	// Trend over the chosen horizon: always monthly (C217), with today appended
	// as the final point so the curve (and its takeaway) ends at the SAME figure
	// the hero shows — not at the month boundary a few days back.
	months := 6
	fmt.Sscanf(horizon.Get(), "%d", &months)
	nwBounds := make([]time.Time, 0, months+2)
	for k := 0; k <= months; k++ {
		nwBounds = append(nwBounds, dateutil.AddMonths(curMonth, k-months))
	}
	nwBounds = append(nwBounds, time.Now().AddDate(0, 0, 1))
	nwSeries, _ := ledger.NetWorthSeries(accounts, txns, nwBounds, rates)
	nwDiv := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		nwDiv *= 10
	}
	nwVals := make([]float64, len(nwSeries))
	nwValueLabels := make([]string, len(nwSeries))
	for i, m := range nwSeries {
		nwVals[i] = float64(m.Amount) / nwDiv
		nwValueLabels[i] = fmtMoney(money.New(m.Amount, base))
	}
	// Month captions, thinned so long horizons stay legible (≤ 8 non-empty labels;
	// blanks keep their flex slots so spacing stays even).
	nwLabels := make([]string, len(nwSeries))
	labelStep := 1
	if n := len(nwSeries); n > 8 {
		labelStep = (n + 7) / 8
	}
	for i := range nwSeries {
		switch {
		case i == len(nwSeries)-1:
			nwLabels[i] = uistate.T("nw.labelNow")
		case i%labelStep == 0:
			format := "Jan"
			if months > 12 {
				format = "Jan 06"
			}
			nwLabels[i] = nwBounds[i].Format(format)
		}
	}

	// Trend takeaway: direction + magnitude over the window (R52a).
	trendTakeaway := ""
	if len(nwSeries) >= 2 {
		delta := nwSeries[len(nwSeries)-1].Amount - nwSeries[0].Amount
		latest := fmtMoney(nwSeries[len(nwSeries)-1])
		mag := fmtMoney(money.New(absMinor(delta), base))
		switch {
		case delta > 0:
			trendTakeaway = uistate.T("nw.trendTakeawayUp", mag, months, latest)
		case delta < 0:
			trendTakeaway = uistate.T("nw.trendTakeawayDown", mag, months, latest)
		default:
			trendTakeaway = uistate.T("nw.trendTakeawayFlat", latest, months)
		}
	}

	// Composition + per-account rows: FX-converted per-account balances, bucketed
	// by type — the SAME buckets as the networth_* variables. Rate-less accounts
	// are skipped (they're already disclosed in the hero note).
	type acctBal struct {
		Acct  domain.Account
		Minor int64 // signed, base currency
	}
	var (
		bals        []acctBal
		assetBucket = map[string]int64{}
		liabBucket  = map[string]int64{}
		cashAssets  int64
	)
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
			continue
		}
		bals = append(bals, acctBal{Acct: a, Minor: conv.Amount})
		if a.Class == domain.ClassLiability {
			m := conv.Amount
			if m < 0 {
				m = -m
			}
			liabBucket[nwLiabilityBucketLabelKey(a.Type)] += m
		} else {
			key := nwTypeBucketLabelKey(a.Type)
			assetBucket[key] += conv.Amount
			if key == "nw.bucketCash" {
				cashAssets += conv.Amount
			}
		}
	}
	liquidPct := int64(0)
	if nwAssets.Amount > 0 {
		liquidPct = cashAssets * 100 / nwAssets.Amount
	}

	// ── Hero tile ────────────────────────────────────────────────────────────────
	var deltaPill ui.Node = Fragment()
	if mtdChange != 0 {
		arrow, tone := "▲", "pos"
		mag := mtdChange
		if mtdChange < 0 {
			arrow, tone, mag = "▼", "neg", -mtdChange
		}
		deltaPill = Span(ClassStr("rpt-delta "+tone), Attr("data-testid", "nw-delta"),
			Attr("title", uistate.T("nw.deltaTitle")),
			arrow+" "+uistate.T("nw.deltaMonth", fmtMoney(money.New(mag, base))))
	}
	chips := []ui.Node{
		rptChip(uistate.T("accounts.assets"), fmtMoney(nwAssets), rptToneCls("pos")),
		rptChip(uistate.T("dashboard.liabilities"), fmtMoney(nwLiab), rptToneCls("neg")),
		rptChip(uistate.T("nw.figLiquid"), fmt.Sprintf("%d%%", liquidPct), ""),
	}
	// Debt-to-asset ratio carries real signal (a bare account count doesn't);
	// danger-toned once debt passes half of what's owned.
	if nwAssets.Amount > 0 {
		ratio := nwLiab.Amount * 100 / nwAssets.Amount
		tone := ""
		if ratio >= 50 {
			tone = rptToneCls("neg")
		}
		chips = append(chips, rptChip(uistate.T("nw.figDebtRatio"), fmt.Sprintf("%d%%", ratio), tone))
	}
	var missingNote, byChoiceNote ui.Node = Fragment(), Fragment()
	if len(nwRes.MissingCurrencies) > 0 {
		missingNote = P(css.Class("err"), Attr("role", "alert"),
			uistate.T("accounts.nwExcludes", plural(len(nwRes.ExcludedAccounts), "account"), strings.Join(nwRes.MissingCurrencies, ", ")))
	}
	// AC11: disclose accounts the household chose to leave out, so the figure is
	// never silently reduced. Informational (role=status), not an error.
	if n := len(nwRes.ExcludedByChoice); n > 0 {
		byChoiceNote = P(css.Class("t-caption", tw.TextDim), Attr("role", "status"), Attr("data-testid", "nw-excludes-by-choice"),
			uistate.T(excludesByChoiceKey(n), n))
	}
	excludesNote := Fragment(missingNote, byChoiceNote)
	heroTile := nwTile("nw-hero", "1 / span 4", nwSection("sec-nw-hero", uistate.T("dashboard.netWorth"), nil,
		Div(css.Class("rpt-hero"),
			P(css.Class("rpt-hero-eyebrow", tw.TextDim), uistate.T("nw.asOf", pr.FormatDate(time.Now()))),
			Div(css.Class("rpt-hero-main"),
				Div(
					Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)+rptToneCls(accentFor(nwNet))), Attr("data-countup", ""), Attr("data-testid", "nw-hero-value"), fmtMoney(nwNet)),
					deltaPill,
				),
			),
			Div(css.Class("debt-chips"), chips),
			excludesNote,
		)))

	// ── Toolbar tile ─────────────────────────────────────────────────────────────
	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("nw.metricsShow")
	if showFormulas.Get() {
		metricsCls += " is-on"
		metricsLabel = uistate.T("nw.metricsHide")
	}
	toolbar := nwTile("nw-toolbar", "1 / span 4", Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			uiw.Segmented(uiw.SegmentedProps{
				Label:    uistate.T("networth.trendHorizon"),
				Selected: horizon.Get(),
				OnSelect: func(v string) { horizon.Set(v) },
				Options: []uiw.SegOption{
					{Value: "6", Label: uistate.T("nw.horizon6")},
					{Value: "12", Label: uistate.T("nw.horizon12")},
					{Value: "24", Label: uistate.T("nw.horizon24")},
				},
			}),
			Button(ClassStr(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(showFormulas.Get())),
				Attr("data-testid", "nw-toggle-formulas"), Title(uistate.T("nw.metricsTitle")),
				OnClick(toggleFormulas), Text(metricsLabel)),
		),
		Div(css.Class("filter-strip-controls"),
			A(css.Class("btn", "btn-ghost"), Href(uistate.RoutePath("/accounts")), Attr("data-testid", "nw-accounts-link"), uistate.T("reports.viewAccounts")),
			A(css.Class("btn", "btn-ghost"), Href(uistate.RoutePath("/debt")), Attr("data-testid", "nw-debt-link"), uistate.T("nw.viewDebts")),
		),
	))

	// ── Trend tile ───────────────────────────────────────────────────────────────
	trendTile := nwTile("nw-trend", "1 / span 4", nwSection("sec-nw-trend", uistate.T("nw.trendTitle"), nil,
		Fragment(
			If(trendTakeaway != "", P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "nw-takeaway"), trendTakeaway)),
			P(css.Class("muted"), uistate.T("reports.nwTrendMonthly", months)),
			If(len(nwVals) >= 2, uiw.AreaChart(uiw.AreaChartProps{
				Values: nwVals, Stroke: accent, GradientID: "nw-page",
				Label: uistate.T("dashboard.netWorthTrend"), Labels: nwLabels, ValueLabels: nwValueLabels,
			})),
		)))

	// ── Composition pair: what you own / what you owe ────────────────────────────
	assetOrder := []string{"nw.bucketCash", "nw.bucketInvested", "nw.bucketProperty", "nw.bucketOther"}
	var ownRows []ui.Node
	for _, key := range assetOrder {
		amt := assetBucket[key]
		if amt == 0 {
			continue
		}
		pct := int64(0)
		if nwAssets.Amount > 0 {
			pct = amt * 100 / nwAssets.Amount
		}
		ownRows = append(ownRows, Div(css.Class("row"),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), uistate.T(key)),
				Div(css.Class("share-bar"), Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)}))),
			),
			Span(css.Class("row-meta"), fmt.Sprintf("%d%%", pct)),
			Span(css.Class("budget-amount"), fmtMoney(money.New(amt, base))),
		))
	}
	ownTile := nwTile("nw-own", "span 2", nwSection("sec-nw-own", uistate.T("nw.ownTitle"), nil,
		IfElse(len(ownRows) == 0,
			P(css.Class("empty"), uistate.T("nw.ownEmpty")),
			Div(css.Class("rows"), ownRows))))

	liabOrder := []string{"nw.bucketCredit", "nw.bucketLoans", "nw.bucketMortgage"}
	var totalLiab int64
	for _, v := range liabBucket {
		totalLiab += v
	}
	var oweRows []ui.Node
	for _, key := range liabOrder {
		amt := liabBucket[key]
		if amt == 0 {
			continue
		}
		pct := int64(0)
		if totalLiab > 0 {
			pct = amt * 100 / totalLiab
		}
		oweRows = append(oweRows, Div(css.Class("row"),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), uistate.T(key)),
				Div(css.Class("share-bar"), Div(css.Class("share-bar-fill", "nw-bar-down"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)}))),
			),
			Span(css.Class("row-meta"), fmt.Sprintf("%d%%", pct)),
			Span(ClassStr("budget-amount "+tw.ColorClass("text-down")), fmtMoney(money.New(amt, base))),
		))
	}
	oweAction := A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/debt")), Attr("data-testid", "nw-owe-drill"), uistate.T("nw.viewDebts"))
	oweTile := nwTile("nw-owe", "span 2", nwSection("sec-nw-owe", uistate.T("nw.oweTitle"), oweAction,
		IfElse(len(oweRows) == 0,
			P(css.Class("empty"), Attr("data-testid", "nw-debt-free"), uistate.T("nw.debtFree")),
			Div(css.Class("rows"), oweRows))))

	// ── Per-account contributions ────────────────────────────────────────────────
	var maxAbs int64
	for _, b := range bals {
		m := b.Minor
		if m < 0 {
			m = -m
		}
		if m > maxAbs {
			maxAbs = m
		}
	}
	// Largest magnitudes first, capped so a many-account household stays scannable.
	const maxAcctRows = 12
	sorted := append([]acctBal(nil), bals...)
	sort.SliceStable(sorted, func(i, j int) bool {
		ai, aj := sorted[i].Minor, sorted[j].Minor
		if ai < 0 {
			ai = -ai
		}
		if aj < 0 {
			aj = -aj
		}
		return ai > aj
	})
	var acctRows []ui.Node
	for i, b := range sorted {
		if i >= maxAcctRows {
			acctRows = append(acctRows, P(css.Class("muted"), uistate.T("nw.accountsMore", len(sorted)-maxAcctRows)))
			break
		}
		m := b.Minor
		if m < 0 {
			m = -m
		}
		pct := 0
		if maxAbs > 0 {
			pct = int(m * 100 / maxAbs)
		}
		amtCls := "budget-amount"
		barCls := "share-bar-fill"
		if b.Acct.Class == domain.ClassLiability {
			amtCls += " " + tw.ColorClass("text-down")
			barCls += " nw-bar-down"
		}
		// The type meta disambiguates the name; drop it when it would just repeat
		// the name verbatim (e.g. an account literally named "Mortgage").
		typeLabel := selectorTypeLabel(b.Acct.Type)
		var meta ui.Node = Fragment()
		if !strings.EqualFold(typeLabel, b.Acct.Name) {
			meta = Span(css.Class("row-meta"), typeLabel)
		}
		acctRows = append(acctRows, Div(css.Class("row"), Attr("data-testid", "nw-acct-row"),
			Div(css.Class("row-main"),
				Span(css.Class("row-desc"), b.Acct.Name),
				meta,
				Div(css.Class("share-bar"), Div(css.Class(barCls), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)}))),
			),
			Span(ClassStr(amtCls), fmtMoney(money.New(b.Minor, base))),
		))
	}
	acctAction := A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/accounts")), Attr("data-testid", "networth-drill"), uistate.T("reports.viewAccounts"))
	acctTile := nwTile("nw-accounts", "1 / span 4", nwSection("sec-nw-accounts", uistate.T("nw.accountsTitle"), acctAction,
		Fragment(
			P(css.Class("muted"), uistate.T("nw.accountsHint")),
			Div(css.Class("rows"), acctRows),
		)))

	tiles := []ui.Node{heroTile, toolbar, trendTile, ownTile, oweTile, acctTile}
	if showFormulas.Get() {
		tiles = append(tiles, nwTile("nw-formula", "1 / span 4", Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("nw.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("nw.metricsShow"), ShowSaved: true}),
		)))
	}
	return Div(css.Class("bento bento-networth"), tiles)
}
