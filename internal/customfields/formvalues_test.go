// SPDX-License-Identifier: MIT

package customfields

import (
	"math"
	"reflect"
	"testing"
)

func TestFormValuesToStringsConvertsValues(t *testing.T) {
	got := FormValuesToStrings(map[string]any{
		"enabled":   true,
		"disabled":  false,
		"amount":    float64(12.5),
		"whole":     float64(12),
		"text":      "hello",
		"integer":   int64(42),
		"nil_value": nil,
		"infinity":  math.Inf(1),
	})
	want := map[string]string{
		"enabled":   "true",
		"disabled":  "false",
		"amount":    "12.5",
		"whole":     "12",
		"text":      "hello",
		"integer":   "42",
		"nil_value": "<nil>",
		"infinity":  "+Inf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FormValuesToStrings() = %#v, want %#v", got, want)
	}
}

func TestFormValuesToStringsEmptyInputReturnsEmptyMap(t *testing.T) {
	for _, input := range []map[string]any{nil, {}} {
		got := FormValuesToStrings(input)
		if got == nil {
			t.Fatalf("FormValuesToStrings(%#v) = nil, want non-nil empty map", input)
		}
		if len(got) != 0 {
			t.Fatalf("FormValuesToStrings(%#v) = %#v, want empty map", input, got)
		}
	}
}

func TestFormValuesToStringsRoundTripsFormValues(t *testing.T) {
	defs := []Def{
		{Key: "name", Label: "Name", Type: TypeText, Required: true},
		{Key: "amount", Label: "Amount", Type: TypeNumber},
		{Key: "enabled", Label: "Enabled", Type: TypeBool},
		{Key: "choice", Label: "Choice", Type: TypeSelect, Options: []string{"a", "b"}},
		{Key: "date", Label: "Date", Type: TypeDate},
	}
	want := map[string]any{
		"name":    "Main",
		"amount":  float64(12.5),
		"enabled": false,
		"choice":  "b",
		"date":    "2026-10-21",
	}

	formValues := FormValuesToStrings(want)
	got, issues := ParseFormValues(defs, formValues)
	if len(issues) != 0 {
		t.Fatalf("ParseFormValues() issues = %v", issues)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseFormValues(FormValuesToStrings(values)) = %#v, want %#v", got, want)
	}
}

func TestFormValuesToStringsDoesNotRelaxStrictParsing(t *testing.T) {
	defs := []Def{
		{Key: "amount", Label: "Amount", Type: TypeNumber},
		{Key: "name", Label: "Name", Type: TypeText},
	}
	formValues := FormValuesToStrings(map[string]any{"amount": "not a number", "name": "Main"})
	got, issues := ParseFormValues(defs, formValues)
	if got != nil {
		t.Fatalf("invalid form returned partial map: %#v", got)
	}
	if !reflect.DeepEqual(issues, []string{"Amount must be a number."}) {
		t.Fatalf("ParseFormValues() issues = %v, want number validation error", issues)
	}
}
