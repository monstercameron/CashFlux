package theme

import "testing"

func TestThemeIsLight(t *testing.T) {
	tests := []struct {
		name   string
		bgBase string
		want   bool
	}{
		{"pure white is light", "#ffffff", true},
		{"pure black is dark", "#000000", false},
		{"paper-ish near-white is light", "#f4f4f5", true},
		{"default dark surface is dark", "#0e1116", false},
		{"mid dark card is dark", "#161b22", false},
		{"shorthand white is light", "#fff", true},
		{"unparseable fails safe to dark", "not-a-color", false},
		{"empty fails safe to dark", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Theme{BgBase: tc.bgBase}).IsLight(); got != tc.want {
				t.Errorf("IsLight(%q) = %v, want %v", tc.bgBase, got, tc.want)
			}
		})
	}
}

// The built-in dark Default must read as dark; this guards against a token edit
// accidentally flipping the shell skin.
func TestDefaultIsDark(t *testing.T) {
	if Default().IsLight() {
		t.Error("Default() theme should not be light")
	}
}
