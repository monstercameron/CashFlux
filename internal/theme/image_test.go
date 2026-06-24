// SPDX-License-Identifier: MIT

package theme

import (
	"strings"
	"testing"
)

func TestValidImageMIME(t *testing.T) {
	for _, ok := range []string{"image/png", "IMAGE/JPEG", " image/webp ", "image/gif"} {
		if !ValidImageMIME(ok) {
			t.Errorf("ValidImageMIME(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"image/svg+xml", "application/pdf", "font/woff2", ""} {
		if ValidImageMIME(bad) {
			t.Errorf("ValidImageMIME(%q) = true, want false", bad)
		}
	}
}

func TestImageMIMEForName(t *testing.T) {
	cases := map[string]string{
		"banner.png":     "image/png",
		"Photo.JPG":      "image/jpeg",
		"a/b/pic.jpeg":   "image/jpeg",
		"hero.webp":      "image/webp",
		"loop.gif":       "image/gif",
		"notimage.woff2": "",
		"noext":          "",
	}
	for name, want := range cases {
		if got := ImageMIMEForName(name); got != want {
			t.Errorf("ImageMIMEForName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestValidateImageUpload(t *testing.T) {
	if errs := ValidateImageUpload("image/png", 100_000); errs != nil {
		t.Errorf("a small png should validate, got %v", errs)
	}
	if errs := ValidateImageUpload("image/svg+xml", 100); len(errs) == 0 {
		t.Error("svg should be rejected")
	}
	if errs := ValidateImageUpload("image/png", 0); len(errs) == 0 {
		t.Error("empty file should be rejected")
	}
	if errs := ValidateImageUpload("image/png", ImageMaxBytes+1); len(errs) == 0 {
		t.Error("over-cap file should be rejected")
	}
	if errs := ValidateImageUpload("image/png", ImageMaxBytes); errs != nil {
		t.Errorf("file exactly at the cap should be allowed, got %v", errs)
	}
}

func TestBannerNoneAndCSS(t *testing.T) {
	if !(Banner{}).None() {
		t.Error("zero Banner should be None")
	}
	if !(Banner{Kind: BannerNone}).None() {
		t.Error("explicit none should be None")
	}
	if !(Banner{Kind: BannerGradient, Value: "  "}).None() {
		t.Error("gradient with blank value should be None")
	}
	if (Banner{}).CSS() != "" {
		t.Error("empty banner CSS should be empty")
	}

	grad := Banner{Kind: BannerGradient, Value: "linear-gradient(135deg, #000, #111)"}
	if grad.None() {
		t.Error("a gradient banner should not be None")
	}
	if grad.CSS() != grad.Value {
		t.Errorf("gradient CSS = %q, want the raw gradient", grad.CSS())
	}

	img := ImageBanner("data:image/png;base64,AAAA", "Hero")
	if img.None() {
		t.Error("an image banner should not be None")
	}
	if css := img.CSS(); !strings.HasPrefix(css, "url(") || !strings.Contains(css, "data:image/png") {
		t.Errorf("image CSS = %q, want a url(dataURL)", css)
	}
}

func TestBannerPresets(t *testing.T) {
	ps := BannerPresets()
	if len(ps) == 0 {
		t.Fatal("expected built-in banner presets")
	}
	for _, p := range ps {
		if p.Kind != BannerGradient {
			t.Errorf("preset %q kind = %q, want gradient", p.Name, p.Kind)
		}
		if p.None() || p.CSS() == "" {
			t.Errorf("preset %q should render a non-empty banner", p.Name)
		}
		if p.Name == "" {
			t.Error("preset should have a name")
		}
	}
	// Returned slice must be a copy (mutating it must not affect later calls).
	ps[0].Value = "tampered"
	if BannerPresets()[0].Value == "tampered" {
		t.Error("BannerPresets must return a defensive copy")
	}
}
