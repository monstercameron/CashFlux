package theme

import (
	"strings"
	"testing"
)

func TestFontFormat(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{
		"font/woff2":      {"woff2", true},
		"FONT/WOFF2":      {"woff2", true}, // case-insensitive
		"  font/woff  ":   {"woff", true},  // trims
		"font/ttf":        {"truetype", true},
		"font/otf":        {"opentype", true},
		"application/pdf": {"", false},
		"image/png":       {"", false},
		"":                {"", false},
	}
	for mime, want := range cases {
		got, ok := FontFormat(mime)
		if got != want.want || ok != want.ok {
			t.Errorf("FontFormat(%q) = (%q, %v), want (%q, %v)", mime, got, ok, want.want, want.ok)
		}
	}
}

func TestFontMIMEForName(t *testing.T) {
	cases := map[string]string{
		"MyFont.woff2":     "font/woff2",
		"MyFont.WOFF":      "font/woff",
		"path/to/Some.ttf": "font/ttf",
		"Display.otf":      "font/otf",
		"notafont.png":     "",
		"noextension":      "",
	}
	for name, want := range cases {
		if got := FontMIMEForName(name); got != want {
			t.Errorf("FontMIMEForName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestFontAssetEmpty(t *testing.T) {
	if !(FontAsset{}).Empty() {
		t.Error("zero FontAsset should be Empty")
	}
	if (FontAsset{DataURL: "data:font/woff2;base64,AAAA"}).Empty() {
		t.Error("asset with a data URL should not be Empty")
	}
	if !(FontAsset{DataURL: "   "}).Empty() {
		t.Error("whitespace-only data URL should be Empty")
	}
}

func TestFontFaceCSS(t *testing.T) {
	// Empty asset → no rule.
	if css := FontFaceCSS(FontAsset{}); css != "" {
		t.Errorf("empty asset should yield no CSS, got %q", css)
	}
	// Missing family → no rule (the family is what the theme references).
	if css := FontFaceCSS(FontAsset{DataURL: "data:font/woff2;base64,AAAA"}); css != "" {
		t.Errorf("asset without a family should yield no CSS, got %q", css)
	}
	// Known format → includes the format() hint.
	css := FontFaceCSS(FontAsset{Family: "MyFont", MIME: "font/woff2", DataURL: "data:font/woff2;base64,AAAA"})
	for _, want := range []string{"@font-face", `font-family: "MyFont"`, "url(data:font/woff2;base64,AAAA)", `format("woff2")`, "font-display: swap"} {
		if !strings.Contains(css, want) {
			t.Errorf("FontFaceCSS missing %q in:\n%s", want, css)
		}
	}
	// Unknown MIME → still emits a rule, just without the format() hint.
	css2 := FontFaceCSS(FontAsset{Family: "X", MIME: "application/octet-stream", DataURL: "data:x;base64,AAAA"})
	if !strings.Contains(css2, "@font-face") || strings.Contains(css2, "format(") {
		t.Errorf("unknown-MIME asset should emit a rule with no format() hint, got:\n%s", css2)
	}
}

func TestValidateFontUpload(t *testing.T) {
	if errs := ValidateFontUpload("font/woff2", 50_000); errs != nil {
		t.Errorf("a small woff2 should validate, got %v", errs)
	}
	if errs := ValidateFontUpload("image/png", 50_000); len(errs) == 0 {
		t.Error("a non-font MIME should be rejected")
	}
	if errs := ValidateFontUpload("font/woff2", 0); len(errs) == 0 {
		t.Error("an empty file should be rejected")
	}
	if errs := ValidateFontUpload("font/woff2", FontMaxBytes+1); len(errs) == 0 {
		t.Error("an over-cap file should be rejected")
	}
	if errs := ValidateFontUpload("font/woff2", FontMaxBytes); errs != nil {
		t.Errorf("a file exactly at the cap should be allowed, got %v", errs)
	}
}
