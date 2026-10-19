// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// FieldA11y describes the accessible relationships a control must render.
type FieldA11y struct {
	ID, Label, DescribedBy string
	Invalid, Required      bool
}

// FieldControl renders a control using the accessibility details supplied by Field.
type FieldControl func(FieldA11y) uic.Node

// FieldProps configures a labelled control with optional hint and error text.
type FieldProps struct {
	ID, Label, Hint, Error string
	Control                FieldControl
	Required               bool
	TestID                 string
}

// Field renders a label, control, hint, and validation message as one field.
func Field(props FieldProps) uic.Node {
	return uic.CreateElement(field, props)
}

func field(props FieldProps) uic.Node {
	generatedID := uic.UseId()
	controlID := props.ID
	if controlID == "" {
		controlID = generatedID
	}
	ids := make([]string, 0, 2)
	if props.Hint != "" {
		ids = append(ids, controlID+"-hint")
	}
	if props.Error != "" {
		ids = append(ids, controlID+"-error")
	}
	control := uic.Node(nil)
	if props.Control != nil {
		control = props.Control(FieldA11y{
			ID: controlID, Label: props.Label, DescribedBy: strings.Join(ids, " "),
			Invalid: props.Error != "", Required: props.Required,
		})
	}
	args := []any{css.Class("cf-field", css.New(tw.Flex, tw.FlexCol, tw.Gap1))}
	if props.TestID != "" {
		args = append(args, Attr("data-testid", props.TestID))
	}
	label := []any{Attr("for", controlID)}
	label = append(label, props.Label)
	label = append([]any{css.Class("cf-field-label")}, label...)
	children := []any{Label(label...), control}
	if props.Hint != "" {
		children = append(children, Div(Attr("id", controlID+"-hint"), css.Class("cf-field-hint"), props.Hint))
	}
	if props.Error != "" {
		children = append(children, Div(Attr("id", controlID+"-error"), css.Class("cf-field-error"), Attr("role", "alert"), props.Error))
	}
	return Div(append(args, children...)...)
}

// AmountFieldProps configures a currency-aware decimal amount input.
type AmountFieldProps struct {
	ID, Label, Value, Currency string
	Hint, Error                string
	Required, Disabled         bool
	OnParse                    func(money.Money, error)
	TestID                     string
}

// AmountField renders a MoneyInput-style control and reports parsed integer minor units.
func AmountField(props AmountFieldProps) uic.Node {
	return uic.CreateElement(amountField, props)
}

func amountField(props AmountFieldProps) uic.Node {
	decimals := currency.Decimals(props.Currency)
	step := "1"
	if decimals > 0 {
		step = "0." + strings.Repeat("0", decimals-1) + "1"
	}
	symbol := currency.Symbol(props.Currency)
	if symbol == "" {
		symbol = props.Currency
	}
	return Field(FieldProps{
		ID: props.ID, Label: props.Label, Hint: props.Hint, Error: props.Error,
		Required: props.Required, TestID: props.TestID,
		Control: func(a11y FieldA11y) uic.Node {
			attrs := []any{ID(a11y.ID), Attr("aria-label", a11y.Label), Type("number"), Step(step), Attr("inputmode", "decimal"), FieldValue(props.Value), OnInput(func(value string) {
				if props.OnParse == nil {
					return
				}
				amount, err := ParseAmountField(value, props.Currency)
				props.OnParse(amount, err)
			})}
			if a11y.DescribedBy != "" {
				attrs = append(attrs, Attr("aria-describedby", a11y.DescribedBy))
			}
			if a11y.Invalid {
				attrs = append(attrs, Attr("aria-invalid", "true"))
			}
			if a11y.Required {
				attrs = append(attrs, Attr("aria-required", "true"))
			}
			if props.Disabled {
				attrs = append(attrs, Attr("disabled", "true"))
			}
			return Div(css.Class("cf-amount-field", css.New(tw.Flex, tw.ItemsCenter, tw.Gap2)),
				Span(Attr("aria-hidden", "true"), symbol),
				Input(append([]any{css.Class("field")}, attrs...)...),
			)
		},
	})
}

// ParseAmountField parses a decimal string into integer minor units.
func ParseAmountField(value, currencyCode string) (money.Money, error) {
	amount, err := money.ParseMinor(value, currency.Decimals(currencyCode))
	if err != nil {
		return money.Money{}, fmt.Errorf("parse %s amount: %w", currencyCode, err)
	}
	return money.New(amount, currencyCode), nil
}
