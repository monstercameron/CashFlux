//go:build js && wasm

package uistate

import (
	"encoding/json"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/theme"
)

const (
	fontsStoreID = "cashflux:fonts"
	fontStyleID  = "cashflux-fonts"
)

// LoadFonts returns the user's uploaded custom fonts from localStorage (kept in
// their own slot, separate from the theme, so theme writes stay small). An empty
// or unreadable slot yields no fonts.
func LoadFonts() []theme.FontAsset {
	v := js.Global().Get("localStorage").Call("getItem", fontsStoreID)
	if v.IsNull() || v.IsUndefined() {
		return nil
	}
	var fonts []theme.FontAsset
	if err := json.Unmarshal([]byte(v.String()), &fonts); err != nil {
		return nil
	}
	return fonts
}

// PersistFonts saves the custom fonts to localStorage.
func PersistFonts(fonts []theme.FontAsset) {
	data, err := json.Marshal(fonts)
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", fontsStoreID, string(data))
}

// AddFont stores a custom font (replacing any existing one with the same family),
// applies the updated @font-face rules, and returns the new font list.
func AddFont(f theme.FontAsset) []theme.FontAsset {
	var next []theme.FontAsset
	replaced := false
	for _, e := range LoadFonts() {
		if e.Family == f.Family {
			next = append(next, f)
			replaced = true
		} else {
			next = append(next, e)
		}
	}
	if !replaced {
		next = append(next, f)
	}
	PersistFonts(next)
	ApplyFonts(next)
	return next
}

// RemoveFont drops the custom font with the given family, re-applies the rules,
// and returns the new list.
func RemoveFont(family string) []theme.FontAsset {
	var next []theme.FontAsset
	for _, e := range LoadFonts() {
		if e.Family != family {
			next = append(next, e)
		}
	}
	PersistFonts(next)
	ApplyFonts(next)
	return next
}

// ApplyFonts injects an @font-face rule for every stored custom font into a
// single managed <style> element in the document head, so the browser can use
// the uploaded families. Replacing its contents is idempotent. Call it at boot
// and whenever the font list changes.
func ApplyFonts(fonts []theme.FontAsset) {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	style := doc.Call("getElementById", fontStyleID)
	if style.IsNull() || style.IsUndefined() {
		style = doc.Call("createElement", "style")
		style.Call("setAttribute", "id", fontStyleID)
		head := doc.Get("head")
		if head.IsNull() || head.IsUndefined() {
			return
		}
		head.Call("appendChild", style)
	}
	var b strings.Builder
	for _, f := range fonts {
		if css := theme.FontFaceCSS(f); css != "" {
			b.WriteString(css)
			b.WriteByte('\n')
		}
	}
	style.Set("textContent", b.String())
}
