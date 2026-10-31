// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"log/slog"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountedit"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/institution"
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

// The ids of the service form's inputs and its error line. The inputs point at the error line
// through aria-describedby, and a refused save moves focus to the input that needs fixing.
const (
	serviceNameInputID     = "service-add-name"
	serviceProviderInputID = "service-add-provider"
	serviceMonthlyInputID  = "service-add-monthly"
	serviceErrID           = "service-err"
)

// ServiceAddFormProps configures the ServiceAddForm component.
type ServiceAddFormProps struct {
	// OnDone is called after a successful add, so the caller can close the modal. On a validation
	// error the form stays open and OnDone is not called.
	OnDone func()
}

// ServiceAddForm is the add-a-service form: a name, the provider and the monthly cost. The service
// is saved as a monthly recurring bill marked as a service. It appears only on the Accounts page and
// stays out of budgets, reports, forecasts and auto-posting.
func ServiceAddForm(props ServiceAddFormProps) ui.Node {
	return ui.CreateElement(serviceAddForm, props)
}

// serviceProviderChoices returns the names of the household's institutions, in the directory's
// order and without duplicates, for the provider suggestions.
func serviceProviderChoices(insts []domain.Institution) []string {
	byID := make(map[string]domain.Institution, len(insts))
	for _, in := range insts {
		byID[in.ID] = in
	}
	return institution.Choices(byID)
}

// serviceAddForm owns the form's state and handlers. Every hook runs before the check that the app
// is ready, so the hook count does not depend on whether the app has loaded.
func serviceAddForm(props ServiceAddFormProps) ui.Node {
	app := appstate.Default

	name := ui.UseState("")
	provider := ui.UseState("")
	monthly := ui.UseState("")
	errMsg := ui.UseState("")
	// errInput is the id of the input the message is about, or "" for an error about the save.
	errInput := ui.UseState("")
	focus := ui.UseFocusManager()

	// clearError drops the message once the input it is about changes, and a message about the save
	// itself (no input) once anything changes, so a fixed field never keeps its old complaint.
	clearError := func(inputID string) {
		if errMsg.Get() != "" && (errInput.Get() == inputID || errInput.Get() == "") {
			errMsg.Set("")
			errInput.Set("")
		}
	}
	onName := ui.UseEvent(func(v string) { name.Set(v); clearError(serviceNameInputID) })
	onProvider := ui.UseEvent(func(v string) { provider.Set(v); clearError(serviceProviderInputID) })
	onMonthly := ui.UseEvent(func(v string) { monthly.Set(v); clearError(serviceMonthlyInputID) })

	add := ui.UseEvent(Prevent(func() {
		cur := accountedit.BaseCurrencyOrDefault(app.Settings().BaseCurrency)
		draft, err := services.ParseDraft(name.Get(), provider.Get(), monthly.Get(), cur, time.Now())
		if err != nil {
			inputID, msg := "", uistate.T("services.addFailed")
			switch {
			case errors.Is(err, services.ErrNameRequired):
				inputID, msg = serviceNameInputID, uistate.T("services.nameRequired")
			case errors.Is(err, services.ErrAmountInvalid):
				inputID, msg = serviceMonthlyInputID, uistate.T("services.amountRequired")
			default:
				slog.Error("add service: read the form", "error", err)
			}
			errInput.Set(inputID)
			errMsg.Set(msg)
			if inputID != "" {
				focus.FocusByID(inputID)
			}
			return
		}
		draft.ID = id.New()
		if err := app.PutRecurring(services.NewMonthly(draft)); err != nil {
			slog.Error("add service", "service", draft.Name, "error", err)
			errInput.Set("")
			errMsg.Set(acctWriteErrorText(err))
			return
		}
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("services.added", draft.Name), false)
		name.Set("")
		provider.Set("")
		monthly.Set("")
		errMsg.Set("")
		errInput.Set("")
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	var choices []string
	amountPlaceholder := money.FormatMinor(0, currency.Decimals(accountedit.DefaultBaseCurrency))
	if app != nil {
		choices = serviceProviderChoices(app.Institutions())
		amountPlaceholder = money.FormatMinor(0, currency.Decimals(accountedit.BaseCurrencyOrDefault(app.Settings().BaseCurrency)))
	}
	// invalidName is the danger-outline class for the input a message is about; it goes in the
	// input's single css.Class call. invalid adds that input's aria-invalid and a pointer to the
	// error line.
	invalidName := func(inputID string) string {
		return condName(errMsg.Get() != "" && errInput.Get() == inputID, "field-invalid")
	}
	invalid := func(inputID string) []any {
		if errMsg.Get() == "" || errInput.Get() != inputID {
			return nil
		}
		return []any{Attr("aria-invalid", "true"), Attr("aria-describedby", serviceErrID)}
	}

	form := Form(css.Class("form-grid"), Attr("id", "service-add-form"), Attr("data-testid", "service-add-form"), OnSubmit(add),
		labeledField(uistate.T("services.name"),
			Input(append([]any{css.Class("field", invalidName(serviceNameInputID)), Attr("id", serviceNameInputID), Type("text"), Attr("aria-label", uistate.T("services.name")),
				Attr("aria-required", "true"), Placeholder(uistate.T("services.namePlaceholder")), OnInput(onName), uiw.FieldValue(name.Get())},
				invalid(serviceNameInputID)...)...)),
		labeledField(uistate.T("services.provider"),
			Div(
				Input(css.Class("field"), Attr("id", serviceProviderInputID), Type("text"), Attr("aria-label", uistate.T("services.provider")),
					Attr("list", serviceProviderListID), Placeholder(uistate.T("services.providerPlaceholder")), OnInput(onProvider), uiw.FieldValue(provider.Get())),
				Datalist(Attr("id", serviceProviderListID), Map(choices, func(c string) ui.Node { return Option(Attr("value", c)) })),
			)),
		labeledField(uistate.T("services.monthly"),
			Input(append([]any{css.Class("field", invalidName(serviceMonthlyInputID)), Attr("id", serviceMonthlyInputID), Type("text"), Attr("inputmode", "decimal"),
				Attr("aria-label", uistate.T("services.monthly")), Attr("aria-required", "true"), Placeholder(amountPlaceholder), OnInput(onMonthly), uiw.FieldValue(monthly.Get())},
				invalid(serviceMonthlyInputID)...)...)),
		P(css.Class("muted"), Attr("data-testid", "service-explain"), uistate.T("services.explain")),
		errText(serviceErrID, errMsg.Get()),
	)
	// The form is built before this check so its handlers register on every render.
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	return form
}
