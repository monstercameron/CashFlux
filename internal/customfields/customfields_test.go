// SPDX-License-Identifier: MIT

package customfields

import "testing"

func defs() []Def {
	return []Def{
		{Key: "nickname", Label: "Nickname", Type: TypeText, Required: true},
		{Key: "apr", Label: "APR", Type: TypeNumber},
		{Key: "active", Label: "Active", Type: TypeBool},
		{Key: "tier", Label: "Tier", Type: TypeSelect, Options: []string{"gold", "silver"}},
		{Key: "opened", Label: "Opened", Type: TypeDate},
	}
}

func TestValidateAllGood(t *testing.T) {
	vals := map[string]any{
		"nickname": "Main",
		"apr":      float64(19.99),
		"active":   true,
		"tier":     "gold",
		"opened":   "2026-06-15",
	}
	if issues := Validate(defs(), vals); len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestValidateRequiredMissing(t *testing.T) {
	issues := Validate(defs(), map[string]any{})
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue (required nickname), got %v", issues)
	}
}

func TestValidateTypeMismatches(t *testing.T) {
	vals := map[string]any{
		"nickname": 5,            // not text
		"apr":      "high",       // not number
		"active":   "yes",        // not bool
		"tier":     "platinum",   // not in options
		"opened":   "2026-13-40", // bad date
	}
	issues := Validate(defs(), vals)
	if len(issues) != 5 {
		t.Errorf("expected 5 issues, got %d: %v", len(issues), issues)
	}
}

func TestValidateIgnoresUnknownKeys(t *testing.T) {
	vals := map[string]any{"nickname": "x", "extra": "ignored"}
	if issues := Validate(defs(), vals); len(issues) != 0 {
		t.Errorf("unknown keys should be ignored, got %v", issues)
	}
}

func TestParseFormValuesTypesAndOptionalBlanks(t *testing.T) {
	defs := []Def{
		{Key: "amount", Label: "Amount", Type: TypeNumber},
		{Key: "date", Label: "Date", Type: TypeDate},
		{Key: "enabled", Label: "Enabled", Type: TypeBool},
		{Key: "choice", Label: "Choice", Type: TypeSelect, Options: []string{"a", "b"}},
		{Key: "optional", Label: "Optional", Type: TypeText},
	}
	got, issues := ParseFormValues(defs, map[string]string{
		"amount": "12.5", "date": "2026-10-21", "enabled": "false", "choice": "b",
		"optional": "  ", "unknown": "ignored",
	})
	if len(issues) != 0 {
		t.Fatalf("ParseFormValues issues = %v", issues)
	}
	if got["amount"] != float64(12.5) || got["date"] != "2026-10-21" || got["enabled"] != false || got["choice"] != "b" {
		t.Fatalf("ParseFormValues = %#v", got)
	}
	if _, ok := got["optional"]; ok {
		t.Fatalf("blank optional value was retained: %#v", got)
	}
}

func TestParseFormValuesRejectsMalformedAndReturnsNoPartialMap(t *testing.T) {
	tests := []struct {
		name  string
		def   Def
		value string
		want  string
	}{
		{"number", Def{Key: "x", Label: "Amount", Type: TypeNumber}, "bad", "Amount must be a number."},
		{"number nan", Def{Key: "x", Label: "Amount", Type: TypeNumber}, "NaN", "Amount must be a number."},
		{"number infinity", Def{Key: "x", Label: "Amount", Type: TypeNumber}, "+Inf", "Amount must be a number."},
		{"number overflow", Def{Key: "x", Label: "Amount", Type: TypeNumber}, "1e999", "Amount must be a number."},
		{"date", Def{Key: "x", Label: "Date", Type: TypeDate}, "2026-02-30", "Date must be a valid date (YYYY-MM-DD)."},
		{"boolean", Def{Key: "x", Label: "Enabled", Type: TypeBool}, "yes", "Enabled must be true or false."},
		{"select", Def{Key: "x", Label: "Choice", Type: TypeSelect, Options: []string{"a"}}, "b", "Choice must be one of the allowed options."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, issues := ParseFormValues([]Def{tt.def, {Key: "valid", Label: "Valid", Type: TypeText}}, map[string]string{"x": tt.value, "valid": "kept"})
			if got != nil {
				t.Fatalf("invalid form returned partial map: %#v", got)
			}
			if len(issues) != 1 || issues[0] != tt.want {
				t.Fatalf("issues = %v, want [%q]", issues, tt.want)
			}
		})
	}
}

func TestParseFormValuesRequiredAndEmpty(t *testing.T) {
	defs := []Def{{Key: "name", Label: "Name", Type: TypeText, Required: true}}
	got, issues := ParseFormValues(defs, map[string]string{"name": "  "})
	if got != nil || len(issues) != 1 || issues[0] != "Name is required." {
		t.Fatalf("empty required value = %#v, %v", got, issues)
	}
	got, issues = ParseFormValues(nil, map[string]string{"unused": "value"})
	if got != nil || len(issues) != 0 {
		t.Fatalf("no definitions = %#v, %v", got, issues)
	}
}

func TestDefValidate(t *testing.T) {
	good := Def{ID: "f1", EntityType: "account", Key: "tier", Label: "Tier", Type: TypeSelect, Options: []string{"gold"}}
	if issues := good.Validate(); len(issues) != 0 {
		t.Errorf("expected sound def, got %v", issues)
	}
	bad := Def{} // missing everything; type invalid
	if issues := bad.Validate(); len(issues) != 5 {
		t.Errorf("expected 5 issues for empty def, got %d: %v", len(issues), issues)
	}
	selNoOpts := Def{ID: "f2", EntityType: "account", Key: "tier", Label: "Tier", Type: TypeSelect}
	if issues := selNoOpts.Validate(); len(issues) != 1 {
		t.Errorf("expected 1 issue (choice needs options), got %v", issues)
	}
}

func TestDefValidateKeyFormat(t *testing.T) {
	base := Def{ID: "f1", EntityType: "account", Label: "Reference", Type: TypeText}
	tests := []struct {
		name string
		key  string
		ok   bool
	}{
		{"letters", "branch", true},
		{"digits", "field2", true},
		{"underscore", "branch_code", true},
		{"space", "branch code", false},
		{"hyphen", "branch-code", false},
		{"punctuation", "branch.code", false},
		{"reserved", "id", false},
		{"reserved case insensitive", "EntityType", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := base
			d.Key = tt.key
			issues := d.Validate()
			if tt.ok && len(issues) != 0 {
				t.Fatalf("expected key %q to be valid, got %v", tt.key, issues)
			}
			if !tt.ok && len(issues) == 0 {
				t.Fatalf("expected key %q to be invalid", tt.key)
			}
		})
	}
}

func TestFieldTypeValid(t *testing.T) {
	for _, ty := range []FieldType{TypeText, TypeNumber, TypeDate, TypeBool, TypeSelect} {
		if !ty.Valid() {
			t.Errorf("%q should be valid", ty)
		}
	}
	if FieldType("color").Valid() {
		t.Error("color should be invalid")
	}
}
