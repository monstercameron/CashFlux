// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/portfolio"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

type investPanelProps struct{ App *appstate.App }

// investHasReconcile reports whether there is a securities-vs-account-balance
// relationship worth spelling out — i.e. some tracked securities exist. With no
// tracked securities there are not two competing "investment value" figures to
// reconcile, so the reconciliation copy is suppressed.
func investHasReconcile(v investView) bool {
	return v.Reconcile.SecuritiesMinor != 0
}

// investReconcileLine builds the one-line reconciliation equation shared by the
// /investments hero and the accounts investment banner: tracked securities plus
// cash & untracked balance equals the investment-account total that net worth uses.
// When recorded balances lag the holdings' market value it states that honestly
// instead of showing a negative "cash & untracked" figure.
func investReconcileLine(v investView) string {
	r := v.Reconcile
	sec := fmtSignedMoney(r.SecuritiesMinor, v.Sym, v.Dec)
	total := fmtSignedMoney(r.AccountsTotalMinor, v.Sym, v.Dec)
	if r.BalanceBehind {
		return uistate.T("investments.reconcileBehindLine", sec, total, fmtSignedMoney(-r.UntrackedMinor, v.Sym, v.Dec))
	}
	return uistate.T("investments.reconcileLine", sec, fmtSignedMoney(r.UntrackedMinor, v.Sym, v.Dec), total)
}

// investReconcileTitle builds the per-account reconciliation breakdown surfaced as
// the hover/title popover on the reconciliation line, one line per account.
func investReconcileTitle(v investView) string {
	lines := make([]string, 0, len(v.Reconcile.Accounts)+1)
	lines = append(lines, uistate.T("investments.reconcileTitle"))
	for _, a := range v.Reconcile.Accounts {
		name := a.Name
		if name == "" {
			name = uistate.T("investments.reconcileUnnamed")
		}
		if a.BalanceBehind {
			lines = append(lines, uistate.T("investments.reconcileAcctBehind",
				name, fmtSignedMoney(a.SecuritiesMinor, v.Sym, v.Dec), fmtSignedMoney(-a.UntrackedMinor, v.Sym, v.Dec)))
			continue
		}
		lines = append(lines, uistate.T("investments.reconcileAcctLine",
			name, fmtSignedMoney(a.SecuritiesMinor, v.Sym, v.Dec),
			fmtSignedMoney(a.UntrackedMinor, v.Sym, v.Dec), fmtSignedMoney(a.BalanceMinor, v.Sym, v.Dec)))
	}
	return strings.Join(lines, "\n")
}

// --- invest-summary --------------------------------------------------------------

// investSummaryWidget is the headline tile: total portfolio value in the display serif, the
// securities/traditional split, and gain / return / cost-basis chips (securities-based).
func investSummaryWidget(props investPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	v := computeInvestView(props.App)
	if !v.HasAny {
		return Fragment()
	}
	gainTone := gainToneClass(v.SecSummary.TotalGainMinor)

	chips := Div(css.Class("debt-chips"),
		Div(css.Class("debt-stat"),
			Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("investments.totalGain")),
			Div(ClassStr("debt-stat-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(gainTone)),
				fmtSignedMoney(v.SecSummary.TotalGainMinor, v.Sym, v.Dec))),
		Div(css.Class("debt-stat"),
			Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("investments.returnPct")),
			Div(ClassStr("debt-stat-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(gainTone)),
				fmt.Sprintf("%.2f%%", v.SecSummary.ReturnPct))),
		Div(css.Class("debt-stat"),
			Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("investments.totalCost")),
			Div(css.Class("debt-stat-value", tw.FontDisplay), fmtSignedMoney(v.SecSummary.TotalCostMinor, v.Sym, v.Dec))),
	)

	body := Div(css.Class("inv-hero"), Attr("id", "sec-overview"),
		Div(css.Class("inv-hero-main"),
			Div(css.Class("inv-hero-label", tw.TextDim), uistate.T("investments.portfolioValue")),
			Div(css.Class("inv-hero-value", tw.FontDisplay), Attr("data-testid", "invest-total"),
				fmtSignedMoney(v.TotalValueMinor, v.Sym, v.Dec)),
			If(investHasReconcile(v),
				Fragment(
					P(css.Class("inv-hero-sub", tw.TextDim), Attr("data-testid", "invest-reconcile"),
						Title(investReconcileTitle(v)), investReconcileLine(v)),
					P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0.05rem 0 0"}),
						uistate.T("investments.reconcileNetWorthNote")),
				)),
			If(!investHasReconcile(v),
				P(css.Class("inv-hero-sub", tw.TextDim),
					uistate.T("investments.splitLine",
						fmtSignedMoney(v.SecSummary.TotalValueMinor, v.Sym, v.Dec),
						fmtSignedMoney(v.TradValueMinor, v.Sym, v.Dec)))),
			investOwnerLink("/networth", uistate.T("debt.linkNetWorth")),
		),
		chips,
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- invest-growth ---------------------------------------------------------------

// growthCutoffs returns the timeline points (and their captions) for the growth chart over
// the given window: weekly points for a 1-month view, monthly points for 6/12 months. The
// last cutoff is now+1 day so today's activity is included in the final value.
func growthCutoffs(now time.Time, months int) ([]time.Time, []string) {
	if months <= 1 {
		cs := make([]time.Time, 0, 5)
		labels := make([]string, 0, 5)
		for i := 4; i >= 1; i-- {
			d := now.AddDate(0, 0, -7*i)
			cs = append(cs, d)
			labels = append(labels, d.Format("Jan 2"))
		}
		cs = append(cs, now.AddDate(0, 0, 1))
		labels = append(labels, now.Format("Jan 2"))
		return cs, labels
	}
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	cs := make([]time.Time, 0, months+1)
	labels := make([]string, 0, months+1)
	for i := months; i >= 0; i-- {
		m := first.AddDate(0, -i, 0)
		cs = append(cs, m)
		labels = append(labels, m.Format("Jan"))
	}
	cs[len(cs)-1] = now.AddDate(0, 0, 1)
	labels[len(labels)-1] = now.Format("Jan")
	return cs, labels
}

// investGrowthWidget charts the investment portfolio's value over time with a 1M / 6M / 1Y
// window toggle. The series is the investment accounts' recorded value at each point (via
// the ledger), so it reflects contributions and value updates — the honest growth history.
func investGrowthWidget(props investPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	monthsAtom := uistate.UseInvestGrowthMonths()
	months := monthsAtom.Get()
	if months != 1 && months != 6 && months != 12 {
		months = 12
	}

	app := props.App
	v := computeInvestView(app)
	if !v.HasAny {
		return Fragment()
	}
	base := v.Base
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	var investAccts []domain.Account
	for _, a := range app.Accounts() {
		if !a.Archived && isInvestmentAccount(a.Type) {
			investAccts = append(investAccts, a)
		}
	}
	now := time.Now()
	cutoffs, labels := growthCutoffs(now, months)
	series, _ := ledger.NetWorthSeries(investAccts, app.Transactions(), cutoffs, rates)

	pts := make([]chartspec.Point, len(series))
	valueLabels := make([]string, len(series))
	for i, m := range series {
		pts[i] = chartspec.Point{X: float64(i), Y: currency.MajorFromMinor(m.Amount, base), Label: labels[i]}
		valueLabels[i] = fmtSignedMoney(m.Amount, v.Sym, v.Dec)
	}

	var startV, endV int64
	if len(series) > 0 {
		startV = series[0].Amount
		endV = series[len(series)-1].Amount
	}
	delta := endV - startV
	deltaPct := 0.0
	if startV != 0 {
		d := delta
		s := startV
		if s < 0 {
			s = -s
		}
		deltaPct = float64(d) / float64(s) * 100
	}
	tone := gainToneClass(delta)
	arrow := "▲"
	if delta < 0 {
		arrow = "▼"
	}
	yFmt := ".2~s"
	if v.Sym == "$" {
		yFmt = "$.2~s"
	}

	// Standard Segmented control (role=radiogroup, sliding pill, keyboard nav) for the
	// 1M / 6M / 1Y window — same primitive the rest of the app uses for time-resolution.
	segToggle := uiw.Segmented(uiw.SegmentedProps{
		Label:    uistate.T("investments.growthWindow"),
		Selected: strconv.Itoa(months),
		Options: []uiw.SegOption{
			{Value: "1", Label: uistate.T("investments.win1m"), TestID: "invest-growth-1m"},
			{Value: "6", Label: uistate.T("investments.win6m"), TestID: "invest-growth-6m"},
			{Value: "12", Label: uistate.T("investments.win12m"), TestID: "invest-growth-12m"},
		},
		OnSelect: func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				monthsAtom.Set(n)
			}
		},
	})

	head := Div(css.Class("inv-growth-head"),
		Div(css.Class("inv-growth-vals"),
			Span(css.Class("inv-growth-now", tw.FontDisplay), fmtSignedMoney(endV, v.Sym, v.Dec)),
			Span(ClassStr("inv-growth-delta "+tw.ColorClass(tone)),
				fmt.Sprintf("%s %s (%+.1f%%)", arrow, fmtSignedMoney(delta, v.Sym, v.Dec), deltaPct)),
		),
		segToggle,
	)

	_ = uistate.UsePrefs().Get() // re-render when the accent/theme preference changes
	accent := chartLineColor(uistate.CurrentAccent())
	chart := uiw.Chart(uiw.ChartProps{
		Spec: chartspec.Spec{Kind: chartspec.Area, Series: []chartspec.Series{
			{Name: uistate.T("investments.portfolioValue"), Color: accent, Points: pts},
		}, Y: chartspec.Axis{Format: yFmt}},
		Height: "240px", CurrencySymbol: v.Sym,
		Label: uistate.T("investments.growthChartLabel"),
	})
	_ = valueLabels

	body := investSection("sec-growth", uistate.T("investments.growthTitle"), Fragment(),
		Div(css.Class("inv-growth"), head,
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "invest-growth-caption"),
				Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("investments.growthCaption")),
			chart))
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-growth", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- invest-toolbar --------------------------------------------------------------

func investToolbarWidget(props investPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	addAtom := uistate.UseInvestAddOpen()
	formulasAtom := uistate.UseInvestShowFormulas()
	openAdd := ui.UseEvent(Prevent(func() { addAtom.Set(true) }))
	toggleFormulas := ui.UseEvent(Prevent(func() { formulasAtom.Set(!formulasAtom.Get()) }))

	formulasLabel := uistate.T("investments.formulaBuilderShow")
	if formulasAtom.Get() {
		formulasLabel = uistate.T("investments.formulaBuilderHide")
	}
	metricsCls := "strip-toggle"
	if formulasAtom.Get() {
		metricsCls += " is-on"
	}

	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(css.Class(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "invest-toggle-formulas"), Title(uistate.T("investments.formulaBuilderTitle")),
				OnClick(toggleFormulas), Text(formulasLabel)),
			A(css.Class("btn btn-ghost"), Href(uistate.RoutePath("/accounts")), uistate.T("debt.linkAccounts")),
		),
		Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "invest-add"), Title(uistate.T("investments.addHoldingTitle")), OnClick(openAdd),
			uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("investments.addSecurity"))),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
	})
}

// --- invest-securities -----------------------------------------------------------

// investSecuritiesWidget is the per-ticker holdings tile: the security cards (or a first-run
// CTA / "add your first security" hint), plus the reveal-on-demand add-holding form.
func investSecuritiesWidget(props investPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	v := computeInvestView(app)

	// No investment accounts at all → send the user to add one first.
	if !v.HasAny {
		body := investSection("sec-securities", uistate.T("investments.securitiesTitle"),
			investOwnerLink("/accounts", uistate.T("debt.linkAccounts")),
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{
				Message: uistate.T("investments.noAccountsBody"), CTALabel: uistate.T("investments.addAccount"),
				AddTarget: "account", Icon: icon.TrendingUp}))
		return uiw.Widget(uiw.WidgetProps{
			ID: "invest-securities", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
			Body: body,
		})
	}

	holdingName := func(holdingID string) string {
		for _, h := range v.Securities {
			if h.ID == holdingID && h.Name != "" {
				return h.Name
			}
		}
		return uistate.T("investments.thisHolding")
	}
	onDelete := func(holdingID string) {
		name := holdingName(holdingID)
		uistate.ConfirmModal(uistate.T("investments.deleteConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteHolding(holdingID); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
		})
	}
	// Closing is the sold-the-position path: same removal, but framed (and
	// confirmed) as a close, with a nudge to record the sale proceeds so the
	// account's cash reflects it.
	onClose := func(holdingID string) {
		name := holdingName(holdingID)
		uistate.ConfirmModalLabeled(uistate.T("investments.closeConfirm", name), uistate.T("investments.closeConfirmBtn"), false, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteHolding(holdingID); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.PostNotice(uistate.T("investments.closedNotice", name), false)
			uistate.BumpDataRevision()
		})
	}

	var listBody ui.Node
	if len(v.Securities) == 0 {
		listBody = P(css.Class("empty"), Attr("data-testid", "invest-no-securities"), uistate.T("investments.emptyHoldings"))
	} else {
		total := v.SecSummary.TotalValueMinor
		rows := MapKeyed(v.Securities, func(h domain.Holding) any { return h.ID }, func(h domain.Holding) ui.Node {
			weight := 0.0
			if total != 0 {
				weight = float64(portfolio.HoldingValueMinor(portfolio.FromDomain(h))) / float64(total) * 100
			}
			return ui.CreateElement(holdingRow, holdingRowProps{H: h, Sym: v.Sym, Dec: v.Dec, WeightPct: weight, OnClose: onClose, OnDelete: onDelete})
		})
		listBody = Div(css.Class("inv-list"), rows)
	}

	// The add-security form is a shell-root flip modal (InvestAddHost), opened by the
	// toolbar's Add button — not rendered inline here.
	body := investSection("sec-securities", uistate.T("investments.securitiesTitle"),
		investOwnerLink("/accounts", uistate.T("debt.linkAccounts")),
		listBody)
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-securities", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// investAccountsOf returns the active investment accounts (for the add-holding picker).
func investAccountsOf(app *appstate.App) []domain.Account {
	var out []domain.Account
	for _, a := range app.Accounts() {
		if !a.Archived && isInvestmentAccount(a.Type) {
			out = append(out, a)
		}
	}
	return out
}

// --- invest-allocation -----------------------------------------------------------

// investAllocationWidget shows the securities allocation two ways: by security type and by
// asset class, as labelled weight bars.
func investAllocationWidget(props investPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	v := computeInvestView(props.App)
	if len(v.Securities) == 0 {
		return Fragment()
	}
	body := investSection("sec-allocation", uistate.T("investments.allocationTitle"), Fragment(),
		Div(css.Class("inv-alloc-cols"),
			allocColumn(uistate.T("investments.byType"), v.AllocType, v.Sym, v.Dec, true),
			allocColumn(uistate.T("investments.byClass"), v.AllocClass, v.Sym, v.Dec, false),
		))
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-allocation", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// allocColumn renders one titled allocation list (weight bars). When typed, the label is
// mapped through the security-type i18n labels; otherwise the raw asset-class label is shown.
func allocColumn(title string, weights []portfolio.Weight, sym string, dec int, typed bool) ui.Node {
	rows := []any{css.Class("inv-alloc-list")}
	for _, w := range weights {
		label := w.Label
		if typed {
			label = securityTypeLabel(domain.SecurityType(w.Label))
		} else if label == "other" || label == "" {
			label = uistate.T("investments.assetClassOther")
		}
		pct := w.Pct
		if pct > 100 {
			pct = 100
		}
		rows = append(rows, Div(css.Class("inv-alloc-row"),
			Div(css.Class("inv-alloc-head"),
				Span(css.Class("inv-alloc-label"), label),
				Span(css.Class("inv-alloc-val", tw.TextDim), fmt.Sprintf("%.1f%% · %s", w.Pct, fmtSignedMoney(w.ValueMinor, sym, dec)))),
			Div(css.Class("inv-alloc-track"),
				Div(css.Class("inv-alloc-fill"), Attr("style", fmt.Sprintf("width:%.1f%%", pct)))),
		))
	}
	return Div(css.Class("inv-alloc-col"),
		Div(css.Class("inv-alloc-title", tw.TextDim), title),
		Div(rows...),
	)
}

// --- invest-formula --------------------------------------------------------------

func investFormulaWidget(props investPanelProps) ui.Node {
	body := Div(
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("investments.formulaBuilderHint")),
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("investments.formulaBuilderShow"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
