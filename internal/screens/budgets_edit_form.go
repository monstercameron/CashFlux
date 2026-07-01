// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// BudgetEditFormProps drives the budget editor form rendered inside the shell-root
// flip modal (see internal/app BudgetEditHost). Mode selects which editor to show.
type BudgetEditFormProps struct {
	BudgetID string
	Mode     string // one of uistate.BudgetEditMode*
	OnDone   func() // clears the atom (closes the modal); called after a save/cancel
}

// BudgetEditForm renders the budget editor (full edit, or top-up) as the body of the
// flip modal. It owns all its form state and its own Save/Cancel buttons; the host's
// FlipPanel is NoFooter. Because the host only renders this when the atom is set, the
// component mounts fresh on each open, so the useState initializers seed correctly
// from the budget. It lives at the shell root, outside the transformed bento/tile
// ancestors, so the modal centers on the viewport.
func BudgetEditForm(props BudgetEditFormProps) ui.Node {
	// Re-render on data mutations so a stale figure can't linger while open.
	_ = uistate.UseDataRevision().Get()

	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	// Resolve the budget (before the hooks so useState seeds from it).
	var b domain.Budget
	found := false
	if app != nil {
		for _, bb := range app.Budgets() {
			if bb.ID == props.BudgetID {
				b, found = bb, true
				break
			}
		}
	}
	cur := b.Limit.Currency
	if cur == "" {
		if app != nil {
			cur = app.Settings().BaseCurrency
		}
		if cur == "" {
			cur = "USD"
		}
	}
	dec := currency.Decimals(cur)
	limitMajor := ""
	if found {
		limitMajor = money.FormatMinor(b.Limit.Amount, dec)
	}

	// All hooks unconditionally at stable positions (before any branch/return).
	nameS := ui.UseState(b.Name)
	limitS := ui.UseState(limitMajor)
	periodS := ui.UseState(string(b.Period))
	ownerS := ui.UseState(b.OwnerID)
	rolloverS := ui.UseState(b.Rollover)
	methodologyS := ui.UseState(b.Methodology)
	customEditVals := ui.UseState(customMapToStrings(b.Custom))
	topupAmt := ui.UseState("")
	errS := ui.UseState("")

	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limitS.Set(v) })
	onRollover := ui.UseEvent(func() { rolloverS.Set(!rolloverS.Get()) })
	onTopupAmt := ui.UseEvent(func(v string) { topupAmt.Set(v) })
	cancel := ui.UseEvent(Prevent(func() { done() }))
	onCustom := func(key, value string) {
		m := customEditVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, val := range m {
			nm[k] = val
		}
		nm[key] = value
		customEditVals.Set(nm)
	}

	saveEdit := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		for _, bb := range app.Budgets() {
			if bb.ID != props.BudgetID {
				continue
			}
			if n := strings.TrimSpace(nameS.Get()); n != "" {
				bb.Name = n
			}
			amt, err := money.ParseMinor(strings.TrimSpace(limitS.Get()), dec)
			if err != nil || amt <= 0 {
				errS.Set(uistate.T("budgets.limitRequired"))
				return
			}
			bb.Limit = money.New(amt, cur)
			if p := domain.Period(periodS.Get()); p.Valid() {
				bb.Period = p
			}
			bb.OwnerID = ownerS.Get()
			if ownerS.Get() == domain.GroupOwnerID {
				bb.Scope = domain.ScopeShared
			} else {
				bb.Scope = domain.ScopeIndividual
			}
			bb.Rollover = rolloverS.Get()
			if m := budgeting.Methodology(methodologyS.Get()); m.Valid() {
				bb.Methodology = methodologyS.Get()
			} else {
				bb.Methodology = ""
			}
			if defs := app.CustomFieldDefsFor("budget"); len(defs) > 0 {
				bb.Custom = customValuesToMap(defs, customEditVals.Get())
			}
			if err := app.PutBudget(bb); err != nil {
				errS.Set(err.Error())
				return
			}
			break
		}
		uistate.BumpDataRevision()
		done()
	}))

	submitTopup := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(topupAmt.Get()), dec)
		if err != nil || amt <= 0 {
			errS.Set(uistate.T("budgets.limitRequired"))
			return
		}
		for _, bb := range app.Budgets() {
			if bb.ID != props.BudgetID {
				continue
			}
			bb.Limit = money.New(bb.Limit.Amount+amt, cur)
			if err := app.PutBudget(bb); err != nil {
				errS.Set(err.Error())
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("budgets.toppedUpToast", fmtMoney(money.New(amt, cur))), false)
			done()
			return
		}
	}))

	if app == nil || !found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	var errLine ui.Node = Fragment()
	if errS.Get() != "" {
		errLine = P(css.Class("err"), Attr("role", "alert"), errS.Get())
	}

	// --- Top-up: a single amount that raises the budget's limit. ---
	if props.Mode == uistate.BudgetEditModeTopup {
		return Form(css.Class("acct-edit-form"), OnSubmit(submitTopup),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("budgets.topupHint", budgetTitle(b.Name, budgetCategoryName(app, b.CategoryID)), fmtMoney(b.Limit))),
			labeledField(uistate.T("budgets.amountToAdd"),
				Input(css.Class("field"), Attr("id", "budget-topup-amt"), Attr("autofocus", ""), Type("number"),
					Attr("aria-label", uistate.T("budgets.amountToAdd")), Placeholder(uistate.T("budgets.amountToAdd")),
					Value(topupAmt.Get()), Step("0.01"), Attr("min", "0.01"), OnInput(onTopupAmt))),
			errLine,
			Div(css.Class("acct-edit-actions"),
				Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("budgets.addFunds")),
			),
		)
	}

	// --- Full edit. Single-column form (.acct-edit-form) — a clean vertical stack so
	// the rollover explanation and Method sit full-width beneath their controls, and the
	// action row's margin-top:auto pins Save/Cancel to the modal's bottom. ---
	return Form(css.Class("acct-edit-form"), OnSubmit(saveEdit),
		labeledField(uistate.T("common.name"),
			Input(css.Class("field"), Attr("id", "budget-edit-name"), Attr("autofocus", ""), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
		labeledField(uistate.T("budgets.limitLabel"),
			Input(css.Class("field"), Type("number"), Placeholder(uistate.T("budgets.limitLabel")), Value(limitS.Get()), Step("0.01"), OnInput(onLimit))),
		labeledField(uistate.T("budgets.period"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: periodOptions(periodS.Get()), Selected: periodS.Get(),
				OnChange: func(v string) { periodS.Set(v) }, AriaLabel: uistate.T("budgets.period"),
			})),
		labeledField(uistate.T("common.owner"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: ownerSelectOptions(app.Members(), ownerS.Get()), Selected: ownerS.Get(),
				OnChange: func(v string) { ownerS.Set(v) }, AriaLabel: uistate.T("common.owner"),
			})),
		Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("style", "flex-wrap:nowrap"),
			Input(append([]any{Type("checkbox"), Attr("style", "flex-shrink:0"), OnChange(onRollover)}, checkedAttr(rolloverS.Get())...)...),
			Span(uistate.T("budgets.rollover")),
		),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.rolloverHint")),
		labeledField(uistate.T("budgets.methodLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: budgetMethodOptions(methodologyS.Get()), Selected: methodologyS.Get(),
				OnChange: func(v string) { methodologyS.Set(v) }, AriaLabel: uistate.T("budgets.methodLabel"),
			})),
		// Custom fields: one input per user-defined "budget" field (renders nothing
		// when there are no defs).
		MapKeyed(app.CustomFieldDefsFor("budget"), func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
			return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customEditVals.Get()[d.Key], OnChange: onCustom}))
		}),
		errLine,
		Div(css.Class("acct-edit-actions"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		),
	)
}

// budgetCategoryName resolves a category's display name (for the top-up hint), or ""
// when the budget has no category or it can't be found.
func budgetCategoryName(app *appstate.App, categoryID string) string {
	if app == nil || categoryID == "" {
		return ""
	}
	for _, c := range app.Categories() {
		if c.ID == categoryID {
			return c.Name
		}
	}
	return ""
}
