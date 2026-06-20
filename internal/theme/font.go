package theme

import (
	"fmt"
	"strings"
)

// FontMaxBytes caps an uploaded custom font's raw file size. Fonts are embedded
// as data URLs in the durable store, so the cap keeps that store within budget
// (base64 inflates the bytes by ~33%); WOFF2 — the recommended web format — is
// comfortably under this for typical text faces.
const FontMaxBytes = 1 << 20 // 1 MiB

// fontFormats maps an uploaded font's MIME type to the CSS `src: format(...)`
// hint, and doubles as the set of accepted web-font formats.
var fontFormats = map[string]string{
	"font/woff2":                  "woff2",
	"font/woff":                   "woff",
	"application/font-woff":       "woff",
	"font/ttf":                    "truetype",
	"font/sfnt":                   "truetype",
	"application/x-font-ttf":      "truetype",
	"font/otf":                    "opentype",
	"application/x-font-opentype": "opentype",
	"application/vnd.ms-opentype": "opentype",
}

// fontExtMIME maps a font file extension to a canonical MIME type, so the wasm
// layer can recover a type when the browser reports none (common for .ttf/.otf).
var fontExtMIME = map[string]string{
	".woff2": "font/woff2",
	".woff":  "font/woff",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
}

// FontFormat returns the CSS format() hint for a font MIME type and whether the
// type is a supported web-font format.
func FontFormat(mime string) (string, bool) {
	f, ok := fontFormats[strings.ToLower(strings.TrimSpace(mime))]
	return f, ok
}

// FontMIMEForName infers a font MIME type from a file name's extension, or "" if
// the extension isn't a known font type. Use it as a fallback when the browser's
// reported MIME type is empty.
func FontMIMEForName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	for ext, mime := range fontExtMIME {
		if strings.HasSuffix(lower, ext) {
			return mime
		}
	}
	return ""
}

// FontAsset is a user-uploaded custom font: a CSS family name the user can pick
// for the interface or heading font, the font's MIME type, and the font bytes as
// a data URL. It is applied by injecting an @font-face rule (see FontFaceCSS).
type FontAsset struct {
	Family  string `json:"family"`
	MIME    string `json:"mime"`
	DataURL string `json:"dataUrl"`
}

// Empty reports whether the asset carries no font data.
func (f FontAsset) Empty() bool { return strings.TrimSpace(f.DataURL) == "" }

// FontFaceCSS renders an @font-face rule that registers the asset's family from
// its embedded data URL, so the browser can use it like any installed font. It
// returns "" for an empty asset or one with no family. `font-display: swap`
// avoids invisible text while the (already-inline) font initializes.
func FontFaceCSS(f FontAsset) string {
	if f.Empty() || strings.TrimSpace(f.Family) == "" {
		return ""
	}
	src := fmt.Sprintf("url(%s)", f.DataURL)
	if format, ok := FontFormat(f.MIME); ok {
		src = fmt.Sprintf("url(%s) format(%q)", f.DataURL, format)
	}
	return fmt.Sprintf("@font-face { font-family: %q; src: %s; font-display: swap; }", f.Family, src)
}

// ValidateFontUpload reports human-readable problems with a font upload — an
// unsupported format, an empty file, or one over the size cap — or nil if the
// upload is acceptable.
func ValidateFontUpload(mime string, size int) []string {
	var errs []string
	if _, ok := FontFormat(mime); !ok {
		errs = append(errs, "That file isn't a supported font format. Use WOFF2, WOFF, TTF, or OTF.")
	}
	if size <= 0 {
		errs = append(errs, "The font file is empty.")
	}
	if size > FontMaxBytes {
		errs = append(errs, fmt.Sprintf("The font file is too large (max %d KB).", FontMaxBytes/1024))
	}
	return errs
}
