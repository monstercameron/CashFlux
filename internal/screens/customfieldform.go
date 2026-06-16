//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/customfields"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// customFieldInputProps drives one custom-field input. Value is the current
// string value; OnChange reports the new (key, value) up to the parent form,
// which owns the value map. The component owns its own event hook, so it is safe
// to render many of these from a keyed list.
type customFieldInputProps struct {
	Def      customfields.Def
	Value    string
	OnChange func(key, value string)
}

// CustomFieldInput renders the right control for a custom field's data type and
// reports changes up to the parent. Both event hooks are declared unconditionally
// so hook order is stable regardless of the field type.
func CustomFieldInput(props customFieldInputProps) ui.Node {
	d := props.Def
	onText := ui.UseEvent(func(v string) { props.OnChange(d.Key, v) })
	onSel := ui.UseEvent(func(e ui.Event) { props.OnChange(d.Key, e.GetValue()) })

	label := d.Label
	if d.Required {
		label += " (required)"
	}

	switch d.Type {
	case customfields.TypeBool:
		return Select(Class("field"), Title(label), OnChange(onSel),
			Option(Value(""), SelectedIf(props.Value == ""), label+"…"),
			Option(Value("true"), SelectedIf(props.Value == "true"), "Yes"),
			Option(Value("false"), SelectedIf(props.Value == "false"), "No"),
		)
	case customfields.TypeSelect:
		opts := []ui.Node{Option(Value(""), SelectedIf(props.Value == ""), label+"…")}
		for _, o := range d.Options {
			opts = append(opts, Option(Value(o), SelectedIf(props.Value == o), o))
		}
		return Select(Class("field"), Title(label), OnChange(onSel), opts)
	case customfields.TypeNumber:
		return Input(Class("field"), Type("number"), Step("any"), Title(label), Placeholder(label), Value(props.Value), OnInput(onText))
	case customfields.TypeDate:
		return Input(Class("field"), Type("date"), Title(label), Value(props.Value), OnInput(onText))
	default: // text
		return Input(Class("field"), Type("text"), Title(label), Placeholder(label), Value(props.Value), OnInput(onText))
	}
}

// customValuesToMap converts the form's string values into a typed custom{} map
// against the given definitions: numbers become float64 (or the raw string if
// unparseable, so validation can flag it), yes/no becomes bool, everything else
// stays a string. Empty values are omitted so optional fields stay unset.
func customValuesToMap(defs []customfields.Def, vals map[string]string) map[string]any {
	if len(defs) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, d := range defs {
		raw := strings.TrimSpace(vals[d.Key])
		if raw == "" {
			continue
		}
		switch d.Type {
		case customfields.TypeNumber:
			if f, err := strconv.ParseFloat(raw, 64); err == nil {
				out[d.Key] = f
			} else {
				out[d.Key] = raw
			}
		case customfields.TypeBool:
			out[d.Key] = raw == "true"
		default:
			out[d.Key] = raw
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
