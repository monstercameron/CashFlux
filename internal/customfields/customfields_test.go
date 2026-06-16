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
