// SPDX-License-Identifier: MIT

package widgetstyle

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/widgetcfg"
)

func TestInlineStyleEmitsOnlySetValidFields(t *testing.T) {
	got := InlineStyle(widgetcfg.Config{
		KeyBg:          "#112233",
		KeyText:        "#ffffff",
		KeyRadius:      "16",
		KeyFont:        "display",
		KeyWeight:      "600",
		KeyShadow:      "soft",
		KeyAccent:      "#ff0066",
		"someOtherKey": "ignored",
	})
	want := map[string]string{
		"background-color": "#112233",
		"color":            "#ffffff",
		"border-radius":    "16px",
		"font-family":      "var(--font-display), 'Fraunces', serif",
		"font-weight":      "600",
		// accent strip composed with the soft drop shadow
		"box-shadow": "inset 0 3px 0 0 #ff0066, 0 1px 3px rgba(0,0,0,.12)",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["border-color"]; ok {
		t.Error("unset border-color should not be emitted")
	}
	if _, ok := got["--accent"]; ok {
		t.Error("accent should render as a box-shadow strip, not a --accent custom property")
	}
}

func TestInlineStyleRejectsInvalid(t *testing.T) {
	got := InlineStyle(widgetcfg.Config{
		KeyBg:      "not-a-color",
		KeyText:    "#xyz",  // bad hex
		KeyBorderW: "99",    // out of range
		KeyRadius:  "-4",    // out of range
		KeyFont:    "comic", // unknown token
		KeyWeight:  "123",   // not allowed
		KeyShadow:  "neon",  // unknown token
	})
	if len(got) != 0 {
		t.Fatalf("invalid values should be dropped, got %v", got)
	}
}

func TestBorderWidthZeroIsExplicit(t *testing.T) {
	got := InlineStyle(widgetcfg.Config{KeyBorderW: "0"})
	if got["border-width"] != "0px" {
		t.Fatalf("border-width 0 should emit 0px (explicit no border), got %q", got["border-width"])
	}
}

func TestEffectivePerWidgetWins(t *testing.T) {
	global := widgetcfg.Config{KeyBg: "#000000", KeyRadius: "8", KeyText: "#cccccc"}
	per := widgetcfg.Config{KeyBg: "#ffffff", KeyAccent: "#ff0000"}
	eff := Effective(global, per)
	if eff[KeyBg] != "#ffffff" {
		t.Errorf("per-widget bg should win, got %q", eff[KeyBg])
	}
	if eff[KeyRadius] != "8" {
		t.Errorf("global radius should carry through, got %q", eff[KeyRadius])
	}
	if eff[KeyText] != "#cccccc" {
		t.Errorf("global text should carry through, got %q", eff[KeyText])
	}
	if eff[KeyAccent] != "#ff0000" {
		t.Errorf("per-widget accent should appear, got %q", eff[KeyAccent])
	}
}

func TestHexNormalizesAndValidates(t *testing.T) {
	cases := map[string]string{
		"#ABC":    "#abc",
		"#11AaFf": "#11aaff",
		"":        "",
		"abc":     "",
		"#12":     "",
		"#1234":   "",
		"#12345g": "",
	}
	for in, want := range cases {
		if got := hex(in); got != want {
			t.Errorf("hex(%q) = %q, want %q", in, got, want)
		}
	}
}
