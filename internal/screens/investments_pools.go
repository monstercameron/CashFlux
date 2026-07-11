// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
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
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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

// chartLineColor returns the color to draw growth charts in: the user's theme accent (so the
// charts track the active theme instead of a fixed hue), falling back to the default seagreen
// when no accent is configured. The D3 renderer paints this into SVG stroke/fill attributes,
// so it must be a resolved color value (not a CSS var()).
func chartLineColor(accent string) string {
	if accent = strings.TrimSpace(accent); accent != "" {
		return accent
	}
	return "#2e8b57"
}

// miniAreaChart renders a compact area chart for a growth series, stroked in the theme accent.
func miniAreaChart(series []money.Money, labels []string, base, sym, accent string) ui.Node {
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
		Spec:   chartspec.Spec{Kind: chartspec.Area, Series: []chartspec.Series{{Color: chartLineColor(accent), Points: pts}}, Y: chartspec.Axis{Format: yFmt}},
		Height: "120px", CurrencySymbol: sym, Label: uistate.T("investments.growthChartLabel"),
	})
}

// growthCard renders a growth card: a header (name + any controls), the current value + a
// toned delta, and a mini area chart drawn in the theme accent.
func growthCard(testid string, header ui.Node, series []money.Money, labels []string, sym string, dec int, base, accent string) ui.Node {
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
		miniAreaChart(series, labels, base, sym, accent),
	)
}

// --- modal trigger buttons (isolated subscribers) --------------------------------
//
// The pool-edit atom drives the create/edit-chart flip modal. Its trigger buttons are
// their OWN leaf components so that ONLY they (not the heavy pools grid or the per-chart
// cards) subscribe to the atom. Opening/closing the modal then re-renders just these tiny
// buttons plus the modal host — never the growth charts (each a NetWorthSeries computation).
// Before this split, every modal open/close rebuilt the whole grid, and that expensive
// re-render raced the host's close and left the modal intermittently open (P10 flake).

// newChartButton is the "New chart" toolbar button.
func newChartButton() ui.Node {
	poolEdit := uistate.UseInvestPoolEditID()
	open := ui.UseEvent(Prevent(func() { poolEdit.Set("new") }))
	return Button(css.Class("btn btn-sm btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "invest-new-pool"), Title(uistate.T("investments.newChartTitle")), OnClick(open),
		uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("investments.newChart")))
}

type poolEditButtonProps struct{ PoolID string }

// poolEditButton is one custom-chart card's edit (pencil) control. It still owns the
// pool-edit atom subscription (so only this leaf, not the whole grid, re-renders on modal
// open/close) but renders the standard IconButton.
func poolEditButton(props poolEditButtonProps) ui.Node {
	poolEdit := uistate.UseInvestPoolEditID()
	return uiw.IconButton(uiw.IconButtonProps{
		Icon:    icon.Pencil,
		Label:   uistate.T("investments.editPool"),
		Class:   "btn-sm btn-ghost",
		TestID:  "invest-pool-edit-" + props.PoolID,
		OnClick: func() { poolEdit.Set(props.PoolID) },
	})
}

// --- pool (custom chart) card ----------------------------------------------------

type investPoolChipProps struct {
	Pool    uistate.InvestPool
	Members int // count of member accounts
	Series  []money.Money
	Labels  []string
	Sym     string
	Dec     int
	Base    string
	Accent  string
}

// investPoolCard renders one pool as a CUSTOM CHART card: an aggregated growth chart of its
// member accounts, with the pool name, its member count, the pool_<slug>_value variable it
// exposes (for use elsewhere), and edit/delete actions.
func investPoolCard(props investPoolChipProps) ui.Node {
	p := props.Pool
	del := func() {
		uistate.ConfirmModal(uistate.T("investments.deletePoolConfirm", p.Name), true, func(ok bool) {
			if ok {
				uistate.DeleteInvestPool(p.ID)
				uistate.BumpDataRevision()
			}
		})
	}
	varName := "pool_" + engineenv.PoolVarSlug(p.Name) + "_value"
	header := Div(css.Class("inv-acct-head"),
		Div(css.Class("inv-pool-title-row"),
			Span(css.Class("inv-chart-tag"), uistate.T("investments.poolTag")),
			Span(css.Class("inv-pool-name"), p.Name),
			Span(css.Class("inv-pool-count", tw.TextDim), uistate.T("investments.poolCount", props.Members)),
			Div(css.Class("inv-pool-actions"),
				ui.CreateElement(poolEditButton, poolEditButtonProps{PoolID: p.ID}),
				uiw.DeleteButton(uiw.DeleteButtonProps{
					AriaLabel: uistate.T("investments.deletePool"),
					Title:     uistate.T("investments.deletePool"),
					TestID:    "invest-pool-del-" + p.ID,
					OnClick:   del,
				}),
			),
		),
		Span(css.Class("inv-pool-var"), Title(uistate.T("investments.poolVarHint")), varName),
	)
	return Div(css.Class("inv-chart-card"),
		growthCard("invest-pool-"+p.ID, header, props.Series, props.Labels, props.Sym, props.Dec, props.Base, props.Accent))
}

// --- per-account growth card -----------------------------------------------------

type investAccountCardProps struct {
	Account domain.Account
	Series  []money.Money
	Labels  []string
	Sym     string
	Dec     int
	Base    string
	Accent  string
	OnView  func(string)
}

// investAccountCard is one account's card: name + type badge + a "view transactions" link,
// its value/delta, and its own single-account growth chart. No pool selector — pool
// membership is managed only when creating/editing a pool (custom chart).
func investAccountCard(props investAccountCardProps) ui.Node {
	a := props.Account
	view := ui.UseEvent(Prevent(func() {
		if props.OnView != nil {
			props.OnView(a.ID)
		}
	}))
	header := Div(css.Class("inv-pool-title-row"),
		Span(css.Class("inv-pool-name"), a.Name),
		Span(css.Class("inv-chip inv-class"), investmentAccountTypeBadge(a.Type)),
		Button(css.Class("btn btn-sm btn-ghost inv-acct-view"), Type("button"), Attr("data-testid", "invest-acct-view-"+a.ID),
			Attr("aria-label", uistate.T("accounts.viewTitle")), Title(uistate.T("nav.transactions")), OnClick(view),
			uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
	)
	return growthCard("invest-acct-"+a.ID, header, props.Series, props.Labels, props.Sym, props.Dec, props.Base, props.Accent)
}

// --- pool editor flip modal ------------------------------------------------------

type poolAccountToggleProps struct {
	Account  domain.Account
	Checked  bool
	OnToggle func(string)
}

// poolAccountToggle is one checkable account row in the pool editor. Its own component so
// the per-row click hook is stable inside the account list.
func poolAccountToggle(props poolAccountToggleProps) ui.Node {
	a := props.Account
	toggle := ui.UseEvent(Prevent(func() { props.OnToggle(a.ID) }))
	cls := "pool-acct-toggle"
	if props.Checked {
		cls += " is-checked"
	}
	var checkMark ui.Node = Fragment()
	if props.Checked {
		checkMark = uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4))
	}
	return Button(ClassStr(cls), Type("button"), Attr("role", "checkbox"), Attr("aria-checked", ariaBool(props.Checked)),
		Attr("data-testid", "pool-acct-"+a.ID), OnClick(toggle),
		Span(css.Class("pool-acct-check"), Attr("aria-hidden", "true"), checkMark),
		Span(css.Class("pool-acct-name"), a.Name),
		Span(css.Class("inv-chip inv-class"), investmentAccountTypeBadge(a.Type)),
	)
}

// InvestPoolFormProps configures the create/edit-pool modal form.
type InvestPoolFormProps struct {
	ID     string // "new" (or "") to create, else the pool id to edit
	OnDone func()
}

// InvestPoolForm is the create/edit-pool flip-modal body: a name field and a checkable list
// of the investment accounts to include in the pool. Saving upserts the pool (an account
// belongs to one pool, so selecting it here moves it out of any other) and closes.
func InvestPoolForm(props InvestPoolFormProps) ui.Node {
	app := appstate.Default
	var accounts []domain.Account
	if app != nil {
		accounts = investAccountsOf(app)
	}
	isNew := props.ID == "" || props.ID == "new"
	var existing uistate.InvestPool
	if !isNew {
		for _, p := range uistate.InvestPools() {
			if p.ID == props.ID {
				existing = p
				break
			}
		}
	}

	nameS := ui.UseState(existing.Name)
	initSel := map[string]bool{}
	for _, aid := range existing.AccountIDs {
		initSel[aid] = true
	}
	selS := ui.UseState(initSel)
	errS := ui.UseState("")
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })

	toggle := func(aid string) {
		cur := selS.Get()
		next := make(map[string]bool, len(cur)+1)
		for k, val := range cur {
			next[k] = val
		}
		if next[aid] {
			delete(next, aid)
		} else {
			next[aid] = true
		}
		selS.Set(next)
	}

	save := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(nameS.Get())
		if name == "" {
			errS.Set(uistate.T("investments.poolNameRequired"))
			return
		}
		var ids []string
		for _, a := range accounts {
			if selS.Get()[a.ID] {
				ids = append(ids, a.ID)
			}
		}
		pid := props.ID
		if isNew {
			pid = id.NewWithPrefix("pool")
		}
		uistate.UpsertInvestPool(pid, name, ids)
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))
	toggles := MapKeyed(accounts, func(a domain.Account) any { return a.ID }, func(a domain.Account) ui.Node {
		return ui.CreateElement(poolAccountToggle, poolAccountToggleProps{Account: a, Checked: selS.Get()[a.ID], OnToggle: toggle})
	})

	return Div(css.Class("inv-pool-modal"),
		Form(css.Class("inv-pool-modal-form"), Attr("id", "invest-pool-form"), OnSubmit(save),
			labeledField(uistate.T("investments.poolNameLabel"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "pool-name"), Attr("autofocus", "true"),
					Placeholder(uistate.T("investments.poolNamePlaceholder")), Value(nameS.Get()), OnInput(onName))),
			Div(css.Class("pool-acct-list-label", tw.TextDim), uistate.T("investments.poolPickAccounts")),
			If(len(accounts) == 0, P(css.Class("empty"), uistate.T("investments.noAccountsBody"))),
			Div(css.Class("pool-acct-list"), toggles),
			If(errS.Get() != "", P(css.Class("err"), Attr("role", "alert"), errS.Get())),
		),
	)
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
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	onView := func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	_ = uistate.UsePrefs().Get() // re-render when the accent/theme preference changes
	accent := chartLineColor(uistate.CurrentAccent())
	investAccts := investAccountsOf(app)
	pools := uistate.InvestPools()
	now := time.Now()
	cutoffs, labels := growthCutoffs(now, months)
	rates := currency.Rates{Base: v.Base, Rates: app.Settings().FXRates}
	txns := app.Transactions()
	seriesFor := func(accts []domain.Account) []money.Money {
		s, _ := ledger.NetWorthSeries(accts, txns, cutoffs, rates)
		return s
	}
	acctByID := map[string]domain.Account{}
	for _, a := range investAccts {
		acctByID[a.ID] = a
	}

	// One card per account (its own single-account chart) followed by one card per pool (a
	// custom chart aggregating its member accounts).
	gridArgs := []any{css.Class("inv-pool-grid")}
	for _, a := range investAccts {
		ac := a
		gridArgs = append(gridArgs, ui.CreateElement(investAccountCard, investAccountCardProps{
			Account: ac, Series: seriesFor([]domain.Account{ac}), Labels: labels, Sym: v.Sym, Dec: v.Dec, Base: v.Base, Accent: accent, OnView: onView,
		}))
	}
	for _, p := range pools {
		pc := p
		var members []domain.Account
		for _, aid := range pc.AccountIDs {
			if a, ok := acctByID[aid]; ok {
				members = append(members, a)
			}
		}
		gridArgs = append(gridArgs, ui.CreateElement(investPoolCard, investPoolChipProps{
			Pool: pc, Members: len(members), Series: seriesFor(members), Labels: labels, Sym: v.Sym, Dec: v.Dec, Base: v.Base, Accent: accent,
		}))
	}

	body := investSection("sec-pools", uistate.T("investments.poolsTitle"), ui.CreateElement(newChartButton), Fragment(
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("investments.poolsHint")),
		Div(gridArgs...),
	))
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-pools", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
