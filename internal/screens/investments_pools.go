// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// growthFigures returns the current value, the change vs the first point, and the percent
// change, for a growth series.
func growthFigures(series []money.Money) (cur, delta int64, pct float64) {
	if len(series) == 0 {
		return 0, 0, 0
	}
	cur = series[len(series)-1].Amount
	delta = cur - series[0].Amount
	if s := series[0].Amount; s != 0 {
		if s < 0 {
			s = -s
		}
		pct = float64(delta) / float64(s) * 100
	}
	return
}

// miniAreaChart renders a compact seagreen area chart for a growth series.
func miniAreaChart(series []money.Money, labels []string, base, sym string) ui.Node {
	pts := make([]chartspec.Point, len(series))
	for i, m := range series {
		lbl := ""
		if i < len(labels) {
			lbl = labels[i]
		}
		pts[i] = chartspec.Point{X: float64(i), Y: currency.MajorFromMinor(m.Amount, base), Label: lbl}
	}
	yFmt := ".2~s"
	if sym == "$" {
		yFmt = "$.2~s"
	}
	return uiw.Chart(uiw.ChartProps{
		Spec:   chartspec.Spec{Kind: chartspec.Area, Series: []chartspec.Series{{Color: "#2e8b57", Points: pts}}, Y: chartspec.Axis{Format: yFmt}},
		Height: "120px", CurrencySymbol: sym, Label: uistate.T("investments.growthChartLabel"),
	})
}

// growthCard renders a growth card: a header (name + any controls), the current value + a
// toned delta, and a mini area chart.
func growthCard(testid string, header ui.Node, series []money.Money, labels []string, sym string, dec int, base string) ui.Node {
	cur, delta, pct := growthFigures(series)
	tone := gainToneClass(delta)
	arrow := "▲"
	if delta < 0 {
		arrow = "▼"
	}
	return Div(css.Class("inv-pool-card"), Attr("data-testid", testid),
		Div(css.Class("inv-pool-card-head"),
			header,
			Div(css.Class("inv-pool-figs"),
				Span(css.Class("inv-pool-val", tw.FontDisplay), fmtSignedMoney(cur, sym, dec)),
				Span(ClassStr("inv-pool-delta "+tw.ColorClass(tone)),
					fmt.Sprintf("%s %s (%+.1f%%)", arrow, fmtSignedMoney(delta, sym, dec), pct)),
			),
		),
		miniAreaChart(series, labels, base, sym),
	)
}

// --- pool chip (create/manage a group; shows its variable name) -------------------

type investPoolChipProps struct {
	Pool       uistate.InvestPool
	ValueMinor int64
	Sym        string
	Dec        int
}

// investPoolChip renders one pool in the pools bar: its name + combined value, the variable
// name it exposes (pool_<slug>_value) for use elsewhere, and rename/delete actions.
func investPoolChip(props investPoolChipProps) ui.Node {
	p := props.Pool
	rename := ui.UseEvent(Prevent(func() {
		uistate.PromptModal(uistate.T("investments.renamePoolPrompt"), p.Name, func(n string) {
			if strings.TrimSpace(n) != "" {
				uistate.RenameInvestPool(p.ID, n)
				uistate.BumpDataRevision()
			}
		})
	}))
	del := ui.UseEvent(Prevent(func() {
		uistate.ConfirmModal(uistate.T("investments.deletePoolConfirm", p.Name), true, func(ok bool) {
			if ok {
				uistate.DeleteInvestPool(p.ID)
				uistate.BumpDataRevision()
			}
		})
	}))
	varName := "pool_" + engineenv.PoolVarSlug(p.Name) + "_value"
	return Div(css.Class("inv-pool-chip"), Attr("data-testid", "invest-pool-"+p.ID),
		Div(css.Class("inv-pool-chip-main"),
			Span(css.Class("inv-pool-chip-name"), p.Name),
			Span(css.Class("inv-pool-chip-val", tw.TextDim), fmtSignedMoney(props.ValueMinor, props.Sym, props.Dec)),
		),
		Span(css.Class("inv-pool-var"), Title(uistate.T("investments.poolVarHint")), varName),
		Button(css.Class("inv-pool-chip-btn"), Type("button"), Attr("data-testid", "invest-pool-rename-"+p.ID),
			Attr("aria-label", uistate.T("investments.renamePool")), Title(uistate.T("investments.renamePool")), OnClick(rename),
			uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W3, tw.H3))),
		Button(css.Class("inv-pool-chip-btn"), Type("button"), Attr("data-testid", "invest-pool-del-"+p.ID),
			Attr("aria-label", uistate.T("investments.deletePool")), Title(uistate.T("investments.deletePool")), OnClick(del),
			uiw.Icon(icon.Close, css.Class(tw.ShrinkO, tw.W3, tw.H3))),
	)
}

// --- per-account growth card (with a pool selector) ------------------------------

type investAccountGraphCardProps struct {
	Account     domain.Account
	Pools       []uistate.InvestPool
	CurrentPool string
	Series      []money.Money
	Labels      []string
	Sym         string
	Dec         int
	Base        string
}

// investAccountGraphCard is one account's growth card: name + type badge + a pool selector
// (to group it), then the value/delta and its own growth chart. Every account gets one.
func investAccountGraphCard(props investAccountGraphCardProps) ui.Node {
	a := props.Account
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("investments.ungrouped")}}
	for _, p := range props.Pools {
		opts = append(opts, uiw.SelectOption{Value: p.ID, Label: p.Name})
	}
	header := Div(css.Class("inv-acct-head"),
		Div(css.Class("inv-pool-title-row"),
			Span(css.Class("inv-pool-name"), a.Name),
			Span(css.Class("inv-chip inv-class"), investmentAccountTypeBadge(a.Type)),
		),
		uiw.SelectInput(uiw.SelectInputProps{Options: opts, Selected: props.CurrentPool, Class: "inv-acct-pool",
			OnChange:  func(v string) { uistate.AssignAccountToPool(a.ID, v); uistate.BumpDataRevision() },
			AriaLabel: fmt.Sprintf(uistate.T("investments.assignAria"), a.Name), TestID: "invest-assign-" + a.ID}),
	)
	return growthCard("invest-acct-"+a.ID, header, props.Series, props.Labels, props.Sym, props.Dec, props.Base)
}

// --- the tile --------------------------------------------------------------------

// investPoolsWidget shows a growth graph for EVERY investment account, plus a "pools" bar
// for grouping accounts into custom named pools. Each pool exposes a pool_<slug>_value
// engine variable usable anywhere (formulas, dashboard widgets). All charts share the 1M /
// 6M / 1Y window from the growth tile.
func investPoolsWidget(props investPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	v := computeInvestView(app)
	if !v.HasAny {
		return Fragment()
	}
	months := uistate.UseInvestGrowthMonths().Get()
	if months != 1 && months != 6 && months != 12 {
		months = 12
	}
	newPool := ui.UseEvent(Prevent(func() {
		uistate.PromptModal(uistate.T("investments.newPoolPrompt"), "", func(name string) {
			if strings.TrimSpace(name) != "" {
				uistate.AddInvestPool(id.NewWithPrefix("pool"), name)
				uistate.BumpDataRevision()
			}
		})
	}))

	investAccts := investAccountsOf(app)
	pools := uistate.InvestPools()
	poolOf := map[string]string{}
	for _, p := range pools {
		for _, aid := range p.AccountIDs {
			poolOf[aid] = p.ID
		}
	}
	now := time.Now()
	cutoffs, labels := growthCutoffs(now, months)
	rates := currency.Rates{Base: v.Base, Rates: app.Settings().FXRates}
	txns := app.Transactions()
	seriesFor := func(accts []domain.Account) []money.Money {
		s, _ := ledger.NetWorthSeries(accts, txns, cutoffs, rates)
		return s
	}
	// A pool's combined current value (base-currency balance sum of its member accounts).
	acctByID := map[string]domain.Account{}
	for _, a := range investAccts {
		acctByID[a.ID] = a
	}
	poolValue := func(p uistate.InvestPool) int64 {
		var total int64
		for _, aid := range p.AccountIDs {
			a, ok := acctByID[aid]
			if !ok {
				continue
			}
			if bal, err := ledger.Balance(a, txns); err == nil {
				if c, cerr := rates.Convert(bal, v.Base); cerr == nil {
					total += c.Amount
				} else {
					total += bal.Amount
				}
			}
		}
		return total
	}

	// Pools bar: a chip per pool + the New-pool action.
	var poolsBar ui.Node = Fragment()
	if len(pools) > 0 {
		chips := MapKeyed(pools, func(p uistate.InvestPool) any { return p.ID }, func(p uistate.InvestPool) ui.Node {
			return ui.CreateElement(investPoolChip, investPoolChipProps{Pool: p, ValueMinor: poolValue(p), Sym: v.Sym, Dec: v.Dec})
		})
		poolsBar = Div(css.Class("inv-pools-bar"),
			Span(css.Class("inv-pools-bar-label", tw.TextDim), uistate.T("investments.poolsBarLabel")),
			chips,
		)
	}

	// One growth card per account (always), with its pool selector.
	cards := MapKeyed(investAccts, func(a domain.Account) any { return a.ID }, func(a domain.Account) ui.Node {
		return ui.CreateElement(investAccountGraphCard, investAccountGraphCardProps{
			Account: a, Pools: pools, CurrentPool: poolOf[a.ID],
			Series: seriesFor([]domain.Account{a}), Labels: labels, Sym: v.Sym, Dec: v.Dec, Base: v.Base,
		})
	})

	newPoolBtn := Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "invest-new-pool"), Title(uistate.T("investments.newPool")), OnClick(newPool),
		uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("investments.newPool")))

	body := investSection("sec-pools", uistate.T("investments.poolsTitle"), newPoolBtn, Fragment(
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("investments.poolsHint")),
		poolsBar,
		Div(css.Class("inv-pool-grid"), cards),
	))
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-pools", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
