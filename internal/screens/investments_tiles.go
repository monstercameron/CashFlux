// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/portfolio"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

type investPanelProps struct{ App *appstate.App }

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
			P(css.Class("inv-hero-sub", tw.TextDim),
				uistate.T("investments.splitLine",
					fmtSignedMoney(v.SecSummary.TotalValueMinor, v.Sym, v.Dec),
					fmtSignedMoney(v.TradValueMinor, v.Sym, v.Dec))),
			investOwnerLink("/networth", uistate.T("debt.linkNetWorth")),
		),
		chips,
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- invest-toolbar --------------------------------------------------------------

func investToolbarWidget(props investPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	addAtom := uistate.UseInvestAddOpen()
	formulasAtom := uistate.UseInvestShowFormulas()
	toggleAdd := ui.UseEvent(Prevent(func() { addAtom.Set(!addAtom.Get()) }))
	toggleFormulas := ui.UseEvent(Prevent(func() { formulasAtom.Set(!formulasAtom.Get()) }))

	formulasLabel := uistate.T("investments.metricsShow")
	if formulasAtom.Get() {
		formulasLabel = uistate.T("investments.metricsHide")
	}
	metricsCls := "strip-toggle"
	if formulasAtom.Get() {
		metricsCls += " is-on"
	}

	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(css.Class(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "invest-toggle-formulas"), Title(uistate.T("investments.metricsTitle")),
				OnClick(toggleFormulas), Text(formulasLabel)),
			A(css.Class("btn btn-ghost"), Href(uistate.RoutePath("/accounts")), uistate.T("debt.linkAccounts")),
		),
		Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "invest-add"), Title(uistate.T("investments.addHoldingTitle")), OnClick(toggleAdd),
			uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
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
	addOpen := uistate.UseInvestAddOpen().Get()
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

	onDelete := func(holdingID string) {
		name := uistate.T("investments.thisHolding")
		for _, h := range v.Securities {
			if h.ID == holdingID && h.Name != "" {
				name = h.Name
				break
			}
		}
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
			return ui.CreateElement(holdingRow, holdingRowProps{H: h, Sym: v.Sym, Dec: v.Dec, WeightPct: weight, OnDelete: onDelete})
		})
		listBody = Div(css.Class("inv-list"), rows)
	}

	var addNode ui.Node = Fragment()
	if addOpen {
		addNode = ui.CreateElement(addHoldingForm, addHoldingFormProps{Accounts: investAccountsOf(app), Sym: v.Sym, Dec: v.Dec})
	}

	body := investSection("sec-securities", uistate.T("investments.securitiesTitle"),
		investOwnerLink("/accounts", uistate.T("debt.linkAccounts")),
		Fragment(addNode, listBody))
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

// --- invest-traditional ----------------------------------------------------------

// investTraditionalWidget is the balance-tracked investment accounts tile — the "traditional"
// investments (a retirement account or brokerage you track as one value, not per security).
func investTraditionalWidget(props investPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	v := computeInvestView(app)
	if len(v.Traditional) == 0 {
		return Fragment()
	}
	onView := func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	rows := MapKeyed(v.Traditional, func(a domain.Account) any { return a.ID }, func(a domain.Account) ui.Node {
		return ui.CreateElement(traditionalRow, traditionalRowProps{Account: a, Balance: v.BalByID[a.ID], Sym: v.Sym, Dec: v.Dec, OnView: onView})
	})
	body := investSection("sec-traditional", uistate.T("investments.traditionalTitle"),
		investOwnerLink("/accounts", uistate.T("debt.linkAccounts")),
		Div(css.Class("inv-list"), rows))
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-traditional", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
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
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("investments.formulaHint")),
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("investments.metricsTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "invest-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
