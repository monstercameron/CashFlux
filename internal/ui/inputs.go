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
	AriaLabel   string       // accessible name when there's no visible <label>
	OnInput     func(string) // the caller's handler; the field owns the hook
	Required    bool
	Disabled    bool
	Class       string // extra classes appended to the base ".field"
	// Rules are css.Rule values folded into the class list. They cannot travel in
	// Extra: a bare css.Rule is not a prop option, and the renderer emits it as a
	// stray text node beside the field rather than applying it.
	Rules []any
	// Extra carries any additional element options the call site needs — an id, a
	// data-testid, autocomplete, a keydown handler. It exists so migrating a
	// hand-rolled Input() to this component is a move rather than a rewrite.
	Extra []any
}

func fieldArgs(p TextFieldProps) []any {
	cls := "field"
	if p.Class != "" {
		cls += " " + p.Class
	}
	// Deliberately NO Value() option — see fieldCore. The value reaches the element
	// through the layout effect instead, so the reconciler can never write it.
	args := []any{css.Class(append([]any{cls}, p.Rules...)...)}
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
	return append(args, p.Extra...)
}

// TextInput renders a single-line text input using the shared .field styling.
//
// It keeps what the user types. See fieldCore for why that needs saying.
func TextInput(p TextFieldProps) uic.Node {
	return Input(append([]any{Type("text")}, append(fieldArgs(p), FieldValue(p.Value), OnInput(p.OnInput))...)...)
}

// NumberInput renders a numeric input (Step defaults to "1" via the browser).
func NumberInput(p TextFieldProps) uic.Node {
	return Input(append([]any{Type("number")}, append(fieldArgs(p), FieldValue(p.Value), OnInput(p.OnInput))...)...)
}

// TextAreaInput renders a multi-line text input using the .field styling.
//
// Multi-line fields hit the value-clobbering bug first and hardest — a budget note
// typed as "PROBE NOTE alpha" arriving as "PROBE NOTE alpa", and saving empty
// while the box plainly showed the text — because the content of a textarea is
// rewritten wholesale under the cursor. It is fixed at the root now: fieldCore
// keeps the value out of the reconciler's hands entirely, for every field shape.
func TextAreaInput(p TextFieldProps) uic.Node {
	return Textarea(append(fieldArgs(p), FieldValue(p.Value), OnInput(p.OnInput))...)
}

// MoneyInputProps configures a currency-aware MoneyInput.
type MoneyInputProps struct {
	Value     string
	Currency  string // ISO code; drives the step (decimal places) and symbol affix
	AriaLabel string
	OnInput   func(string)
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
	inArgs := []any{css.Class("field"), Type("number"), Step(step)}
	if p.AriaLabel != "" {
		inArgs = append(inArgs, Attr("aria-label", p.AriaLabel))
	}
	if p.Disabled {
		inArgs = append(inArgs, Attr("disabled", "true"))
	}
	sym := currency.Symbol(p.Currency)
	if sym == "" {
		sym = p.Currency
	}
	return Label(css.Class("money-input"),
		Span(css.Class("money-input-affix"), Attr("aria-hidden", "true"), sym),
		// An amount is the field where a dropped digit is most expensive: 1250
		// silently becoming 120 is a wrong number that still looks like a number.
		Input(append(inArgs, FieldValue(p.Value), OnInput(p.OnInput))...),
	)
}

// SuggestProps configures a Combobox.
type SuggestProps struct {
	Value       string
	Placeholder string
	AriaLabel   string
	OnInput     func(string)
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
	inArgs := []any{css.Class("field"), Type("text"), Attr("list", listID), Attr("autocomplete", "off")}
	if p.Placeholder != "" {
		inArgs = append(inArgs, Placeholder(p.Placeholder))
	}
	if p.AriaLabel != "" {
		inArgs = append(inArgs, Attr("aria-label", p.AriaLabel))
	}
	opts := make([]any, 0, len(p.Options)+1)
	opts = append(opts, ID(listID))
	for _, o := range p.Options {
		opts = append(opts, Option(Attr("value", o)))
	}
	return Fragment(
		Input(append(inArgs, FieldValue(p.Value), OnInput(p.OnInput))...),
		Datalist(opts...),
	)
}
