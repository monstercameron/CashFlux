// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// TextFieldProps configures TextInput / NumberInput / TextAreaInput.
type TextFieldProps struct {
	Value       string
	Placeholder string
	AriaLabel   string      // accessible name when there's no visible <label>
	OnInput     uic.Handler // required — the input's change handler (from UseEvent)
	Required    bool
	Disabled    bool
	Class       string // extra classes appended to the base ".field"
}

func fieldArgs(p TextFieldProps) []any {
	cls := "field"
	if p.Class != "" {
		cls += " " + p.Class
	}
	args := []any{css.Class(cls), Value(p.Value)}
	if p.Placeholder != "" {
		args = append(args, Placeholder(p.Placeholder))
	}
	if p.AriaLabel != "" {
		args = append(args, Attr("aria-label", p.AriaLabel))
	}
	if p.Required {
		args = append(args, Attr("aria-required", "true"))
	}
	if p.Disabled {
		args = append(args, Attr("disabled", "true"))
	}
	return args
}

// TextInput renders a single-line text input using the shared .field styling.
func TextInput(p TextFieldProps) uic.Node {
	return Input(append(fieldArgs(p), Type("text"), OnInput(p.OnInput))...)
}

// NumberInput renders a numeric input (Step defaults to "1" via the browser).
func NumberInput(p TextFieldProps) uic.Node {
	return Input(append(fieldArgs(p), Type("number"), OnInput(p.OnInput))...)
}

// TextAreaInput renders a multi-line text input using the .field styling.
//
// It binds the caller's handler to CHANGE, not input — deliberately, and this is
// the whole reason multi-line fields did not work.
//
// Two things went wrong with per-keystroke binding. The value is carried by
// fieldArgs' Value option, which the renderer writes as the textarea's content;
// so every keystroke set state, state re-rendered the node, and the re-render
// rewrote the content under the cursor. That dropped characters mid-word —
// "PROBE NOTE alpha" arriving as "PROBE NOTE alpa" — and left the value the
// submit handler read out of step with the text visibly in the box, so a budget
// note saved as empty while the field plainly showed it.
//
// A multi-line field does not need its state updated per character. Committing
// on blur is both correct and cheaper: no re-render while typing, and the value
// is captured before any Save button can be clicked, because clicking it blurs
// the field first.
//
// Callers still pass OnInput; the component picks the event a textarea actually
// wants, so no call site has to know this.
func TextAreaInput(p TextFieldProps) uic.Node {
	return Textarea(append(fieldArgs(p), OnChange(p.OnInput))...)
}

// MoneyInputProps configures a currency-aware MoneyInput.
type MoneyInputProps struct {
	Value     string
	Currency  string // ISO code; drives the step (decimal places) and symbol affix
	AriaLabel string
	OnInput   uic.Handler
	Disabled  bool
}

// MoneyInput renders a currency-aware amount field: a number input whose step
// matches the currency's minor-unit precision (e.g. 0.01 for USD, 1 for JPY),
// prefixed with the currency symbol so the user sees what they're entering. The
// affix + field share a flex wrapper; parsing/formatting stays the caller's job
// (money.ParseMinor), this primitive only standardizes the control + precision.
func MoneyInput(p MoneyInputProps) uic.Node {
	dec := currency.Decimals(p.Currency)
	step := "1"
	if dec > 0 {
		step = "0." + strings.Repeat("0", dec-1) + "1"
	}
	inArgs := []any{css.Class("field"), Type("number"), Step(step), Value(p.Value)}
	if p.AriaLabel != "" {
		inArgs = append(inArgs, Attr("aria-label", p.AriaLabel))
	}
	if p.Disabled {
		inArgs = append(inArgs, Attr("disabled", "true"))
	}
	inArgs = append(inArgs, OnInput(p.OnInput))
	sym := currency.Symbol(p.Currency)
	if sym == "" {
		sym = p.Currency
	}
	return Label(css.Class("money-input"),
		Span(css.Class("money-input-affix"), Attr("aria-hidden", "true"), sym),
		Input(inArgs...),
	)
}

// SuggestProps configures a Combobox.
type SuggestProps struct {
	Value       string
	Placeholder string
	AriaLabel   string
	OnInput     uic.Handler
	Options     []string // suggestion values offered via a native <datalist>
	ListID      string   // unique id tying the input to its datalist
}

// Combobox renders a free-text input with a native-datalist suggestion dropdown:
// the user can type freely or pick from Options (filterable by what they type).
// Built on <input list> + <datalist> so it needs no JS popover, stays keyboard-
// and screen-reader-accessible, and degrades to a plain text field. For a strict
// pick-one control use SelectInput instead.
func Combobox(p SuggestProps) uic.Node {
	listID := p.ListID
	if listID == "" {
		listID = "cf-combobox-list"
	}
	inArgs := []any{css.Class("field"), Type("text"), Value(p.Value), Attr("list", listID), Attr("autocomplete", "off")}
	if p.Placeholder != "" {
		inArgs = append(inArgs, Placeholder(p.Placeholder))
	}
	if p.AriaLabel != "" {
		inArgs = append(inArgs, Attr("aria-label", p.AriaLabel))
	}
	inArgs = append(inArgs, OnInput(p.OnInput))
	opts := make([]any, 0, len(p.Options)+1)
	opts = append(opts, ID(listID))
	for _, o := range p.Options {
		opts = append(opts, Option(Attr("value", o)))
	}
	return Fragment(Input(inArgs...), Datalist(opts...))
}
