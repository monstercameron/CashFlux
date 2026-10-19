// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestFieldWiresLabelDescriptionAndInvalidState(t *testing.T) {
	f := render.New(t)
	f.Render(Field(FieldProps{
		ID: "payee", Label: "Payee", Hint: "Use the name on the receipt.", Error: "Enter a payee.",
		Control: func(a11y FieldA11y) uic.Node {
			return Input(ID(a11y.ID), Attr("aria-label", a11y.Label), Type("text"), Attr("aria-describedby", a11y.DescribedBy), Attr("aria-invalid", boolAttribute(a11y.Invalid)))
		},
	}))
	control := f.ByRole("textbox", "Payee")
	if !control.Exists() {
		t.Fatal("Field label is not associated with its control")
	}
	if got := control.Attr("aria-describedby"); got != "payee-hint payee-error" {
		t.Fatalf("aria-describedby = %q", got)
	}
	if got := control.Attr("aria-invalid"); got != "true" {
		t.Fatalf("aria-invalid = %q, want true", got)
	}
}

func TestAmountFieldParsesMinorUnitsAndExposesValidation(t *testing.T) {
	f := render.New(t)
	var parsed money.Money
	var parseErr error
	f.Render(AmountField(AmountFieldProps{
		ID: "amount", Label: "Amount", Value: "12.34", Currency: "USD", Hint: "Enter the transaction amount.", Error: "Check the amount.",
		OnParse: func(value money.Money, err error) { parsed, parseErr = value, err },
	}))
	inputs := f.AllByTag("input")
	if len(inputs) != 1 {
		t.Fatal("AmountField has no labelled numeric input")
	}
	input := inputs[0]
	if input.Attr("type") != "number" || input.Attr("aria-label") != "Amount" {
		t.Fatalf("AmountField input type/name = %q/%q, want number/Amount", input.Attr("type"), input.Attr("aria-label"))
	}
	if got := input.Attr("aria-describedby"); got != "amount-hint amount-error" {
		t.Fatalf("aria-describedby = %q", got)
	}
	if got := input.Attr("aria-invalid"); got != "true" {
		t.Fatalf("aria-invalid = %q, want true", got)
	}
	input.Dispatch("oninput", render.Event{Value: "12.34"})
	if parseErr != nil || parsed.Amount != 1234 || parsed.Currency != "USD" {
		t.Fatalf("valid amount parsed as %+v, %v; want 1234 USD", parsed, parseErr)
	}
	input.Dispatch("oninput", render.Event{Value: "12.345"})
	if parseErr == nil {
		t.Fatal("too many fractional digits were accepted")
	}
}

func TestParseAmountFieldCurrencyPrecision(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		currency string
		want     int64
		wantErr  bool
	}{
		{"dollar cents", "10.25", "USD", 1025, false},
		{"zero-decimal yen", "120", "JPY", 120, false},
		{"reject extra decimal", "1.001", "USD", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseAmountField(tc.value, tc.currency)
			if (err != nil) != tc.wantErr || (!tc.wantErr && (got.Amount != tc.want || got.Currency != tc.currency)) {
				t.Fatalf("ParseAmountField(%q, %q) = %+v, %v", tc.value, tc.currency, got, err)
			}
		})
	}
}

func boolAttribute(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
