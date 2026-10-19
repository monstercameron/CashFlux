// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetEditFieldsProps carries the explicit inputs for budgetEditFields.
type budgetEditFieldsProps struct {
	periodS        ui.State[string]
	b              domain.Budget
	methodologyS   ui.State[string]
	limitS         ui.State[string]
	dec            int
	cur            string
	advEditOpen    ui.State[bool]
	catsOpen       state.Atom[string]
	props          BudgetEditFormProps
	closeCats      ui.Handler
	saveEdit       ui.Handler
	ev             entityVarState
	nameS          ui.State[string]
	onLimit        ui.Handler
	app            *appstate.App
	selfStatus     budgeting.Status
	viewAnchor     time.Time
	targetKindS    ui.State[string]
	targetAmtS     ui.State[string]
	targetDateS    ui.State[string]
	linkedGoalS    ui.State[string]
	onFillPick     func(major string)
	selfFound      bool
	onTargetAmt    ui.Handler
	onTargetDate   ui.Handler
	ownerS         ui.State[string]
	openCats       ui.Handler
	onRollover     ui.Handler
	rolloverS      ui.State[bool]
	rolloverCapS   ui.State[string]
	customEditVals ui.State[map[string]string]
	onCustom       func(key string, value string)
	toggleAdvEdit  ui.Handler
	errLine        ui.Node
	cancel         ui.Handler
}

// budgetEditFields builds the edit fields section.
func budgetEditFields(p budgetEditFieldsProps) ui.Node {
	// C665: the limit's live verdict, shared by the field, its inline message and
	// the Save button, so the three can never disagree about whether the form is
	// committable.
	_, limitProblem := budgeting.ParseLimitMajor(p.limitS.Get(), p.dec)
	limitErrMsg := budgetLimitError(limitProblem, p.cur)
	limitSaveTitle := uistate.T("action.save")
	if limitErrMsg != "" {
		limitSaveTitle = uistate.T("budgets.limitSaveBlocked")
	}

	// C521: the tracked-categories editor is a SECOND PAGE of this panel, not a
	// second panel.
	//
	// It used to close this form and open its own modal, which destroyed this
	// component and every unsaved edit with it — change the name, step into the
	// categories, come back, and the name was gone. It also left no way back: the
	// return trip was main page → edit → categories → main page, skipping the form.
	//
	// Layering two FlipPanels was tried first and does not work: each registers its
	// own document-level Escape handler and captures its close callback in a ref, so
	// one press closes both and no guard on the parent can prevent it. Keeping ONE
	// panel and swapping its body means there is one Escape, one backdrop, an
	// explicit Back, and a component that never unmounts — so nothing can be lost.
	if p.catsOpen.Get() == p.props.BudgetID {
		return Div(css.Class("acct-edit-form", "budget-edit"),
			Div(css.Class("modal-scroll"),
				Div(css.Class("budget-cats-back"),
					Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budget-cats-back"),
						OnClick(p.closeCats), uistate.T("budgets.editCatsBack")),
					Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.editCatsBackHint")),
				),
				ui.CreateElement(BudgetCategoriesBody, struct{}{}),
			),
		)
	}

	return Form(css.Class("acct-edit-form", "budget-edit"), OnSubmit(p.saveEdit),
		Div(css.Class("modal-scroll"),
			ui.CreateElement(budgetEditBasics, budgetEditBasicsProps{budgetEditFieldsProps: p, limitErrMsg: limitErrMsg}),
			// BG4: one-tap fill chips (last month, 3/6-mo average, last period, to target).
			budgetQuickFillRow(p.app, p.b, p.selfStatus, p.viewAnchor, budgetTargetDraft{
				Kind: p.targetKindS.Get(), Amount: p.targetAmtS.Get(), Date: p.targetDateS.Get(),
				GoalID: p.linkedGoalS.Get(), Decimals: p.dec, Currency: p.cur,
			}, p.onFillPick),
			If(p.app != nil && p.selfFound, budgetRecommendationLine(p.app, p.b, p.selfStatus, p.viewAnchor)),
			// BG1: optional funding target (refill-up-to, set-aside, or by-date via a goal).
			budgetTargetSection(p.app, budgetTargetDraft{
				Kind: p.targetKindS.Get(), Amount: p.targetAmtS.Get(), Date: p.targetDateS.Get(),
				GoalID: p.linkedGoalS.Get(), Decimals: p.dec, Currency: p.cur,
			}, func(v string) { p.targetKindS.Set(v) }, func(v string) { p.linkedGoalS.Set(v) }, p.onTargetAmt, p.onTargetDate),
			ui.CreateElement(budgetEditTracking, p),
			ui.CreateElement(budgetEditRollover, p),
			// Custom fields: one input per user-defined "budget" field (renders nothing
			// when there are no defs).
			MapKeyed(p.app.CustomFieldDefsFor("budget"), func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
				return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: p.customEditVals.Get()[d.Key], OnChange: p.onCustom}))
			}),
			ui.CreateElement(budgetEditFormulas, p),
			p.errLine,
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(p.cancel), uistate.T("action.cancel")),
			// C665: Save is dead while the limit is unusable, and its tooltip says
			// why. The handler refuses the same values, so a keyboard submit cannot
			// slip past a disabled button.
			Button(append([]any{css.Class("btn btn-primary"), Type("submit"),
				Attr("data-testid", "budget-edit-save"), Title(limitSaveTitle), uistate.T("action.save")},
				disabledAttr(limitErrMsg != "")...)...),
		),
	)
}

type budgetEditBasicsProps struct {
	budgetEditFieldsProps
	limitErrMsg string
}

// budgetEditBasics renders one stable section of the full budget editor.
func budgetEditBasics(p budgetEditBasicsProps) ui.Node {
	periodChanged := p.periodS.Get() != string(p.b.Period)
	return Fragment(
		labeledField(uistate.T("common.name"),
			Input(css.Class("field"), Attr("id", "budget-edit-name"), Attr("autofocus", ""), Type("text"), Placeholder(uistate.T("common.name")), OnInput(p.ev.OnName), uiw.FieldValue(p.nameS.Get()))),
		// The core budget params pair into two columns so the form reads calmly and fits
		// the panel instead of a long single stack: amount + cadence, then owner + method.
		Div(css.Class("budget-edit-row"),
			// C665: min + live validation. The field accepted "-1" and left Save
			// enabled; a negative limit inverts every figure computed from it
			// (remaining, percent used, over/under), so it is refused while it is
			// being typed rather than argued about after a commit attempt.
			labeledField(uistate.T("budgets.limitLabel"),
				Fragment(
					Input(append(append([]any{css.Class("field"), Type("number"), Attr("data-testid", "budget-edit-limit"), Attr("aria-label", uistate.T("budgets.limitLabel")),
						Placeholder(uistate.T("budgets.limitLabel")), Step("0.01"), Attr("min", "0.01"), Attr("aria-required", "true"), OnInput(p.onLimit)},
						errAttrs("budget-edit-limit-err", p.limitErrMsg)...), uiw.FieldValue(p.limitS.Get()))...),
					errText("budget-edit-limit-err", p.limitErrMsg))),
			labeledField(uistate.T("budgets.period"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: periodOptions(p.periodS.Get()), Selected: p.periodS.Get(),
					OnChange: func(v string) { p.periodS.Set(v) }, AriaLabel: uistate.T("budgets.period"),
				}))),
		If(periodChanged, P(css.Class(tw.TextWarn, tw.Text12), Attr("data-testid", "budget-period-hint"),
			Style(map[string]string{"margin": "0"}), uistate.T("budgets.periodChangeHint"))),
	)
}

// budgetEditTracking renders one stable section of the full budget editor.
func budgetEditTracking(p budgetEditFieldsProps) ui.Node {
	methodChanged := p.methodologyS.Get() != p.b.Methodology
	return Fragment(
		Div(css.Class("budget-edit-row"),
			labeledField(uistate.T("common.owner"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: ownerSelectOptions(p.app.Members(), p.ownerS.Get()), Selected: p.ownerS.Get(),
					OnChange: func(v string) { p.ownerS.Set(v) }, AriaLabel: uistate.T("common.owner"),
				})),
			// C590: this picker moves ONE budget; the page's "Budgeting method"
			// popover moves the household. Both were called "method" and neither
			// said so, which is how a card-level edit became a page-wide switch.
			labeledField(uistate.T("budgets.methodLabel"),
				Fragment(
					uiw.SelectInput(uiw.SelectInputProps{
						Options: budgetMethodOptions(p.methodologyS.Get()), Selected: p.methodologyS.Get(),
						OnChange: func(v string) { p.methodologyS.Set(v) }, AriaLabel: uistate.T("budgets.methodLabel"),
					}),
					Span(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "budget-edit-method-hint"),
						budgetOwnMethodHint(p.app.Settings().BudgetMethodology, p.methodologyS.Get()))))),
		// C5: owner silently set the budget's shared/individual scope — say what it means.
		If(len(p.app.Members()) > 0, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "budget-edit-owner-hint"),
			Style(map[string]string{"margin": "0"}), budgetOwnerScopeHint(p.app.Members(), p.ownerS.Get()))),
		If(methodChanged, P(css.Class(tw.TextWarn, tw.Text12), Attr("data-testid", "budget-method-hint"),
			Style(map[string]string{"margin": "0"}), uistate.T("budgets.methodChangeHint"))),
		// C1: tracked categories are managed in their own searchable editor — one
		// concept, one place. This closes the edit form and opens it.
		Div(css.Class("budget-edit-cats-link"),
			Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.editCatsElsewhere")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budget-edit-open-cats"),
				OnClick(p.openCats), uistate.T("budgets.editCatsOpen"))),
	)
}

// budgetEditRollover renders one stable section of the full budget editor.
func budgetEditRollover(p budgetEditFieldsProps) ui.Node {

	return Fragment(
		Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("style", "flex-wrap:nowrap"),
			Input(append([]any{Type("checkbox"), Attr("style", "flex-shrink:0"), OnChange(p.onRollover)}, checkedAttr(p.rolloverS.Get())...)...),
			Span(uistate.T("budgets.rollover")),
		),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.rolloverHint")),
		// BG5: cap how much unused budget can roll over, so a neglected budget can't
		// build an unbounded fictional cushion. Only shown while rollover is on.
		If(p.rolloverS.Get(),
			labeledField(uistate.T("budgets.rolloverCapLabel"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   rolloverCapOptions(),
					Selected:  p.rolloverCapS.Get(),
					OnChange:  func(v string) { p.rolloverCapS.Set(v) },
					AriaLabel: uistate.T("budgets.rolloverCapLabel"),
				}))),
	)
}

// budgetEditFormulas renders one stable section of the full budget editor.
func budgetEditFormulas(p budgetEditFieldsProps) ui.Node {
	advEditLabel := uistate.T("budgets.advancedShow")
	if p.advEditOpen.Get() {
		advEditLabel = uistate.T("budgets.advancedHide")
	}
	return Fragment(
		// C6: the formula-engine variable name — plumbing, behind a disclosure.
		Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("data-testid", "budget-edit-advanced"),
			Attr("aria-expanded", ariaBool(p.advEditOpen.Get())), OnClick(p.toggleAdvEdit), Text(advEditLabel)),
		If(p.advEditOpen.Get(),
			labeledField(uistate.T("budgets.varNameLabel"),
				entityVarField(budgetVarKind, budgetVarEntities(p.app.Budgets()), p.props.BudgetID, "budget-edit-varname", "budget-varname-warn", p.ev.VarName.Get(), p.nameS.Get(), p.ev.OnVarName))),
	)
}
