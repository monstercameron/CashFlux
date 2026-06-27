// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// BudgetAddFormProps configures the BudgetAddForm component.
type BudgetAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
}

// BudgetAddForm is the standalone add-a-budget form. It owns all its state
// and handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Budgets() for use in the AddHost modal.
func BudgetAddForm(props BudgetAddFormProps) ui.Node {
	return ui.CreateElement(budgetAddForm, props)
}

func budgetAddForm(props BudgetAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	categories := app.Categories()
	var expenseCats []domain.Category
	for _, c := range categories {
		if c.Kind == domain.KindExpense {
			expenseCats = append(expenseCats, c)
		}
	}

	// Budget add requires at least one expense category.
	if len(expenseCats) == 0 {
		return P(css.Class("empty"), uistate.T("budgets.needCategory"))
	}

	defaultCat := expenseCats[0].ID

	name := ui.UseState("")
	limit := ui.UseState("")
	catID := ui.UseState(defaultCat)
	owner := ui.UseState(domain.GroupOwnerID)
	period := ui.UseState(string(domain.PeriodMonthly))
	rollover := ui.UseState(false)
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limit.Set(v) })
	// onCat/onOwner/onPeriod hooks kept for stable hook ordering; SelectInput owns
	// the change event internally so these handlers are no longer wired to DOM.
	ui.UseEvent(func(e ui.Event) { catID.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { period.Set(e.GetValue()) })
	onRollover := ui.UseEvent(func() { rollover.Set(!rollover.Get()) })

	budgetDefs := app.CustomFieldDefsFor("budget")
	onCustom := func(key, value string) {
		m := customVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customVals.Set(nm)
	}

	add := ui.UseEvent(Prevent(func() {
		amt, err := money.ParseMinor(strings.TrimSpace(limit.Get()), currency.Decimals(base))
		if err != nil || amt <= 0 {
			errMsg.Set(uistate.T("budgets.limitRequired"))
			return
		}
		// One budget per (category, period, owner) — reject duplicates (L40).
		if budgeting.IsDuplicateBudget(app.Budgets(), catID.Get(), period.Get(), owner.Get(), "") {
			errMsg.Set(uistate.T("budgets.duplicateBudget"))
			return
		}
		scope := domain.ScopeIndividual
		if owner.Get() == domain.GroupOwnerID {
			scope = domain.ScopeShared
		}
		b := domain.Budget{
			ID: id.New(), Name: strings.TrimSpace(name.Get()), Scope: scope, OwnerID: owner.Get(),
			CategoryID: catID.Get(), Period: domain.Period(period.Get()), Limit: money.New(amt, base),
			Rollover: rollover.Get(), Custom: customValuesToMap(budgetDefs, customVals.Get()),
		}
		if err := app.PutBudget(b); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// Reset fields.
		name.Set("")
		limit.Set("")
		rollover.Set(false)
		catID.Set(defaultCat)
		customVals.Set(map[string]string{})
		errMsg.Set("")
		uistate.PostNotice(uistate.T("budgets.addedToast"), false)
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	catOptions := uiw.OptionsFrom(expenseCats,
		func(c domain.Category) string { return c.ID },
		func(c domain.Category) string { return c.Name },
		catID.Get())
	ownerOptions := ownerSelectOptions(app.Members(), owner.Get())

	// Suggest a limit from the selected category's recent monthly spend (D6).
	suggestRates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	suggestion, _ := budgeting.SuggestLimit(catID.Get(), app.Transactions(), time.Now(), 6, suggestRates)

	return Form(css.Class("form-grid"), Attr("data-testid", "budget-add-form"), OnSubmit(add),
		labeledField(uistate.T("common.name"),
			Input(append([]any{css.Class("field"), Attr("id", "budget-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("budget-err", errMsg.Get())...)...)),
		labeledField(uistate.T("budgets.categoryLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   catOptions,
				Selected:  catID.Get(),
				OnChange:  func(v string) { catID.Set(v) },
				AriaLabel: uistate.T("budgets.categoryLabel"),
			})),
		// C30: hide the owner picker until members exist (it only offers "Everyone"
		// otherwise — meaningless in a 0-member household; owner stays shared).
		If(len(app.Members()) > 0, labeledField(uistate.T("common.owner"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   ownerOptions,
				Selected:  owner.Get(),
				OnChange:  func(v string) { owner.Set(v) },
				AriaLabel: uistate.T("common.owner"),
			}))),
		labeledField(uistate.T("budgets.period"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   periodOptions(period.Get()),
				Selected:  period.Get(),
				OnChange:  func(v string) { period.Set(v) },
				AriaLabel: uistate.T("budgets.period"),
			})),
		labeledField(uistate.T("budgets.limitLabel"),
			Input(css.Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("budgets.limitPlaceholder", base)), Value(limit.Get()), Step("0.01"), OnInput(onLimit))),
		// C117: keep the checkbox on the same line as its label at narrow widths
		// (≤1280px) — flex + nowrap, shrink-0 on the box (matches budgets_row.go).
		Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("style", "flex-wrap:nowrap"),
			Input(append([]any{Type("checkbox"), Attr("style", "flex-shrink:0"), OnChange(onRollover)}, checkedAttr(rollover.Get())...)...),
			Span(Title(uistate.T("budgets.rolloverTitle")), uistate.T("budgets.rollover")),
		),
		MapKeyed(budgetDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
		}),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("budgets.add")),
		If(suggestion > 0, Div(css.Class("suggest-row"),
			Span(css.Class("muted"), uistate.T("budgets.suggest", fmtMoney(money.New(suggestion, base)))),
			Button(css.Class("btn"), Type("button"), OnClick(func() { limit.Set(money.FormatMinor(suggestion, currency.Decimals(base))) }), uistate.T("budgets.useSuggest")),
		)),
		errText("budget-err", errMsg.Get()),
	)
}
