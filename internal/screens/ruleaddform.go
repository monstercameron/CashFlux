// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// categorySelectOptions builds a []uiw.SelectOption for a category picker
// (a leading "choose" placeholder, then every category by name).
func categorySelectOptions(cats []domain.Category, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("rules.chooseCategory")}}
	for _, c := range cats {
		opts = append(opts, uiw.SelectOption{Value: c.ID, Label: c.Name})
	}
	return opts
}

// RuleAddFormProps configures the RuleAddForm component.
type RuleAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
}

// condSlotOpts returns the field dropdown options for a structured condition slot.
func condSlotFieldOpts() []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "", Label: "— Field —"},
		{Value: string(rules.ConditionFieldPayee), Label: uistate.T("rulecond.field.payee")},
		{Value: string(rules.ConditionFieldDescription), Label: uistate.T("rulecond.field.description")},
		{Value: string(rules.ConditionFieldAmount), Label: uistate.T("rulecond.field.amount")},
		{Value: string(rules.ConditionFieldAccount), Label: uistate.T("rulecond.field.account")},
		{Value: string(rules.ConditionFieldDate), Label: uistate.T("rulecond.field.date")},
	}
}

// condSlotOpOpts returns operator options appropriate for the chosen field.
func condSlotOpOpts(field string) []uiw.SelectOption {
	switch rules.ConditionField(field) {
	case rules.ConditionFieldPayee, rules.ConditionFieldDescription:
		return []uiw.SelectOption{
			{Value: string(rules.ConditionOpContains), Label: uistate.T("rulecond.op.contains")},
			{Value: string(rules.ConditionOpEquals), Label: uistate.T("rulecond.op.equals")},
		}
	case rules.ConditionFieldAmount:
		return []uiw.SelectOption{
			{Value: string(rules.ConditionOpGt), Label: uistate.T("rulecond.op.gt")},
			{Value: string(rules.ConditionOpGte), Label: uistate.T("rulecond.op.gte")},
			{Value: string(rules.ConditionOpLt), Label: uistate.T("rulecond.op.lt")},
			{Value: string(rules.ConditionOpLte), Label: uistate.T("rulecond.op.lte")},
			{Value: string(rules.ConditionOpEq), Label: uistate.T("rulecond.op.eq")},
			{Value: string(rules.ConditionOpNeq), Label: uistate.T("rulecond.op.neq")},
		}
	case rules.ConditionFieldAccount:
		return []uiw.SelectOption{
			{Value: string(rules.ConditionOpIs), Label: uistate.T("rulecond.op.is")},
			{Value: string(rules.ConditionOpIsNot), Label: uistate.T("rulecond.op.is-not")},
		}
	case rules.ConditionFieldDate:
		return []uiw.SelectOption{
			{Value: string(rules.ConditionOpInMonth), Label: uistate.T("rulecond.op.in-month")},
			{Value: string(rules.ConditionOpOn), Label: uistate.T("rulecond.op.on")},
			{Value: string(rules.ConditionOpBefore), Label: uistate.T("rulecond.op.before")},
			{Value: string(rules.ConditionOpAfter), Label: uistate.T("rulecond.op.after")},
		}
	default:
		return []uiw.SelectOption{{Value: "", Label: "—"}}
	}
}

// condValueHint returns a short input placeholder for the chosen field.
func condValueHint(field string) string {
	switch rules.ConditionField(field) {
	case rules.ConditionFieldAmount:
		return uistate.T("rulecond.amountHint")
	case rules.ConditionFieldDate:
		return uistate.T("rulecond.dateHint")
	case rules.ConditionFieldAccount:
		return "Account ID"
	default:
		return "Value"
	}
}

// RuleAddForm is the standalone add-a-rule form. It owns all its state and
// handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Rules() for use in the AddHost modal.
func RuleAddForm(props RuleAddFormProps) ui.Node {
	return ui.CreateElement(ruleAddForm, props)
}

func ruleAddForm(props RuleAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	match := ui.UseState("")
	categoryID := ui.UseState("")
	tags := ui.UseState("")
	errMsg := ui.UseState("")

	// C105: 3 bounded fixed condition slots — each gets stable hook positions.
	// Slot 1.
	cond1Enabled := ui.UseState(false)
	cond1Field := ui.UseState("")
	cond1Op := ui.UseState("")
	cond1Value := ui.UseState("")
	// Slot 2.
	cond2Enabled := ui.UseState(false)
	cond2Field := ui.UseState("")
	cond2Op := ui.UseState("")
	cond2Value := ui.UseState("")
	// Slot 3.
	cond3Enabled := ui.UseState(false)
	cond3Field := ui.UseState("")
	cond3Op := ui.UseState("")
	cond3Value := ui.UseState("")

	// Consume any pending "Always categorize like this" prefill once on mount:
	// seed match/category from the draft set by the transaction row, then clear
	// it so a later blank visit starts empty. The atom is captured by the dialog
	// host (dialoghost.go); reading it here is a stable hook position.
	draft := uistate.UseRuleDraft()
	ui.UseEffect(func() func() {
		if d := draft.Get(); d != nil {
			match.Set(d.Match)
			categoryID.Set(d.CategoryID)
			uistate.ClearRuleDraft()
		}
		return nil
	}, "rule-draft-consume")

	onMatch := ui.UseEvent(func(v string) { match.Set(v) })
	// onCategory hook slot kept for stable hook ordering; SelectInput owns the event.
	ui.UseEvent(func(e ui.Event) { categoryID.Set(e.GetValue()) })
	onTags := ui.UseEvent(func(v string) { tags.Set(v) })

	// Slot 1 handlers — stable hook positions, not in a loop.
	onCond1Enable := ui.UseEvent(func(e ui.Event) { cond1Enabled.Set(e.IsChecked()) })
	onCond1Value := ui.UseEvent(func(v string) { cond1Value.Set(v) })
	// Slot 2 handlers.
	onCond2Enable := ui.UseEvent(func(e ui.Event) { cond2Enabled.Set(e.IsChecked()) })
	onCond2Value := ui.UseEvent(func(v string) { cond2Value.Set(v) })
	// Slot 3 handlers.
	onCond3Enable := ui.UseEvent(func(e ui.Event) { cond3Enabled.Set(e.IsChecked()) })
	onCond3Value := ui.UseEvent(func(v string) { cond3Value.Set(v) })

	cats := app.Categories()

	// Text each rule is matched against (payee + description), mirroring the engine
	// at entry/import. Used for the live authoring preview.
	txns := app.Transactions()
	texts := make([]string, len(txns))
	for i, t := range txns {
		texts[i] = t.Payee + " " + t.Desc
	}
	// Live match-count preview while authoring.
	liveMatch := strings.TrimSpace(match.Get())
	liveCount := 0
	if liveMatch != "" {
		liveCount = rules.Rule{Match: liveMatch}.MatchCount(texts)
	}

	// collectConditions gathers the enabled condition slots into a []RuleCondition.
	collectConditions := func() []rules.RuleCondition {
		var conds []rules.RuleCondition
		slots := [3]struct {
			enabled bool
			field   string
			op      string
			value   string
		}{
			{cond1Enabled.Get(), cond1Field.Get(), cond1Op.Get(), cond1Value.Get()},
			{cond2Enabled.Get(), cond2Field.Get(), cond2Op.Get(), cond2Value.Get()},
			{cond3Enabled.Get(), cond3Field.Get(), cond3Op.Get(), cond3Value.Get()},
		}
		for _, s := range slots {
			if !s.enabled || s.field == "" || s.op == "" {
				continue
			}
			conds = append(conds, rules.RuleCondition{
				Field: rules.ConditionField(s.field),
				Op:    rules.ConditionOp(s.op),
				Value: strings.TrimSpace(s.value),
			})
		}
		return conds
	}

	add := ui.UseEvent(Prevent(func() {
		if errKey := validateRuleInput(match.Get(), categoryID.Get()); errKey != "" {
			errMsg.Set(uistate.T(errKey))
			return
		}
		r := rules.Rule{
			ID:            id.New(),
			Match:         strings.TrimSpace(match.Get()),
			SetCategoryID: categoryID.Get(),
			SetTags:       textutil.CommaFields(tags.Get()),
			Conditions:    collectConditions(),
		}
		if err := app.PutRule(r); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// Reset all fields.
		match.Set("")
		categoryID.Set("")
		tags.Set("")
		cond1Enabled.Set(false); cond1Field.Set(""); cond1Op.Set(""); cond1Value.Set("")
		cond2Enabled.Set(false); cond2Field.Set(""); cond2Op.Set(""); cond2Value.Set("")
		cond3Enabled.Set(false); cond3Field.Set(""); cond3Op.Set(""); cond3Value.Set("")
		errMsg.Set("")
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	catOpts := categorySelectOptions(cats, categoryID.Get())
	fieldOpts := condSlotFieldOpts()

	// renderCondSlot renders one bounded condition row. All event handlers are
	// passed in as ui.Handler values — they must have been registered with
	// UseEvent at stable hook positions above, not inside this helper.
	// The onField and onOp closures are plain funcs because SelectInput.OnChange
	// accepts func(string) directly; OnChange/OnInput accept any (including
	// ui.Handler), so we pass the Handler from UseEvent straight through.
	renderCondSlot := func(
		label string,
		enabled bool, onEnable ui.Handler,
		field string, onField func(string),
		op string, onOp func(string),
		value string, onValue ui.Handler,
	) ui.Node {
		opOpts := condSlotOpOpts(field)
		hint := condValueHint(field)
		return Div(css.Class("cond-slot"),
			Label(css.Class("cond-slot-header"),
				Input(Type("checkbox"), Checked(enabled), OnChange(onEnable)),
				Span(label),
			),
			If(enabled,
				Div(css.Class("cond-slot-body"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   fieldOpts,
						Selected:  field,
						OnChange:  onField,
						AriaLabel: uistate.T("rulecond.fieldLabel"),
					}),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   opOpts,
						Selected:  op,
						OnChange:  onOp,
						AriaLabel: uistate.T("rulecond.opLabel"),
					}),
					Input(css.Class("field"), Type("text"),
						Placeholder(hint),
						Attr("aria-label", uistate.T("rulecond.valueLabel")),
						Value(value),
						OnInput(onValue),
					),
				),
			),
		)
	}

	return Form(css.Class("form-grid"), Attr("data-testid", "rule-add-form"), OnSubmit(add),
		// No static id (C107): RuleAddForm renders both inline on /rules and inside the
		// AddHost modal, so a hardcoded id="rule-add" produced a duplicate id when the
		// modal opened over the screen. Nothing references the id (the aria-label is the
		// accessible name, data-testid is the test hook), so it's dropped.
		// C109: Match wrapped in FormField for a visible label (previously aria-label-only).
		// Order is trigger-first (Match → Category → Tags): "when payee contains X, assign Y".
		uiw.FormField(uistate.T("rules.matchFieldLabel"),
			Input(append([]any{css.Class("field"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("rules.matchPlaceholder")), Value(match.Get()), OnInput(onMatch)}, errAttrs("rule-err", errMsg.Get())...)...),
		),
		uiw.FormField(uistate.T("rules.categoryFieldLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   catOpts,
				Selected:  categoryID.Get(),
				OnChange:  func(v string) { categoryID.Set(v) },
				AriaLabel: uistate.T("rules.categoryFieldLabel"),
			})),
		uiw.FormField(uistate.T("rules.tagsFieldLabel"),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("rules.tagsPlaceholder")), Value(tags.Get()), OnInput(onTags)),
		),

		// C105: Up to 3 bounded condition slots. Each slot's On* handlers are
		// registered at stable hook positions above, not inside a loop.
		Fieldset(css.Class("cond-slots"),
			Legend(uistate.T("rulecond.sectionLabel")),
			renderCondSlot(
				uistate.T("rulecond.slot1"),
				cond1Enabled.Get(), onCond1Enable,
				cond1Field.Get(), func(v string) { cond1Field.Set(v); cond1Op.Set("") },
				cond1Op.Get(), func(v string) { cond1Op.Set(v) },
				cond1Value.Get(), onCond1Value,
			),
			renderCondSlot(
				uistate.T("rulecond.slot2"),
				cond2Enabled.Get(), onCond2Enable,
				cond2Field.Get(), func(v string) { cond2Field.Set(v); cond2Op.Set("") },
				cond2Op.Get(), func(v string) { cond2Op.Set(v) },
				cond2Value.Get(), onCond2Value,
			),
			renderCondSlot(
				uistate.T("rulecond.slot3"),
				cond3Enabled.Get(), onCond3Enable,
				cond3Field.Get(), func(v string) { cond3Field.Set(v); cond3Op.Set("") },
				cond3Op.Get(), func(v string) { cond3Op.Set(v) },
				cond3Value.Get(), onCond3Value,
			),
		),

		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		If(liveMatch != "" && len(texts) > 0, P(css.Class("muted"), Attr("role", "status"), uistate.T("rules.matchCountMeta", plural(liveCount, "transaction")))),
		errText("rule-err", errMsg.Get()),
	)
}
