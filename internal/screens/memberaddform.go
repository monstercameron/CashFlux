//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// MemberAddFormProps configures the MemberAddForm component.
type MemberAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
}

// MemberAddForm is the standalone add-a-member form. It owns all its state and
// handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Members() for use in the AddHost modal.
func MemberAddForm(props MemberAddFormProps) ui.Node {
	return ui.CreateElement(memberAddForm, props)
}

func memberAddForm(props MemberAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	name := ui.UseState("")
	color := ui.UseState("#7c83ff")
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onColor := ui.UseEvent(func(v string) { color.Set(v) })

	memberDefs := app.CustomFieldDefsFor("member")
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
		n := strings.TrimSpace(name.Get())
		if n == "" {
			errMsg.Set(uistate.T("members.nameRequired"))
			return
		}
		m := domain.Member{
			ID: id.New(), Name: n, Color: strings.TrimSpace(color.Get()),
			Custom: customValuesToMap(memberDefs, customVals.Get()),
		}
		if err := app.PutMember(m); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// Reset fields.
		name.Set("")
		color.Set("#7c83ff")
		customVals.Set(map[string]string{})
		errMsg.Set("")
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	return Fragment(
		Form(css.Class("form-grid"), Attr("data-testid", "member-add-form"), OnSubmit(add),
			Input(append([]any{css.Class("field"), Attr("id", "member-add"), Type("text"), Attr("aria-label", uistate.T("members.name")), Attr("aria-required", "true"), Placeholder(uistate.T("members.name")), Value(name.Get()), OnInput(onName)}, errAttrs("member-err", errMsg.Get())...)...),
			Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("members.color")), Attr("aria-label", uistate.T("members.color")), Value(color.Get()), OnInput(onColor)),
			MapKeyed(memberDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
				return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
			}),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("members.add")),
		),
		errText("member-err", errMsg.Get()),
	)
}
