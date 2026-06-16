//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/id"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// cfEntities are the entity types that can carry custom fields, in display order.
var cfEntities = []struct{ Value, Label string }{
	{"account", "Accounts"},
	{"transaction", "Transactions"},
	{"budget", "Budgets"},
	{"goal", "Goals"},
	{"member", "Members"},
}

// cfTypes are the supported custom-field data types, in display order.
var cfTypes = []struct {
	Value customfields.FieldType
	Label string
}{
	{customfields.TypeText, "Text"},
	{customfields.TypeNumber, "Number"},
	{customfields.TypeDate, "Date"},
	{customfields.TypeBool, "Yes / no"},
	{customfields.TypeSelect, "Choice"},
}

// CustomFieldsManager lets you define extra fields on your entities: pick the
// entity type, a key and label, a data type, and (for choice fields) the allowed
// options. Definitions are validated and persisted; existing ones list below with
// per-row delete. This is the management UI for the internal/customfields engine.
func CustomFieldsManager() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
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
			Options:    parseOptions(options.Get()),
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
		entityOptions = append(entityOptions, Option(Value(e.Value), SelectedIf(entity.Get() == e.Value), e.Label))
	}
	typeOptions := make([]ui.Node, 0, len(cfTypes))
	for _, ty := range cfTypes {
		typeOptions = append(typeOptions, Option(Value(string(ty.Value)), SelectedIf(ftype.Get() == string(ty.Value)), ty.Label))
	}

	isChoice := ftype.Get() == string(customfields.TypeSelect)

	form := Section(Class("card"),
		H2(Class("card-title"), "Add a custom field"),
		P(Class("muted"), "Define your own fields on any entity. Choose where it lives, name it, and pick a type. Choice fields take a comma-separated list of options."),
		Form(Class("form-grid"), OnSubmit(add),
			Select(Class("field"), OnChange(onEntity), entityOptions),
			Input(Class("field"), Type("text"), Placeholder("Key (e.g. account_number)"), Value(key.Get()), OnInput(onKey)),
			Input(Class("field"), Type("text"), Placeholder("Label (e.g. Account number)"), Value(label.Get()), OnInput(onLabel)),
			Select(Class("field"), OnChange(onType), typeOptions),
			If(isChoice, Input(Class("field field-wide"), Type("text"), Placeholder("Options, comma-separated (e.g. gold, silver, bronze)"), Value(options.Get()), OnInput(onOptions))),
			Select(Class("field"), OnChange(onRequired),
				Option(Value("no"), SelectedIf(required.Get() == "no"), "Optional"),
				Option(Value("yes"), SelectedIf(required.Get() == "yes"), "Required"),
			),
			Button(Class("btn btn-primary"), Type("submit"), "Add field"),
		),
		If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
	)

	// Group existing defs by entity type for a tidy list.
	defs := app.CustomFieldDefs()
	sections := make([]any, 0, len(cfEntities))
	for _, e := range cfEntities {
		var rows []customfields.Def
		for _, d := range defs {
			if d.EntityType == e.Value {
				rows = append(rows, d)
			}
		}
		if len(rows) == 0 {
			continue
		}
		renderRow := func(d customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldRow, customFieldRowProps{Def: d, OnDelete: deleteDef})
		}
		keyOf := func(d customfields.Def) any { return d.ID }
		sections = append(sections, Section(Class("card"),
			H2(Class("card-title"), e.Label),
			Div(Class("rows"), MapKeyed(rows, keyOf, renderRow)),
		))
	}

	list := ui.Node(nil)
	if len(sections) == 0 {
		list = Section(Class("card"), P(Class("empty"), "No custom fields yet. Add one above."))
	} else {
		list = Fragment(sections...)
	}

	return Div(form, list)
}

// parseOptions splits a comma-separated option list, trimming blanks.
func parseOptions(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// cfTypeLabel renders a field type as the human label used in the form.
func cfTypeLabel(t customfields.FieldType) string {
	for _, ty := range cfTypes {
		if ty.Value == t {
			return ty.Label
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
		meta += " · required"
	}
	if props.Def.Type == customfields.TypeSelect && len(props.Def.Options) > 0 {
		meta += " · " + strings.Join(props.Def.Options, ", ")
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), props.Def.Label),
			Span(Class("row-meta"), props.Def.Key+" — "+meta),
		),
		Button(Class("btn-del"), Type("button"), Title("Delete custom field"), OnClick(del), "✕"),
	)
}
