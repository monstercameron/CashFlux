// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// cfEntities are the entity types that can carry custom fields, in display order.
// Key is an i18n key resolved at render (reuses the nav labels).
var cfEntities = []struct{ Value, Key string }{
	{"account", "nav.accounts"},
	{"transaction", "nav.transactions"},
	{"budget", "nav.budgets"},
	{"cover", "budgets.coverEntityLabel"},
	{"goal", "nav.goals"},
	{"member", "nav.members"},
}

// cfTypes are the supported custom-field data types, in display order. Key is an
// i18n key resolved at render.
var cfTypes = []struct {
	Value customfields.FieldType
	Key   string
}{
	{customfields.TypeText, "cf.typeText"},
	{customfields.TypeNumber, "cf.typeNumber"},
	{customfields.TypeDate, "cf.typeDate"},
	{customfields.TypeBool, "cf.typeBool"},
	{customfields.TypeSelect, "cf.typeSelect"},
}

// CustomFieldsManager lets you define extra fields on your entities: pick the
// entity type, a key and label, a data type, and (for choice fields) the allowed
// options. Definitions are validated and persisted; existing ones list below with
// per-row delete. This is the management UI for the internal/customfields engine.
func CustomFieldsManager() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	rev := state.UseAtom("rev:customfields", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	entity := ui.UseState("account")
	key := ui.UseState("")
	label := ui.UseState("")
	ftype := ui.UseState(string(customfields.TypeText))
	options := ui.UseState("")
	required := ui.UseState("no")
	errMsg := ui.UseState("")

	onEntity := ui.UseEvent(func(e ui.Event) { entity.Set(e.GetValue()) })
	onKey := ui.UseEvent(func(v string) { key.Set(v) })
	onLabel := ui.UseEvent(func(v string) { label.Set(v) })
	onType := ui.UseEvent(func(e ui.Event) { ftype.Set(e.GetValue()) })
	onOptions := ui.UseEvent(func(v string) { options.Set(v) })
	onRequired := ui.UseEvent(func(e ui.Event) { required.Set(e.GetValue()) })

	add := ui.UseEvent(Prevent(func() {
		def := customfields.Def{
			ID:         id.New(),
			EntityType: entity.Get(),
			Key:        strings.TrimSpace(key.Get()),
			Label:      strings.TrimSpace(label.Get()),
			Type:       customfields.FieldType(ftype.Get()),
			Options:    textutil.CommaFields(options.Get()),
			Required:   required.Get() == "yes",
		}
		if err := app.PutCustomFieldDef(def); err != nil {
			errMsg.Set(err.Error())
			return
		}
		key.Set("")
		label.Set("")
		options.Set("")
		errMsg.Set("")
		bump()
	}))

	deleteDef := func(defID string) {
		if err := app.DeleteCustomFieldDef(defID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	entityOptions := make([]ui.Node, 0, len(cfEntities))
	for _, e := range cfEntities {
		entityOptions = append(entityOptions, Option(Value(e.Value), SelectedIf(entity.Get() == e.Value), uistate.T(e.Key)))
	}
	typeOptions := make([]ui.Node, 0, len(cfTypes))
	for _, ty := range cfTypes {
		typeOptions = append(typeOptions, Option(Value(string(ty.Value)), SelectedIf(ftype.Get() == string(ty.Value)), uistate.T(ty.Key)))
	}

	isChoice := ftype.Get() == string(customfields.TypeSelect)

	form := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("cf.addTitle"),
		Body: Fragment(
			P(css.Class("muted"), uistate.T("cf.addDesc")),
			Form(css.Class("form-grid"), OnSubmit(add),
				Select(css.Class("field"), OnChange(onEntity), entityOptions),
				Label(css.Class("labeled-field"),
					Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
					Span(css.Class("muted"), uistate.T("cf.keyLabel")),
					Input(append([]any{css.Class("field"), Type("text"), Placeholder(uistate.T("cf.keyPlaceholder")), Title(uistate.T("cf.keyTitle")), Attr("pattern", "[A-Za-z0-9_]+"), Value(key.Get()), OnInput(onKey)}, errAttrs("cf-err", errMsg.Get())...)...),
				),
				Label(css.Class("labeled-field"),
					Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
					Span(css.Class("muted"), uistate.T("cf.labelLabel")),
					Input(css.Class("field"), Type("text"), Placeholder(uistate.T("cf.labelPlaceholder")), Value(label.Get()), OnInput(onLabel)),
				),
				Select(css.Class("field"), OnChange(onType), typeOptions),
				If(isChoice, Input(css.Class("field field-wide"), Type("text"), Placeholder(uistate.T("cf.optionsPlaceholder")), Value(options.Get()), OnInput(onOptions))),
				Select(css.Class("field"), OnChange(onRequired),
					Option(Value("no"), SelectedIf(required.Get() == "no"), uistate.T("cf.optional")),
					Option(Value("yes"), SelectedIf(required.Get() == "yes"), uistate.T("cf.required")),
				),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("cf.addField")),
			),
			errText("cf-err", errMsg.Get()),
		),
	})

	// Group existing defs by entity type. Every entity type always renders so
	// users can see what categories exist even before adding a field (G15 §7).
	defs := app.CustomFieldDefs()
	sections := make([]any, 0, len(cfEntities))
	for _, e := range cfEntities {
		var rows []customfields.Def
		for _, d := range defs {
			if d.EntityType == e.Value {
				rows = append(rows, d)
			}
		}
		renderRow := func(d customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldRow, customFieldRowProps{Def: d, OnDelete: deleteDef})
		}
		keyOf := func(d customfields.Def) any { return d.ID }
		props := uiw.EntityListSectionProps{Title: uistate.T(e.Key)}
		if len(rows) == 0 {
			props.Body = P(css.Class("empty"), uistate.T("cf.entityEmpty", uistate.T(e.Key)))
		} else {
			props.Rows = MapKeyed(rows, keyOf, renderRow)
		}
		sections = append(sections, uiw.EntityListSection(props))
	}

	list := Fragment(sections...)

	return Div(form, list)
}

// parseOptions splits a comma-separated option list, trimming blanks.
// cfTypeLabel renders a field type as the human label used in the form.
func cfTypeLabel(t customfields.FieldType) string {
	for _, ty := range cfTypes {
		if ty.Value == t {
			return uistate.T(ty.Key)
		}
	}
	return string(t)
}

type customFieldRowProps struct {
	Def      customfields.Def
	OnDelete func(string)
}

// CustomFieldRow is a per-definition row with a stable delete-handler hook.
func CustomFieldRow(props customFieldRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Def.ID) }))
	meta := cfTypeLabel(props.Def.Type)
	if props.Def.Required {
		meta += uistate.T("cf.requiredSuffix")
	}
	if props.Def.Type == customfields.TypeSelect && len(props.Def.Options) > 0 {
		meta += " · " + strings.Join(props.Def.Options, ", ")
	}
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), props.Def.Label),
			Span(css.Class("row-meta"), props.Def.Key+" — "+meta),
		),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("cf.deleteTitle")), Title(uistate.T("cf.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}
