// SPDX-License-Identifier: MIT

package theme

import "testing"

func TestMixHex(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		frac float64
		want string
	}{
		{"t=0 returns a", "#102030", "#ffffff", 0, "#102030"},
		{"t=1 returns b", "#102030", "#ffffff", 1, "#ffffff"},
		{"midpoint of black and white", "#000000", "#ffffff", 0.5, "#808080"},
		{"clamps below 0 to a", "#123456", "#ffffff", -2, "#123456"},
		{"clamps above 1 to b", "#123456", "#ffffff", 5, "#ffffff"},
		{"bad first color degrades to a", "nope", "#ffffff", 0.5, "nope"},
		{"bad second color degrades to a", "#123456", "nope", 0.5, "#123456"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mixHex(tc.a, tc.b, tc.frac); got != tc.want {
				t.Errorf("mixHex(%q,%q,%g) = %q, want %q", tc.a, tc.b, tc.frac, got, tc.want)
			}
		})
	}
}

func TestDerivedVarsEmitted(t *testing.T) {
	vars := Default().CSSVars()
	for _, k := range []string{"--bg-elev", "--text-faint", "--accent-dim", "--warn", "--danger"} {
		if vars[k] == "" {
			t.Errorf("CSSVars() is missing derived token %q", k)
		}
	}
	// --danger aliases Down exactly (mirrors the --down token).
	if vars["--danger"] != vars["--down"] {
		t.Errorf("--danger = %q, want it to alias --down = %q", vars["--danger"], vars["--down"])
	}
	// --warn is the fixed semantic amber.
	if vars["--warn"] != warnToken {
		t.Errorf("--warn = %q, want %q", vars["--warn"], warnToken)
	}
	// The elevated surface differs from the plain card (it's lifted toward text).
	if vars["--bg-elev"] == vars["--bg-card"] {
		t.Error("--bg-elev should differ from --bg-card")
	}
}
