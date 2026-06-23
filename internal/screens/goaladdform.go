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
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTarget := ui.UseEvent(func(v string) { target.Set(v) })
	onCurrent := ui.UseEvent(func(v string) { current.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	onLinkAcct := ui.UseEvent(func(e ui.Event) { linkAcct.Set(e.GetValue()) })

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
		customVals.Set(map[string]string{})
		errMsg.Set("")
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	ownerOptions := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(owner.Get() == domain.GroupOwnerID), uistate.T("owner.group"))}
	for _, m := range app.Members() {
		ownerOptions = append(ownerOptions, Option(Value(m.ID), SelectedIf(owner.Get() == m.ID), m.Name))
	}
	linkOptions := goalAccountOptions(accounts, linkAcct.Get())

	return Form(css.Class("form-grid"), Attr("data-testid", "goal-add-form"), OnSubmit(add),
		labeledField(uistate.T("common.name"),
			Input(append([]any{css.Class("field"), Attr("id", "goal-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("goal-err", errMsg.Get())...)...)),
		labeledField(uistate.T("goals.targetLabel"),
			Input(css.Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("goals.targetPlaceholder", base)), Value(target.Get()), Step("0.01"), OnInput(onTarget))),
		labeledField(uistate.T("goals.savedSoFar"),
			Input(css.Class("field"), Type("number"), Placeholder(uistate.T("goals.savedSoFar")), Value(current.Get()), Step("0.01"), OnInput(onCurrent))),
		labeledField(uistate.T("goals.owner"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("goals.owner")), OnChange(onOwner), ownerOptions)),
		labeledField(uistate.T("goals.linkedOptional"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("goals.linkedOptional")), Title(uistate.T("goals.linkedOptional")), OnChange(onLinkAcct), linkOptions)),
		labeledField(uistate.T("goals.dateLabel"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("goals.dateLabel")), Value(dateStr.Get()), OnInput(onDate))),
		MapKeyed(goalDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
		}),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		errText("goal-err", errMsg.Get()),
	)
}
