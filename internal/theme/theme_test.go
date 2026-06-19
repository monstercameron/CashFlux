package theme

import (
	"strings"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	if issues := Default().Validate(); len(issues) != 0 {
		t.Fatalf("Default() should validate cleanly, got %+v", issues)
	}
}

func TestPresetsAreValid(t *testing.T) {
	ps := Presets()
	if len(ps) == 0 {
		t.Fatal("expected at least one preset")
	}
	for _, p := range ps {
		if issues := p.Validate(); len(issues) != 0 {
			t.Errorf("preset %q failed validation: %+v", p.Name, issues)
		}
	}
	// Sorted by name.
	for i := 1; i < len(ps); i++ {
		if ps[i-1].Name > ps[i].Name {
			t.Errorf("presets not sorted: %q before %q", ps[i-1].Name, ps[i].Name)
		}
	}
}

func TestPresetLookup(t *testing.T) {
	if _, ok := Preset("Paper"); !ok {
		t.Error("Paper preset should exist")
	}
	if _, ok := Preset("Nope"); ok {
		t.Error("unknown preset should not be found")
	}
}

func TestValidateBadColor(t *testing.T) {
	bad := Default()
	bad.Accent = "not-a-color"
	issues := bad.Validate()
	if !hasField(issues, "accent") {
		t.Errorf("expected an accent color issue, got %+v", issues)
	}
}

func TestValidateLowContrast(t *testing.T) {
	bad := Default()
	bad.Text = bad.BgBase // text identical to background → ratio 1.0
	issues := bad.Validate()
	if !hasField(issues, "text") {
		t.Fatalf("expected a text contrast issue, got %+v", issues)
	}
	for _, is := range issues {
		if is.Field == "text" && is.Ratio == 0 {
			t.Error("contrast issue should report the failing ratio")
		}
	}
}

func TestValidateNonColorTokens(t *testing.T) {
	bad := Default()
	bad.Radius = 999
	bad.Scale = 5
	bad.Density = "huge"
	issues := bad.Validate()
	for _, field := range []string{"radius", "scale", "density"} {
		if !hasField(issues, field) {
			t.Errorf("expected a %s issue, got %+v", field, issues)
		}
	}
}

func TestCSSVars(t *testing.T) {
	vars := Default().CSSVars()
	checks := map[string]string{
		"--bg-base":  "#0e1116",
		"--accent":   "#7c83ff",
		"--radius":   "12px",
		"--ui-scale": "1",
		"--density":  "comfortable",
	}
	for k, want := range checks {
		if got := vars[k]; got != want {
			t.Errorf("CSSVars()[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestMerge(t *testing.T) {
	base := Default()
	merged := base.Merge(Theme{Accent: "#ff0000", Radius: 4})
	if merged.Accent != "#ff0000" {
		t.Errorf("Accent = %q, want override #ff0000", merged.Accent)
	}
	if merged.Radius != 4 {
		t.Errorf("Radius = %d, want override 4", merged.Radius)
	}
	// Unset override fields keep the base value.
	if merged.BgBase != base.BgBase {
		t.Errorf("BgBase = %q, want base %q (empty override should not clear it)", merged.BgBase, base.BgBase)
	}
	if merged.Scale != base.Scale {
		t.Errorf("Scale = %g, want base %g (zero override should not clear it)", merged.Scale, base.Scale)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	orig := Default()
	orig.Name = "My Theme"
	orig.Accent = "#abcdef"
	data, err := orig.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	got, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if got != orig {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, orig)
	}
}

func TestFromJSONFillsMissingFields(t *testing.T) {
	// A partial theme file (only an accent) should still produce a complete,
	// valid theme by inheriting Default()'s other tokens.
	got, err := FromJSON([]byte(`{"name":"Partial","accent":"#3344ff"}`))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if got.Accent != "#3344ff" {
		t.Errorf("Accent = %q, want #3344ff", got.Accent)
	}
	if got.BgBase != Default().BgBase {
		t.Errorf("BgBase = %q, want inherited default %q", got.BgBase, Default().BgBase)
	}
	if issues := got.Validate(); len(issues) != 0 {
		t.Errorf("partial theme should still validate, got %+v", issues)
	}
}

func TestFromJSONBadInput(t *testing.T) {
	if _, err := FromJSON([]byte("{not json")); err == nil {
		t.Error("FromJSON should error on malformed JSON")
	}
}

func hasField(issues []Issue, field string) bool {
	for _, is := range issues {
		if is.Field == field {
			return true
		}
	}
	return false
}

func TestIssueMessagesAreFriendly(t *testing.T) {
	bad := Default()
	bad.TextDim = bad.BgCard
	for _, is := range bad.Validate() {
		if strings.TrimSpace(is.Message) == "" {
			t.Errorf("issue for %q has an empty message", is.Field)
		}
	}
}
