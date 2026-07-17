// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/memberrole"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	roleS := ui.UseState(string(memberrole.DefaultRole(false)))
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
		role, err := memberrole.ParseRole(roleS.Get())
		if err != nil {
			role = memberrole.DefaultRole(false)
		}
		m := domain.Member{
			ID:     id.New(),
			Name:   n,
			Color:  strings.TrimSpace(color.Get()),
			Role:   role,
			Custom: customValuesToMap(memberDefs, customVals.Get()),
		}
		if err := app.PutMember(m); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// QA M1: without the revision bump the Household roster kept rendering the
		// pre-save member list (the modal closed, the count stayed stale, and only
		// a route change revealed the saved member). The notice is the success
		// feedback the silent close lacked.
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("members.added", n), false)
		// Reset fields.
		name.Set("")
		color.Set("#7c83ff")
		roleS.Set(string(memberrole.DefaultRole(false)))
		customVals.Set(map[string]string{})
		errMsg.Set("")
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	return Fragment(
		Form(css.Class("form-grid"), Attr("id", "member-add-form"), Attr("data-testid", "member-add-form"), OnSubmit(add),
			labeledField(uistate.T("members.name"),
				Input(append([]any{css.Class("field"), Attr("id", "member-add"), Type("text"), Attr("aria-label", uistate.T("members.name")), Attr("aria-required", "true"), Placeholder(uistate.T("members.name")), Value(name.Get()), OnInput(onName)}, errAttrs("member-err", errMsg.Get())...)...)),
			labeledField(uistate.T("members.color"),
				Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("members.color")), Attr("aria-label", uistate.T("members.color")), Value(color.Get()), OnInput(onColor))),
			labeledField(uistate.T("members.roleLabel"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   memberRoleOptions(),
					Selected:  roleS.Get(),
					OnChange:  func(v string) { roleS.Set(v) },
					AriaLabel: "Role",
					TestID:    "member-add-role",
				})),
			MapKeyed(memberDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
				return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
			}),
		),
		errText("member-err", errMsg.Get()),
	)
}
