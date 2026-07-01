// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
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
		label += uistate.T("cf.requiredLabel")
	}

	switch d.Type {
	case customfields.TypeBool:
		return Select(css.Class("field"), Title(label), OnChange(onSel),
			Option(Value(""), SelectedIf(props.Value == ""), label+"…"),
			Option(Value("true"), SelectedIf(props.Value == "true"), uistate.T("cf.yes")),
			Option(Value("false"), SelectedIf(props.Value == "false"), uistate.T("cf.no")),
		)
	case customfields.TypeSelect:
		opts := []ui.Node{Option(Value(""), SelectedIf(props.Value == ""), label+"…")}
		for _, o := range d.Options {
			opts = append(opts, Option(Value(o), SelectedIf(props.Value == o), o))
		}
		return Select(css.Class("field"), Title(label), OnChange(onSel), opts)
	case customfields.TypeNumber:
		return Input(css.Class("field"), Type("number"), Step("any"), Title(label), Placeholder(label), Value(props.Value), OnInput(onText))
	case customfields.TypeDate:
		return Input(css.Class("field"), Type("date"), Title(label), Value(props.Value), OnInput(onText))
	default: // text
		return Input(css.Class("field"), Type("text"), Title(label), Placeholder(label), Value(props.Value), OnInput(onText))
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

// customMapToStrings renders a typed custom{} map back into the string form-state map
// the input components edit — the inverse of customValuesToMap (bool → "true"/"false",
// float → trimmed decimal, anything else → its default string form). Used to seed an
// inline-edit form from an entity's saved custom values.
func customMapToStrings(custom map[string]any) map[string]string {
	out := make(map[string]string, len(custom))
	for k, v := range custom {
		switch t := v.(type) {
		case bool:
			if t {
				out[k] = "true"
			} else {
				out[k] = "false"
			}
		case float64:
			out[k] = strconv.FormatFloat(t, 'f', -1, 64)
		case string:
			out[k] = t
		default:
			out[k] = fmt.Sprintf("%v", t)
		}
	}
	return out
}

// customSummary builds a compact "Label: value" string for an entity's non-empty
// custom fields, in def order, for a read-only row display. Empty when there are no
// defs or no values. Bool values render as Yes/No.
func customSummary(defs []customfields.Def, custom map[string]any) string {
	if len(defs) == 0 || len(custom) == 0 {
		return ""
	}
	parts := make([]string, 0, len(defs))
	for _, d := range defs {
		v, ok := custom[d.Key]
		if !ok {
			continue
		}
		var s string
		switch t := v.(type) {
		case bool:
			if t {
				s = uistate.T("cf.yes")
			} else {
				s = uistate.T("cf.no")
			}
		case float64:
			s = strconv.FormatFloat(t, 'f', -1, 64)
		case string:
			s = t
		default:
			s = fmt.Sprintf("%v", t)
		}
		if strings.TrimSpace(s) == "" {
			continue
		}
		parts = append(parts, d.Label+": "+s)
	}
	return strings.Join(parts, " · ")
}
