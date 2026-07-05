// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// PlanAddFormProps configures the add-scenario modal form.
type PlanAddFormProps struct {
	OnDone func() // called to close the modal (Cancel / backdrop)
}

// PlanAddForm is the "Add a savings & spending plan" form rendered inside the /planning
// add-scenario FlipPanel. It is a self-contained component so its many input hooks sit at
// stable render positions and so the scenarios tile stays a thin surface with just an
// "Add plan" trigger. Saving a valid plan persists it (app.PutPlan), bumps the data
// revision so the scenarios list behind the modal updates live, then resets the fields and
// shows a brief confirmation so several plans can be entered in a row; Cancel calls OnDone.
func PlanAddForm(props PlanAddFormProps) ui.Node {
	app := appstate.Default
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}

	name := ui.UseState("")
	horizon := ui.UseState("12")
	// account prefills the starting balance from a chosen account's current balance;
	// selecting one overwrites start with that account's ledger balance (L27).
	account := ui.UseState("")
	start := ui.UseState("")
	monthly := ui.UseState("")
	onceAmt := ui.UseState("")
	onceMonth := ui.UseState("")
	errS := ui.UseState("")
	savedS := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onHorizon := ui.UseEvent(func(v string) { horizon.Set(v) })
	onStart := ui.UseEvent(func(v string) { start.Set(v) })
	onMonthly := ui.UseEvent(func(v string) { monthly.Set(v) })
	onOnceAmt := ui.UseEvent(func(v string) { onceAmt.Set(v) })
	onOnceMonth := ui.UseEvent(func(v string) { onceMonth.Set(v) })
	onAccount := ui.UseEvent(func(e ui.Event) {
		aid := e.GetValue()
		account.Set(aid)
		if app == nil || aid == "" {
			return
		}
		for _, a := range app.Accounts() {
			if a.ID != aid {
				continue
			}
			if bal, err := ledger.Balance(a, app.Transactions()); err == nil {
				start.Set(money.FormatMinor(bal.Abs().Amount, currency.Decimals(a.Currency)))
			}
			return
		}
	})

	save := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		nm := strings.TrimSpace(name.Get())
		if nm == "" {
			errS.Set(uistate.T("plans.nameRequired"))
			return
		}
		months, herr := strconv.Atoi(strings.TrimSpace(horizon.Get()))
		if herr != nil || months <= 0 {
			errS.Set(uistate.T("plans.horizonRequired"))
			return
		}
		// Start balance and monthly change are optional; blank means 0.
		startMinor, _ := money.ParseMinor(strings.TrimSpace(start.Get()), currency.Decimals(base))
		monthlyMinor, _ := money.ParseMinor(strings.TrimSpace(monthly.Get()), currency.Decimals(base))
		p := domain.Plan{ID: id.New(), Name: nm, HorizonMonths: months, StartBalance: startMinor}
		if monthlyMinor != 0 {
			p.Items = append(p.Items, domain.PlanItem{
				ID: id.New(), Label: uistate.T("plans.monthlyLabel"), Kind: domain.PlanItemRecurring, Amount: monthlyMinor,
			})
		}
		// Optional one-time amount in a chosen month (e.g. a bonus or big expense).
		// Only added when both an amount and an in-horizon month are given.
		onceMinor, _ := money.ParseMinor(strings.TrimSpace(onceAmt.Get()), currency.Decimals(base))
		onceM, monthErr := strconv.Atoi(strings.TrimSpace(onceMonth.Get()))
		if onceMinor != 0 && strings.TrimSpace(onceMonth.Get()) != "" {
			if monthErr != nil || onceM < 1 || onceM > months {
				errS.Set(uistate.T("plans.onceMonthRange"))
				return
			}
			p.Items = append(p.Items, domain.PlanItem{
				ID: id.New(), Label: uistate.T("plans.onceLabel"), Kind: domain.PlanItemOneTime, Amount: onceMinor, Month: onceM,
			})
		}
		if err := app.PutPlan(p); err != nil {
			errS.Set(err.Error())
			return
		}
		uistate.BumpDataRevision()
		name.Set("")
		account.Set("")
		start.Set("")
		monthly.Set("")
		onceAmt.Set("")
		onceMonth.Set("")
		errS.Set("")
		savedS.Set(uistate.T("plans.addedFlash", nm))
	}))
	cancel := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	acctOpts := []ui.Node{Option(Value(""), SelectedIf(account.Get() == ""), uistate.T("plans.prefillNone"))}
	if app != nil {
		for _, a := range app.Accounts() {
			if a.Archived {
				continue
			}
			acctOpts = append(acctOpts, Option(Value(a.ID), SelectedIf(account.Get() == a.ID), a.Name))
		}
	}

	form := Form(css.Class("plan-add-form"), OnSubmit(save),
		P(css.Class("muted"), uistate.T("plans.hint")),
		Input(append([]any{css.Class("field"), Attr("id", "plan-add"), Type("text"), Attr("aria-required", "true"),
			Placeholder(uistate.T("plans.namePlaceholder")), Value(name.Get()), OnInput(onName)}, errAttrs("plan-err", errS.Get())...)...),
		Div(css.Class("form-grid"),
			labeledField(uistate.T("plans.horizonPlaceholder"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("aria-required", "true"),
					Value(horizon.Get()), Step("1"), OnInput(onHorizon))),
			// Account prefill: selecting an account fills the start-balance input from that
			// account's current balance so the user doesn't have to look it up.
			Label(css.Class("field-label"), uistate.T("plans.prefillAccount"),
				Select(css.Class("field"), Attr("aria-label", uistate.T("plans.prefillAccount")),
					Attr("data-testid", "plan-prefill-account"), OnChange(onAccount), acctOpts)),
			labeledField(uistate.T("plans.startPlaceholder", base),
				Input(css.Class("field"), Type("number"), Value(start.Get()), Step("0.01"), OnInput(onStart))),
			labeledField(uistate.T("plans.monthlyPlaceholder", base),
				Input(css.Class("field"), Type("number"), Value(monthly.Get()), Step("0.01"), OnInput(onMonthly))),
			labeledField(uistate.T("plans.onceAmtPlaceholder", base),
				Input(css.Class("field"), Type("number"), Value(onceAmt.Get()), Step("0.01"), OnInput(onOnceAmt))),
			labeledField(uistate.T("plans.onceMonthPlaceholder"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", horizon.Get()),
					Value(onceMonth.Get()), Step("1"), OnInput(onOnceMonth))),
		),
		errText("plan-err", errS.Get()),
		If(errS.Get() == "" && savedS.Get() != "", P(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("role", "status"), savedS.Get())),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("plans.add")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "plan-add-cancel"), OnClick(cancel), uistate.T("action.cancel")),
		),
	)
	return Div(css.Class("plan-add-modal"), Attr("data-testid", "plan-add-form"), form)
}
