// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smarttext"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// GoalAddFormProps configures the GoalAddForm component.
type GoalAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
}

// GoalAddForm is the standalone add-a-goal form. It owns all its state and
// handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Goals() for use in the AddHost modal.
func GoalAddForm(props GoalAddFormProps) ui.Node {
	return ui.CreateElement(goalAddForm, props)
}

func goalAddForm(props GoalAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	accounts := app.Accounts()

	name := ui.UseState("")
	target := ui.UseState("")
	current := ui.UseState("0")
	owner := ui.UseState(domain.GroupOwnerID)
	dateStr := ui.UseState("")
	linkAcct := ui.UseState("")
	advOpen := ui.UseState(false)
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")
	// C189: sinking-fund flag; C192: optional linked spending category.
	isSinkingFund := ui.UseState(false)
	categoryID := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTarget := ui.UseEvent(func(v string) { target.Set(v) })
	onCurrent := ui.UseEvent(func(v string) { current.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	// onOwner/onLinkAcct hooks kept for stable hook ordering; SelectInput owns the
	// change event internally so these handlers are no longer wired to DOM.
	ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { linkAcct.Set(e.GetValue()) })
	// onToggleAdv kept for stable hook ordering (C176: toggle removed, fields
	// are always visible); the state var advOpen remains registered too.
	ui.UseEvent(func() { advOpen.Set(!advOpen.Get()) })
	// onSinkingFund / onCategoryID hooks kept for stable hook ordering; the
	// checkbox uses OnChange(onSinkingFund) and SelectInput owns onCategoryID.
	onSinkingFund := ui.UseEvent(func(e ui.Event) { isSinkingFund.Set(e.IsChecked()) })
	ui.UseEvent(func(e ui.Event) { categoryID.Set(e.GetValue()) })

	goalDefs := app.CustomFieldDefsFor("goal")
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
		if strings.TrimSpace(name.Get()) == "" {
			errMsg.Set(uistate.T("goals.nameRequired"))
			return
		}
		tgt, err := money.ParseMinor(strings.TrimSpace(target.Get()), currency.Decimals(base))
		if err != nil || tgt <= 0 {
			errMsg.Set(uistate.T("goals.targetRequired"))
			return
		}
		cur, err := money.ParseMinor(strings.TrimSpace(current.Get()), currency.Decimals(base))
		if err != nil {
			cur = 0
		}
		var targetDate time.Time
		if ds := strings.TrimSpace(dateStr.Get()); ds != "" {
			if targetDate, err = dateutil.ParseDate(ds); err != nil {
				errMsg.Set(uistate.T("goals.invalidDate"))
				return
			}
		}
		scope := domain.ScopeIndividual
		if owner.Get() == domain.GroupOwnerID {
			scope = domain.ScopeShared
		}
		g := domain.Goal{
			ID: id.New(), Name: strings.TrimSpace(name.Get()), Scope: scope, OwnerID: owner.Get(),
			TargetAmount: money.New(tgt, base), CurrentAmount: money.New(cur, base), TargetDate: targetDate,
			AccountID: linkAcct.Get(), Custom: customValuesToMap(goalDefs, customVals.Get()),
			IsSinkingFund: isSinkingFund.Get(), CategoryID: categoryID.Get(),
		}
		if err := app.PutGoal(g); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// Reset fields.
		name.Set("")
		target.Set("")
		current.Set("0")
		dateStr.Set("")
		linkAcct.Set("")
		isSinkingFund.Set(false)
		categoryID.Set("")
		customVals.Set(map[string]string{})
		errMsg.Set("")
		uistate.PostNotice(uistate.T("goals.addedToast"), false)
		// The add modal is a sibling (AddHost), so bump the shared data revision to
		// re-render the Goals list immediately rather than only after a reload
		// (C177/R2). Goals() subscribes to UseDataRevision for this.
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	ownerOptions := ownerSelectOptions(app.Members(), owner.Get())
	linkOptions := goalAccountOptions(accounts, linkAcct.Get())
	catOptions := goalCategoryOptions(app.Categories(), categoryID.Get())

	// Wish→goal assist: if the typed name parses as a free-text savings wish,
	// offer a chip that fills both the name and the target amount in one click.
	goalSmartSettings := uistate.LoadSmartSettings()
	decimals := currency.Decimals(base)
	var wishSuggestion string
	var wishMinor int64
	if wn, wamt, wok := smarttext.ParseWish(name.Get()); wok {
		// Only show when the parsed name differs from what was typed (real suggestion).
		if wn != strings.TrimSpace(name.Get()) || target.Get() == "" || target.Get() == "0" {
			wishSuggestion = wn
			wishMinor = wamt
		}
	}
	wishAssist := smartFieldAssist(goalSmartSettings, "goal-wish", wishSuggestion, func() {
		name.Set(wishSuggestion)
		if wishMinor > 0 {
			target.Set(money.FormatMinor(wishMinor, decimals))
		}
	})

	return Form(css.Class("form-grid"), Attr("data-testid", "goal-add-form"), OnSubmit(add),
		labeledField(uistate.T("common.name"),
			Input(append([]any{css.Class("field"), Attr("id", "goal-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("goal-err", errMsg.Get())...)...)),
		wishAssist,
		labeledField(uistate.T("goals.targetLabel"),
			Input(css.Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("goals.targetPlaceholder", base)), Value(target.Get()), Step("0.01"), OnInput(onTarget))),
		labeledField(uistate.T("goals.dateLabel"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("goals.dateLabel")), Value(dateStr.Get()), OnInput(onDate))),
		// C176: Saved-so-far, Owner, and Linked account are always visible —
		// they are core goal attributes, not advanced options.
		labeledField(uistate.T("goals.savedSoFar"),
			Input(css.Class("field"), Type("number"), Placeholder(uistate.T("goals.savedSoFar")), Value(current.Get()), Step("0.01"), OnInput(onCurrent))),
		labeledField(uistate.T("goals.owner"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   ownerOptions,
				Selected:  owner.Get(),
				OnChange:  func(v string) { owner.Set(v) },
				AriaLabel: uistate.T("goals.owner"),
			})),
		labeledField(uistate.T("goals.linkedOptional"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   linkOptions,
				Selected:  linkAcct.Get(),
				OnChange:  func(v string) { linkAcct.Set(v) },
				AriaLabel: uistate.T("goals.linkedOptional"),
			})),
		// C189: sinking-fund toggle — marks this goal as a regular-save-for-irregular-expense fund.
		labeledField(uistate.T("goals.sinkingFund"),
			func() ui.Node {
				cbArgs := []any{Type("checkbox"), Attr("id", "goal-add-sinking"), OnChange(onSinkingFund)}
				if isSinkingFund.Get() {
					cbArgs = append(cbArgs, Attr("checked", ""))
				}
				return Div(
					Input(cbArgs...),
					Span(css.Class("budget-sub"), uistate.T("goals.sinkingFundHint")),
				)
			}()),
		// C192: optional linked spending category for the fund (always shown, meaningful mainly for sinking funds).
		labeledField(uistate.T("goals.linkedCategory"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   catOptions,
				Selected:  categoryID.Get(),
				OnChange:  func(v string) { categoryID.Set(v) },
				AriaLabel: uistate.T("goals.linkedCategory"),
			})),
		MapKeyed(goalDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
		}),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("goals.add")),
		errText("goal-err", errMsg.Get()),
	)
}
