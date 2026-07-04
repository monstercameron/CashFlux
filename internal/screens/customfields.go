// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/engineenv"
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

// cfEntityLabel resolves the display label for a custom-field entity type value.
func cfEntityLabel(value string) string {
	for _, e := range cfEntities {
		if e.Value == value {
			return uistate.T(e.Key)
		}
	}
	return value
}

// fldFocusSoon focuses the element with the given id on the next tick — after
// the render that mounts/unmounts it has committed — so keyboard focus follows
// the delete-confirm swap instead of being dumped to <body>.
func fldFocusSoon(id string) {
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		cb.Release()
		el := js.Global().Get("document").Call("getElementById", id)
		if el.Truthy() {
			el.Call("focus")
		}
		return nil
	})
	js.Global().Call("setTimeout", cb, 50)
}

// focusComposerKey scrolls the composer's key input into view and focuses it —
// the landing point for the per-group "Define one" shortcuts.
func focusComposerKey() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	el := doc.Call("getElementById", "fld-key")
	if !el.Truthy() {
		return
	}
	el.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "center"})
	el.Call("focus", map[string]any{"preventScroll": true})
}

// CustomFieldsManager is the /fields surface: a from-scratch "schema ledger"
// (FEATURE_MAP §5.3, rebuilt 2026-07-04). The left column is the field
// registry — one ruled group per entity, each definition a spec line showing
// its type, label, key, options and (for numbers) the live cf_* formula
// variable it feeds. The right rail is a sticky composer whose "what this
// field will do" footprint updates as you type. Definitions are validated and
// persisted through the internal/customfields engine as before.
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

	// defineFor is the per-group shortcut: pre-select the entity in the
	// composer and put the cursor in the key input.
	defineFor := func(entityVal string) {
		entity.Set(entityVal)
		focusComposerKey()
	}

	// ── Registry: one ruled ledger group per entity ─────────────────────────
	defs := app.CustomFieldDefs()
	countStr := uistate.T("fld.countNone")
	switch {
	case len(defs) == 1:
		countStr = uistate.T("fld.countOne")
	case len(defs) > 1:
		countStr = uistate.T("fld.countMany", len(defs))
	}

	// Populated entities become full ledger groups; empty ones compress into a
	// single "Nothing yet on …" line of define-shortcut chips so real schema
	// always visually leads (a fresh install is one quiet line, not six empty
	// sections).
	groups := make([]any, 0, len(cfEntities))
	var empty []struct{ Value, Key string }
	for _, e := range cfEntities {
		var rows []customfields.Def
		for _, d := range defs {
			if d.EntityType == e.Value {
				rows = append(rows, d)
			}
		}
		if len(rows) == 0 {
			empty = append(empty, e)
			continue
		}
		groups = append(groups, ui.CreateElement(fldGroup, fldGroupProps{
			Entity:   e.Value,
			Title:    uistate.T(e.Key),
			Defs:     rows,
			OnDelete: deleteDef,
			OnDefine: defineFor,
		}))
	}

	undefinedLine := If(len(empty) > 0, Div(css.Class("fld-undefined"),
		Span(css.Class("fld-undef-label"), uistate.T("fld.undefinedLabel")),
		MapKeyed(empty,
			func(e struct{ Value, Key string }) any { return e.Value },
			func(e struct{ Value, Key string }) ui.Node {
				return ui.CreateElement(fldDefineChip, fldDefineChipProps{
					Entity: e.Value, Title: uistate.T(e.Key), OnDefine: defineFor,
				})
			}),
	))

	registry := Div(css.Class("fld-registry"), Attr("data-testid", "fields-registry"),
		Div(css.Class("fld-reg-head"),
			Span(css.Class("fld-kicker"), uistate.T("fld.registryKicker")),
			Span(css.Class("fld-reg-count"), countStr),
		),
		P(css.Class("fld-reg-lede"), uistate.T("fld.registryLede")),
		Div(append([]any{css.Class("fld-groups")}, groups...)...),
		undefinedLine,
	)

	// ── Composer: labeled controls + the live footprint ─────────────────────
	entityOptions := make([]ui.Node, 0, len(cfEntities))
	for _, e := range cfEntities {
		entityOptions = append(entityOptions, Option(Value(e.Value), SelectedIf(entity.Get() == e.Value), uistate.T(e.Key)))
	}
	typeOptions := make([]ui.Node, 0, len(cfTypes))
	for _, ty := range cfTypes {
		typeOptions = append(typeOptions, Option(Value(string(ty.Value)), SelectedIf(ftype.Get() == string(ty.Value)), uistate.T(ty.Key)))
	}

	isChoice := ftype.Get() == string(customfields.TypeSelect)
	isNumber := ftype.Get() == string(customfields.TypeNumber)
	trimmedKey := strings.TrimSpace(key.Get())
	fvarPreview := ""
	if isNumber && trimmedKey != "" {
		fvarPreview = engineenv.CustomFieldVar(customfields.Def{
			Type: customfields.TypeNumber, EntityType: entity.Get(), Key: trimmedKey,
		})
	}

	fldField := func(lbl string, control ui.Node) ui.Node {
		return Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), lbl),
			control,
		)
	}

	foot := Div(css.Class("fld-foot"),
		Span(css.Class("fld-foot-title"), uistate.T("fld.footTitle")),
		P(css.Class("fld-foot-line"), uistate.T("fld.footForm", cfEntityLabel(entity.Get()))),
		If(entity.Get() == "transaction",
			P(css.Class("fld-foot-line"), uistate.T("fld.footReports"))),
		If(fvarPreview != "",
			P(css.Class("fld-foot-line"), uistate.T("fld.footFormula"), " ",
				Span(css.Class("fld-var"), fvarPreview))),
		If(isNumber && trimmedKey == "",
			P(css.Class("fld-foot-line"), uistate.T("fld.footFormulaHint"))),
		If(required.Get() == "yes",
			P(css.Class("fld-foot-line"), uistate.T("fld.footRequired"))),
	)

	composer := Div(css.Class("fld-composer"), Attr("data-testid", "fields-composer"),
		H2(css.Class("fld-comp-title"), uistate.T("cf.addTitle")),
		P(css.Class("fld-comp-lede"), uistate.T("fld.compLede")),
		Form(css.Class("fld-form"), OnSubmit(add),
			fldField(uistate.T("fld.livesOn"),
				Select(css.Class("field"), OnChange(onEntity), entityOptions)),
			Label(css.Class("fld-field"),
				Span(css.Class("fld-lbl"), uistate.T("cf.keyLabel")),
				Input(append([]any{css.Class("field"), Attr("id", "fld-key"), Type("text"), Placeholder(uistate.T("cf.keyPlaceholder")), Attr("pattern", "[A-Za-z0-9_]+"), Value(key.Get()), OnInput(onKey)}, errAttrs("cf-err", errMsg.Get())...)...),
				Span(css.Class("fld-hint"), uistate.T("cf.keyTitle")),
			),
			fldField(uistate.T("cf.labelLabel"),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("cf.labelPlaceholder")), Value(label.Get()), OnInput(onLabel))),
			fldField(uistate.T("fld.typeLabel"),
				Select(css.Class("field"), OnChange(onType), typeOptions)),
			If(isChoice, fldField(uistate.T("fld.choicesLabel"),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("cf.optionsPlaceholder")), Value(options.Get()), OnInput(onOptions)))),
			fldField(uistate.T("fld.requirementLabel"),
				Select(css.Class("field"), OnChange(onRequired),
					Option(Value("no"), SelectedIf(required.Get() == "no"), uistate.T("cf.optional")),
					Option(Value("yes"), SelectedIf(required.Get() == "yes"), uistate.T("cf.required")),
				)),
			foot,
			Button(css.Class("btn btn-primary fld-submit"), Type("submit"), uistate.T("cf.addField")),
		),
		errText("cf-err", errMsg.Get()),
	)

	return Div(css.Class("fld-deck"), Attr("data-testid", "fields-deck"), registry, composer)
}

// cfTypeLabel renders a field type as the human label used in the form.
func cfTypeLabel(t customfields.FieldType) string {
	for _, ty := range cfTypes {
		if ty.Value == t {
			return uistate.T(ty.Key)
		}
	}
	return string(t)
}

// fldGroupProps drives one registry group: an entity, its field definitions,
// delete for its rows, and the "define one" shortcut into the composer.
type fldGroupProps struct {
	Entity   string
	Title    string
	Defs     []customfields.Def
	OnDelete func(string)
	OnDefine func(string)
}

// fldGroup renders one populated entity's ledger section: serif title, field
// count, an "Add another" shortcut into the composer, and the spec-line rows.
// It owns its shortcut handler so hooks stay stable per group.
func fldGroup(p fldGroupProps) ui.Node {
	define := ui.UseEvent(Prevent(func() { p.OnDefine(p.Entity) }))

	head := Div(css.Class("fld-group-head"),
		H3(css.Class("fld-group-title"), p.Title),
		Span(css.Class("fld-group-count"), fmt.Sprintf("%d", len(p.Defs))),
		Button(css.Class("fld-define"), Type("button"),
			Attr("aria-label", uistate.T("fld.defineFor", p.Title)),
			OnClick(define), uistate.T("fld.addAnother")),
	)

	renderRow := func(d customfields.Def) ui.Node {
		return ui.CreateElement(CustomFieldRow, customFieldRowProps{Def: d, OnDelete: p.OnDelete})
	}
	keyOf := func(d customfields.Def) any { return d.ID }

	return Div(css.Class("fld-group"), head,
		Div(css.Class("fld-rows"), MapKeyed(p.Defs, keyOf, renderRow)))
}

// fldDefineChipProps drives one "Nothing yet on …" shortcut chip: the entity
// it starts a definition for, its display title, and the composer callback.
type fldDefineChipProps struct {
	Entity   string
	Title    string
	OnDefine func(string)
}

// fldDefineChip is a dashed "empty slot" chip for an entity with no fields
// yet; clicking it pre-selects the entity in the composer. It owns its click
// hook so it is safe to render from the variable-length empty-entity list.
func fldDefineChip(p fldDefineChipProps) ui.Node {
	click := ui.UseEvent(Prevent(func() { p.OnDefine(p.Entity) }))
	return Button(css.Class("fld-undef-chip"), Type("button"),
		Attr("aria-label", uistate.T("fld.defineFor", p.Title)),
		OnClick(click), p.Title)
}

type customFieldRowProps struct {
	Def      customfields.Def
	OnDelete func(string)
}

// CustomFieldRow is a per-definition spec line: boxed type tag, label
// (+ required marker), then a sub-line with the monospace key, the cf_*
// formula chip a number field feeds, and choice options. The chip lives in
// the sub-line (not its own grid track) so delete buttons stay ruled-column
// aligned whether or not a row has a formula variable. Deleting is a
// two-step inline confirm — field data on existing records and any formulas
// using the variable go with it, so it never fires off a single click.
func CustomFieldRow(props customFieldRowProps) ui.Node {
	confirming := ui.UseState(false)
	ask := ui.UseEvent(Prevent(func() {
		confirming.Set(true)
		fldFocusSoon("fld-keep-" + props.Def.ID) // land on the safe choice
	}))
	keep := ui.UseEvent(Prevent(func() {
		confirming.Set(false)
		fldFocusSoon("fld-del-" + props.Def.ID) // hand focus back to the ×
	}))
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Def.ID) }))
	d := props.Def
	fvar := engineenv.CustomFieldVar(d)

	warn := uistate.T("fld.deleteWarn")
	if fvar != "" {
		warn += " " + uistate.T("fld.deleteFormulaWarn", fvar)
	}

	return Div(css.Class("fld-row"),
		Span(css.Class("fld-type"), cfTypeLabel(d.Type)),
		Div(css.Class("fld-row-main"),
			Div(css.Class("fld-row-top"),
				Span(css.Class("fld-label"), d.Label),
				If(d.Required, Span(css.Class("fld-req"), uistate.T("cf.required"))),
			),
			Div(css.Class("fld-row-sub"),
				Span(css.Class("fld-key"), d.Key),
				If(fvar != "", Span(css.Class("fld-var"), Attr("title", uistate.T("fld.varTitle")), fvar)),
				If(d.Type == customfields.TypeSelect && len(d.Options) > 0,
					Span(css.Class("fld-opts"), strings.Join(d.Options, " · "))),
			),
		),
		If(!confirming.Get(),
			Button(css.Class("btn-del"), Type("button"), Attr("id", "fld-del-"+d.ID), Attr("aria-label", uistate.T("cf.deleteTitle")), Title(uistate.T("cf.deleteTitle")), OnClick(ask), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4)))),
		If(confirming.Get(), Div(css.Class("fld-confirm"), Attr("role", "alert"),
			Span(css.Class("fld-confirm-msg"), warn),
			Button(css.Class("fld-confirm-del"), Type("button"), OnClick(del), uistate.T("fld.deleteYes")),
			Button(css.Class("fld-confirm-keep"), Type("button"), Attr("id", "fld-keep-"+d.ID), OnClick(keep), uistate.T("fld.deleteNo")),
		)),
	)
}
