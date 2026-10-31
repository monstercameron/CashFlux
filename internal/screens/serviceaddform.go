// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/services"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// serviceProviderListID is the id of the datalist that suggests the household's institutions.
const serviceProviderListID = "service-provider-list"

// ServiceAddFormProps configures the ServiceAddForm component.
type ServiceAddFormProps struct {
	// OnDone is called after a successful add, so the caller can close the modal. On a validation
	// error the form stays open and OnDone is not called.
	OnDone func()
}

// ServiceAddForm is the add-a-service form: a name, the provider and the monthly cost. The service
// is saved as a monthly recurring bill, so its cost counts toward budgets and reports.
func ServiceAddForm(props ServiceAddFormProps) ui.Node {
	return ui.CreateElement(serviceAddForm, props)
}

func serviceAddForm(props ServiceAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	name := ui.UseState("")
	provider := ui.UseState("")
	monthly := ui.UseState("")
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onProvider := ui.UseEvent(func(v string) { provider.Set(v) })
	onMonthly := ui.UseEvent(func(v string) { monthly.Set(v) })

	add := ui.UseEvent(Prevent(func() {
		n := strings.TrimSpace(name.Get())
		if n == "" {
			errMsg.Set(uistate.T("services.nameRequired"))
			return
		}
		cur := app.Settings().BaseCurrency
		minor, err := money.ParseMinor(strings.TrimSpace(monthly.Get()), currency.Decimals(cur))
		if err != nil || minor <= 0 {
			errMsg.Set(uistate.T("services.amountRequired"))
			return
		}
		r := services.NewMonthly(services.Draft{ID: id.New(), Name: n, Provider: strings.TrimSpace(provider.Get()),
			Currency: cur, MonthlyMinor: minor, NextDue: dateutil.NextMonthlyDue(time.Now(), 1)})
		if err := app.PutRecurring(r); err != nil {
			errMsg.Set(err.Error())
			return
		}
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("services.added", n), false)
		name.Set("")
		provider.Set("")
		monthly.Set("")
		errMsg.Set("")
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	byID := map[string]domain.Institution{}
	for _, in := range app.Institutions() {
		byID[in.ID] = in
	}
	choices := institutionChoices(byID)

	return Fragment(
		Form(css.Class("form-grid"), Attr("id", "service-add-form"), Attr("data-testid", "service-add-form"), OnSubmit(add),
			labeledField(uistate.T("services.name"),
				Input(css.Class("field"), Attr("id", "service-add-name"), Type("text"), Attr("aria-label", uistate.T("services.name")),
					Attr("aria-required", "true"), Placeholder(uistate.T("services.namePlaceholder")), OnInput(onName), uiw.FieldValue(name.Get()))),
			labeledField(uistate.T("services.provider"),
				Div(
					Input(css.Class("field"), Attr("id", "service-add-provider"), Type("text"), Attr("aria-label", uistate.T("services.provider")),
						Attr("list", serviceProviderListID), Placeholder(uistate.T("services.providerPlaceholder")), OnInput(onProvider), uiw.FieldValue(provider.Get())),
					Datalist(Attr("id", serviceProviderListID), Map(choices, func(c string) ui.Node { return Option(Attr("value", c)) })),
				)),
			labeledField(uistate.T("services.monthly"),
				Input(css.Class("field"), Attr("id", "service-add-monthly"), Type("text"), Attr("inputmode", "decimal"),
					Attr("aria-label", uistate.T("services.monthly")), Attr("aria-required", "true"), Placeholder("0.00"), OnInput(onMonthly), uiw.FieldValue(monthly.Get()))),
			P(css.Class("muted"), Attr("data-testid", "service-explain"), uistate.T("services.explain")),
		),
		errText("service-err", errMsg.Get()),
	)
}
