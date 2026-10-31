// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgetview/recurring"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/services"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// serviceClassView is the class-filter value that shows the household's services in place of its
// accounts.
const serviceClassView = "service"

// servicesBody lists the household's services under their monthly total. It is the body of the
// Accounts list while the Services filter is on.
func servicesBody(app *appstate.App, list []domain.Recurring, base string, rates currency.Rates) ui.Node {
	if len(list) == 0 {
		return P(css.Class("empty"), Attr("data-testid", "acct-services-none"), uistate.T("accounts.servicesNone"))
	}
	total, err := services.MonthlyTotal(list, base, rates)
	totalText := uistate.T("calculation.unavailable")
	if err == nil {
		totalText = fmtMoney(money.New(total, base))
	}
	return Div(css.Class("acct-services"),
		P(css.Class("acct-services-total"), Attr("data-testid", "acct-services-total"), uistate.T("accounts.servicesMonthly", totalText)),
		Div(css.Class("rows"), MapKeyedComponent(list, func(r domain.Recurring) any { return r.ID }, func(r domain.Recurring) ui.Node {
			return ui.CreateElement(serviceRow, serviceRowProps{App: app, Service: r, Base: base, Rates: rates})
		})),
	)
}

type serviceRowProps struct {
	App     *appstate.App
	Service domain.Recurring
	Base    string
	Rates   currency.Rates
}

// serviceRow is one service: its name and provider, its monthly cost in the base currency, and a
// remove action. Each row is its own component, so its click handler keeps a stable position.
func serviceRow(props serviceRowProps) ui.Node {
	svc := props.Service
	remove := ui.UseEvent(Prevent(func() {
		uistate.ConfirmModal(uistate.T("services.removeConfirm", svc.Label), true, func(ok bool) {
			if !ok {
				return
			}
			if err := props.App.DeleteRecurring(svc.ID); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("services.removed", svc.Label), false)
		})
	}))

	costText := uistate.T("calculation.unavailable")
	if monthly, err := recurring.MonthlyBase(svc, props.Base, props.Rates); err == nil {
		costText = uistate.T("services.perMonth", fmtMoney(money.New(monthly, props.Base)))
	}
	return Div(css.Class("row acct-service-row"), Attr("data-testid", "acct-service-"+svc.ID),
		Div(css.Class("acct-row-id"),
			Span(css.Class("row-desc"), svc.Label),
			If(svc.Provider != "", Span(css.Class("row-meta"), svc.Provider)),
		),
		Span(css.Class("acct-service-cost", tw.TextDim), Attr("data-testid", "acct-service-cost-"+svc.ID), costText),
		uiw.ToolButton(uiw.ToolButtonProps{Label: uistate.T("services.remove"), Icon: icon.Trash, TestID: "acct-service-remove-" + svc.ID},
			OnClick(remove)),
	)
}
