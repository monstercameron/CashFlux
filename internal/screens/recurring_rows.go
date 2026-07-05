// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// recurUpcomingRow renders one derived due date in the next-30-days schedule: a
// calendar medallion (ghosted for the second+ row of the same day, so a date isn't
// re-stamped per line), the flow's name (+ autopay pill), and the toned amount.
// Display-only (no hooks), safe inside the loop.
func recurUpcomingRow(occ recurOccurrence, showDate bool) ui.Node {
	r := occ.R
	rowCls := "rec-up-row"
	if occ.Overdue {
		rowCls += " is-overdue"
	}
	dateCls := "rec-up-date"
	if !showDate {
		dateCls += " is-ghost"
	}
	return Div(ClassStr(rowCls), Attr("role", "listitem"), Attr("data-testid", "recurring-up-"+r.ID),
		Div(ClassStr(dateCls), Attr("aria-hidden", "true"),
			Span(css.Class("rec-up-day", tw.FontDisplay), occ.Date.Format("2")),
			Span(css.Class("rec-up-mon", tw.TextDim), occ.Date.Format("Jan")),
		),
		Div(css.Class("rec-up-main"),
			Span(css.Class("rec-up-name"), r.Label),
			Div(css.Class("rec-up-tags"),
				If(occ.Overdue, Span(css.Class("rec-tag rec-tag-overdue"), uistate.T("recurring.overdue"))),
				If(r.Autopay, Span(css.Class("rec-tag"), Title(uistate.T("recurring.autopayHint")), uistate.T("recurring.autopayBadge"))),
			),
		),
		Span(ClassStr("rec-up-amount "+recurAmountTone(r.Amount)), fmtMoney(r.Amount)),
	)
}

// recurAmountTone returns the semantic tone class for a signed money amount.
func recurAmountTone(m money.Money) string {
	if m.IsNegative() {
		return tw.ColorClass("text-down")
	}
	return tw.ColorClass("text-up")
}

// recurFlowCardProps drives one scheduled-flow card.
type recurFlowCardProps struct {
	R        domain.Recurring
	Base     string
	OutTotal int64 // Σ monthly outflow, for the share meter
	// VarPrefix is the flow's engine-variable prefix ("recurring_<slug>_") — its
	// stable identity on the formula surface, shown on the card.
	VarPrefix string
	// HasBudget is true when the flow's category has a budget (enables View budget).
	HasBudget bool
	Actions   recurFlowActions
}

// recurFlowCard renders one recurring flow as a card: a cadence tag + name +
// autopay/auto-post pills, the next due date, the flow's formula identity
// (recurring_<slug>_monthly), the per-month equivalent in the display serif (toned
// by sign), a share-of-outflow meter for expenses, and a ⋯ menu that interconnects
// the flow with its homes — Edit, View transactions (pre-filtered to its
// account/category), View budget (when its category is budgeted), View account,
// Delete. Its own component so the menu hooks stay stable in the list.
func recurFlowCard(props recurFlowCardProps) ui.Node {
	r := props.R
	me := r.MonthlyEquivalent()

	editItem := ui.UseEvent(Prevent(func() {
		if props.Actions.OnEdit != nil {
			props.Actions.OnEdit(r.ID)
		}
	}))
	delItem := ui.UseEvent(Prevent(func() {
		if props.Actions.OnDelete != nil {
			props.Actions.OnDelete(r.ID)
		}
	}))
	viewAcctItem := ui.UseEvent(Prevent(func() {
		if props.Actions.OnViewAccount != nil {
			props.Actions.OnViewAccount(r.AccountID)
		}
	}))
	viewTxnsItem := ui.UseEvent(Prevent(func() {
		if props.Actions.OnViewTxns != nil {
			props.Actions.OnViewTxns(r)
		}
	}))
	viewBudgetItem := ui.UseEvent(Prevent(func() {
		if props.Actions.OnViewBudget != nil {
			props.Actions.OnViewBudget(r)
		}
	}))

	items := []ui.Node{
		Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "recurring-edit-"+r.ID), Title(uistate.T("recurring.editTitle")),
			OnClick(editItem), uistate.T("recurring.editTitle")),
		Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "recurring-viewtxns-"+r.ID), Title(uistate.T("recurring.viewTxnsTitle")),
			OnClick(viewTxnsItem), uistate.T("recurring.viewTxns")),
	}
	if props.HasBudget {
		items = append(items, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "recurring-viewbudget-"+r.ID), OnClick(viewBudgetItem), uistate.T("recurring.viewBudget")))
	}
	if r.AccountID != "" {
		items = append(items, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "recurring-viewacct-"+r.ID), OnClick(viewAcctItem), uistate.T("recurring.viewAccount")))
	}
	items = append(items, Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"),
		Attr("data-testid", "recurring-del-"+r.ID), Title(uistate.T("recurring.deleteTitle")),
		OnClick(delItem), uistate.T("recurring.deleteTitle")))

	// Share of monthly outflow — only meaningful for expenses with any outflow at
	// all. Neutral (dim) fill: green is reserved for money-in on this page, and a
	// green bar beside a red outflow figure read as a contradiction.
	var meter ui.Node = Fragment()
	if me < 0 && props.OutTotal > 0 {
		pct := float64(-me) / float64(props.OutTotal) * 100
		meter = Div(css.Class("rec-flow-share"),
			uiw.MeterBar(uiw.MeterBarProps{Value: pct, Tone: "bg-dim", Label: uistate.T("recurring.shareOfOut")}),
			Span(css.Class("rec-flow-share-label", tw.TextDim), uistate.T("recurring.shareLabel", pct)),
		)
	}

	nextDue := uistate.T("recurring.nextDue", uistate.LoadPrefs().FormatDate(r.NextDue))
	perOccurrence := fmtMoney(r.Amount) + " " + recurCadence(r.Cadence)

	// The flow's formula identity — the exact variable name a formula or dashboard
	// widget can reference (mirrors the pool_<slug>_value chip on /investments).
	var varChip ui.Node = Fragment()
	if props.VarPrefix != "" {
		varChip = Span(css.Class("rec-flow-var"), Title(uistate.T("recurring.varHint")), props.VarPrefix+"monthly")
	}

	return Div(css.Class("rec-flow"), Attr("role", "listitem"), Attr("data-testid", "recurring-flow-"+r.ID),
		Span(css.Class("rec-cad-tag"), Attr("aria-hidden", "true"), Title(recurCadence(r.Cadence)),
			uiw.Icon(icon.Repeat, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Div(css.Class("rec-flow-body"),
			Div(css.Class("rec-flow-head"),
				Span(css.Class("rec-flow-name"), r.Label),
				If(r.Autopay, Span(css.Class("rec-tag"), Attr("data-testid", "recurring-autopay"), Title(uistate.T("recurring.autopayHint")), uistate.T("recurring.autopayBadge"))),
				If(r.Autopost, Span(css.Class("rec-tag"), Title(uistate.T("recurring.autopostHint")), uistate.T("recurring.autopostBadge"))),
				varChip,
			),
			Div(css.Class("rec-flow-meta", tw.TextDim), perOccurrence, Span(css.Class("rec-sep"), " · "), nextDue),
			meter,
		),
		Div(css.Class("rec-flow-figs"),
			Span(ClassStr("rec-flow-monthly "+tw.Fold(tw.FontDisplay)+" "+recurAmountTone(money.New(me, props.Base))),
				uistate.T("recurring.perMonth", fmtMoney(money.New(me, props.Base)))),
		),
		uiw.KebabMenu(uiw.KebabMenuProps{
			ID:           "recurring-menu-" + r.ID,
			AriaLabel:    uistate.T("recurring.moreActions"),
			ToggleTestID: "recurring-menu-" + r.ID,
			WrapClass:    "rec-flow-menu",
			Items:        items,
		}),
	)
}

// recurDetectedRowProps drives one auto-detected charge row.
type recurDetectedRowProps struct {
	Name  string
	Meta  string
	OnAdd func()
}

// recurDetectedRow renders one detected-but-unplanned charge with a one-click
// "Add to plan". Its own component so the button's hook sits at a stable position.
func recurDetectedRow(props recurDetectedRowProps) ui.Node {
	add := ui.UseEvent(Prevent(func() {
		if props.OnAdd != nil {
			props.OnAdd()
		}
	}))
	return Div(css.Class("rec-detected"),
		uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextDim)),
		Div(css.Class("rec-detected-main"),
			Span(css.Class("rec-flow-name"), props.Name),
			Span(css.Class("rec-flow-meta", tw.TextDim), props.Meta),
		),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "detected-add"),
			Attr("aria-label", uistate.T("recurring.addToPlanAria", props.Name)),
			Title(uistate.T("recurring.addToPlan")), OnClick(add), uistate.T("recurring.addToPlan")),
	)
}
