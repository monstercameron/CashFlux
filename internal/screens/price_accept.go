// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// PriceAcceptProps configures the price-creep accept-flow card (XC5).
type PriceAcceptProps struct {
	RecurringID string
	Label       string
	OldMinor    int64 // prior expected amount (positive magnitude)
	NewMinor    int64 // detected new amount (positive magnitude)
	Currency    string
	// OnDone is called after the user accepts, makes a task, or cancels, so the
	// host can close the card.
	OnDone func()
}

// PriceAcceptCard renders the accept-the-new-price flow: it states the increase,
// previews the budget impact as two short lines (before / after — not a chart),
// and offers accept, accept-and-raise-budget, or make-it-a-task. Every path posts
// a plain-English confirmation and calls OnDone.
func PriceAcceptCard(props PriceAcceptProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return nil
	}
	imp := app.PriceCreepBudgetImpact(props.RecurringID, props.NewMinor)

	done := func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}
	onAccept := ui.UseEvent(func() {
		if err := app.AcceptNewPrice(props.RecurringID, props.NewMinor); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("pricecreep.accepted", props.Label,
			fmtMoney(money.New(props.NewMinor, props.Currency))), false)
		done()
	})
	onAcceptRaise := ui.UseEvent(func() {
		if err := app.AcceptNewPrice(props.RecurringID, props.NewMinor); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		if err := app.RaiseBudgetForCreep(props.RecurringID, props.NewMinor); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("pricecreep.accepted", props.Label,
			fmtMoney(money.New(props.NewMinor, props.Currency))), false)
		done()
	})
	onMakeTask := ui.UseEvent(func() {
		if err := app.CreatePriceCreepTask(props.RecurringID, props.OldMinor); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("pricecreep.taskCreated", props.Label), false)
		done()
	})
	onCancel := ui.UseEvent(func() { done() })

	var impactLines ui.Node
	if imp.HasBudget {
		impactLines = Fragment(
			P(css.Class("t-body", tw.TextDim), Attr("data-testid", "pricecreep-impact-before"),
				uistate.T("pricecreep.impactBefore", imp.BudgetName, imp.BeforePct)),
			P(css.Class("t-body"), Attr("data-testid", "pricecreep-impact-after"),
				uistate.T("pricecreep.impactAfter", imp.AfterPct)),
		)
	} else {
		impactLines = P(css.Class("t-body", tw.TextDim), uistate.T("pricecreep.impactNoBudget"))
	}

	return Div(css.Class("catchup-card"), Attr("role", "dialog"),
		Attr("data-testid", "pricecreep-accept"), Attr("aria-label", uistate.T("pricecreep.acceptTitle")),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol),
			Strong(uistate.T("pricecreep.acceptTitle")),
			P(uistate.T("pricecreep.crept", props.Label,
				fmtMoney(money.New(props.OldMinor, props.Currency)),
				fmtMoney(money.New(props.NewMinor, props.Currency)))),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mt2), impactLines),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-sm btn-primary"), Type("button"),
					Attr("data-testid", "pricecreep-accept-btn"), OnClick(onAccept),
					uistate.T("pricecreep.acceptPrice")),
				If(imp.HasBudget, Button(css.Class("btn btn-sm"), Type("button"),
					Attr("data-testid", "pricecreep-accept-raise"), OnClick(onAcceptRaise),
					uistate.T("pricecreep.acceptAndRaise"))),
				Button(css.Class("btn btn-sm"), Type("button"),
					Attr("data-testid", "pricecreep-make-task"), OnClick(onMakeTask),
					uistate.T("pricecreep.makeTask")),
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "pricecreep-cancel"), OnClick(onCancel),
					uistate.T("pricecreep.cancel")),
			),
		),
	)
}

// The SMART-BL16 insight's "Review the price" action navigates to /recurring,
// where priceCreepNotices (recurring_pricecreep.go) surfaces each detected
// creep with an in-place Review → PriceAcceptCard flow — the flag and the tool
// share one home surface.

// PriceCreepFor returns the first detected price-creep for a recurring (XC5),
// so a host surface can populate PriceAcceptProps without re-deriving detection.
// Returns ok=false when the recurring isn't currently crept.
func PriceCreepFor(app *appstate.App, recurringID string, weekStart time.Weekday) (smartengine.Creep, bool) {
	for _, c := range smartengine.DetectCreep(buildSmartInput(app, weekStart)) {
		if c.RecurringID == recurringID {
			return c, true
		}
	}
	return smartengine.Creep{}, false
}
