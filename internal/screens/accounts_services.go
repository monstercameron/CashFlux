// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"log/slog"

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

// serviceCostLogged remembers the service figures whose failure is already in the log. The list
// renders again on every keystroke elsewhere on the page, so each failure is logged once.
var serviceCostLogged = map[string]bool{}

// logServiceCostOnce logs a failed service figure with its subject (the total, or one service) the
// first time that failure is seen, so the cause stays traceable without a line per render.
func logServiceCostOnce(subject string, err error) {
	key := subject + "|" + err.Error()
	if serviceCostLogged[key] {
		return
	}
	serviceCostLogged[key] = true
	slog.Error("calculate service cost", "subject", subject, "error", err)
}

// servicesUnavailableText is the text shown in place of a service figure that could not be
// calculated: "Unavailable" with the reason, such as a missing exchange rate.
func servicesUnavailableText(err error, subject string) string {
	logServiceCostOnce(subject, err)
	reason := uistate.T("services.reasonOther")
	if errors.Is(err, currency.ErrUnknownRate) {
		reason = uistate.T("services.reasonNoRate")
	}
	return uistate.T("services.unavailableReason", reason)
}

// utilityAccounts returns the household's utility, HOA and service accounts. They are services in
// the household's sense, so the Services view lists them with the repeating charges recorded
// against them.
func utilityAccounts(accounts []domain.Account) []domain.Account {
	var out []domain.Account
	for _, a := range accounts {
		if a.Type == domain.TypeUtilities {
			out = append(out, a)
		}
	}
	return out
}

// utilityCharges groups the repeating charges recorded against each utility account by account.
func utilityCharges(utilities []domain.Account, recs []domain.Recurring) map[string][]domain.Recurring {
	ids := make(map[string]bool, len(utilities))
	for _, a := range utilities {
		ids[a.ID] = true
	}
	out := make(map[string][]domain.Recurring)
	for _, r := range recs {
		if ids[r.AccountID] {
			out[r.AccountID] = append(out[r.AccountID], r)
		}
	}
	return out
}

// servicesBody lists the household's services and utility accounts under their monthly total. A
// utility account's charges count in that total. It is the body of the Accounts list while the
// Services filter is on.
func servicesBody(app *appstate.App, list []domain.Recurring, utilities []domain.Account, charges map[string][]domain.Recurring, base string, rates currency.Rates) ui.Node {
	if len(list) == 0 && len(utilities) == 0 {
		return uiw.Callout(uiw.CalloutProps{TestID: "acct-services-none", Body: Span(uistate.T("accounts.servicesNone"))})
	}
	all := append([]domain.Recurring(nil), list...)
	for _, a := range utilities {
		all = append(all, charges[a.ID]...)
	}
	var totalText string
	if total, err := services.MonthlyTotal(all, base, rates); err != nil {
		totalText = servicesUnavailableText(err, "services total")
	} else {
		totalText = fmtMoney(money.New(total, base))
	}
	return Div(css.Class("acct-services"),
		P(css.Class("acct-services-total"), Attr("data-testid", "acct-services-total"), uistate.T("accounts.servicesMonthly", totalText)),
		Div(css.Class("rows"),
			MapKeyedComponent(list, func(r domain.Recurring) any { return r.ID }, func(r domain.Recurring) ui.Node {
				return ui.CreateElement(serviceRow, serviceRowProps{App: app, Service: r, Base: base, Rates: rates})
			}),
			MapKeyedComponent(utilities, func(a domain.Account) any { return a.ID }, func(a domain.Account) ui.Node {
				return ui.CreateElement(utilityAccountRow, utilityAccountRowProps{Account: a, Charges: charges[a.ID], Base: base, Rates: rates})
			})),
	)
}

type utilityAccountRowProps struct {
	Account domain.Account
	Charges []domain.Recurring
	Base    string
	Rates   currency.Rates
}

// utilityAccountRow is one utility account in the Services view: its name, its type and currency,
// and what its repeating charges cost a month.
func utilityAccountRow(props utilityAccountRowProps) ui.Node {
	a := props.Account
	costText := uistate.T("accounts.servicesNoCharge")
	if len(props.Charges) > 0 {
		if monthly, err := services.MonthlyTotal(props.Charges, props.Base, props.Rates); err != nil {
			costText = servicesUnavailableText(err, "utility "+a.ID)
		} else {
			costText = uistate.T("services.perMonth", fmtMoney(money.New(monthly, props.Base)))
		}
	}
	return Div(css.Class("acct-entity"), Attr("data-testid", "acct-utility-"+a.ID),
		uiw.EntityRow(uiw.EntityRowProps{
			Title: a.Name,
			Meta:  []string{uistate.T("accounts.metaLine", humanizeType(string(a.Type)), a.Currency)},
			Value: Span(css.Class(tw.TextDim), Attr("data-testid", "acct-utility-cost-"+a.ID), costText),
		}),
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
				slog.Error("remove service", "service_id", svc.ID, "error", err)
				uistate.PostNotice(uistate.T("services.removeFailed"), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("services.removed", svc.Label), false)
		})
	}))

	var costText string
	if monthly, err := recurring.MonthlyBase(svc, props.Base, props.Rates); err != nil {
		costText = servicesUnavailableText(err, "service "+svc.ID)
	} else {
		costText = uistate.T("services.perMonth", fmtMoney(money.New(monthly, props.Base)))
	}
	var meta []string
	if svc.Provider != "" {
		meta = append(meta, svc.Provider)
	}
	return Div(css.Class("acct-entity"), Attr("data-testid", "acct-service-"+svc.ID),
		uiw.EntityRow(uiw.EntityRowProps{
			Title: svc.Label,
			Meta:  meta,
			Value: Span(css.Class(tw.TextDim), Attr("data-testid", "acct-service-cost-"+svc.ID), costText),
			Actions: []ui.Node{uiw.ToolButton(uiw.ToolButtonProps{Label: uistate.T("services.remove"), AriaLabel: uistate.T("services.removeNamed", svc.Label), Icon: icon.Trash, TestID: "acct-service-remove-" + svc.ID},
				OnClick(remove))},
		}),
	)
}
