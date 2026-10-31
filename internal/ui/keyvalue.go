// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// KeyValueFact is one labelled fact in a KeyValue list.
type KeyValueFact struct {
	// Label names the fact. It is rendered in the same quiet style as a stat label.
	Label string
	// Value is the fact's content, such as text, a MoneyFig or a Pill.
	Value uic.Node
	// Tone is an optional class appended to the value, such as "pos" or "neg". Empty keeps the
	// default colour.
	Tone string
	// TestID is the data-testid of the fact's value element. Empty adds none.
	TestID string
}

// KeyValueProps configures a KeyValue list.
type KeyValueProps struct {
	// Facts are rendered in order, one label and value pair each.
	Facts []KeyValueFact
	// Class adds classes to the <dl> root.
	Class string
	// TestID is the data-testid of the <dl>. Empty adds none.
	TestID string
}

// KeyValue renders a <dl> of labelled facts. Each fact is a .kv-row holding a <dt> that shares
// the .stat-label look and a <dd> value. It owns no hooks, so it is safe inside a
// variable-length loop.
func KeyValue(props KeyValueProps) uic.Node {
	cls := "kv"
	if props.Class != "" {
		cls += " " + props.Class
	}
	args := []any{ClassStr(cls)}
	if props.TestID != "" {
		args = append(args, Attr("data-testid", props.TestID))
	}
	for _, f := range props.Facts {
		valueCls := "kv-value"
		if f.Tone != "" {
			valueCls += " " + f.Tone
		}
		value := []any{ClassStr(valueCls)}
		if f.TestID != "" {
			value = append(value, Attr("data-testid", f.TestID))
		}
		if f.Value != nil {
			value = append(value, f.Value)
		}
		args = append(args, Div(ClassStr("kv-row"),
			Tag("dt", ClassStr("stat-label kv-label"), f.Label),
			Tag("dd", value...),
		))
	}
	return Tag("dl", args...)
}
