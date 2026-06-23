//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// RuleAddFormProps configures the RuleAddForm component.
type RuleAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
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

	onMatch := ui.UseEvent(func(v string) { match.Set(v) })
	onCategory := ui.UseEvent(func(e ui.Event) { categoryID.Set(e.GetValue()) })
	onTags := ui.UseEvent(func(v string) { tags.Set(v) })

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
		}
		if err := app.PutRule(r); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// Reset fields.
		match.Set("")
		categoryID.Set("")
		tags.Set("")
		errMsg.Set("")
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	return Form(css.Class("form-grid"), Attr("data-testid", "rule-add-form"), OnSubmit(add),
		Input(append([]any{css.Class("field"), Attr("id", "rule-add"), Type("text"), Attr("aria-label", uistate.T("rules.matchFieldLabel")), Attr("aria-required", "true"), Placeholder(uistate.T("rules.matchPlaceholder")), Value(match.Get()), OnInput(onMatch)}, errAttrs("rule-err", errMsg.Get())...)...),
		Select(css.Class("field"), Attr("aria-label", uistate.T("rules.categoryFieldLabel")), OnChange(onCategory), categoryOptions(cats, categoryID.Get())),
		Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("rules.tagsFieldLabel")), Placeholder(uistate.T("rules.tagsPlaceholder")), Value(tags.Get()), OnInput(onTags)),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		If(liveMatch != "" && len(texts) > 0, P(css.Class("muted"), Attr("role", "status"), uistate.T("rules.matchCountMeta", plural(liveCount, "transaction")))),
		errText("rule-err", errMsg.Get()),
	)
}
