//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/i18n"
)

const (
	langStoreID       = "cashflux:languages"
	activeLangStoreID = "cashflux:active-lang"
)

// bundle is the app's shared language bundle: the English source catalog with any
// imported languages merged in from localStorage. English is the source/fallback.
var bundle = loadBundle()

// activeLang is the language T resolves to, loaded from localStorage at boot.
var activeLang = loadActiveLang()

// loadBundle seeds the English source catalog and merges any persisted imported
// languages (from a prior Import) on top.
func loadBundle() *i18n.Bundle {
	b := i18n.DefaultBundle()
	v := js.Global().Get("localStorage").Call("getItem", langStoreID)
	if !v.IsNull() && !v.IsUndefined() {
		_ = b.ImportJSON([]byte(v.String()))
	}
	return b
}

// loadActiveLang reads the chosen language from localStorage, defaulting to
// English when absent.
func loadActiveLang() i18n.Lang {
	v := js.Global().Get("localStorage").Call("getItem", activeLangStoreID)
	if v.IsNull() || v.IsUndefined() || v.String() == "" {
		return i18n.English
	}
	return i18n.Lang(v.String())
}

// T translates a dot-namespaced key in the active language for display,
// formatting with fmt.Sprintf when args are given. It does not call a hook (so
// it is safe inside loops and row components); the bundle falls back to English
// (then the key) for anything untranslated.
func T(key string, args ...any) string {
	return bundle.T(activeLang, key, args...)
}

// Languages lists the languages available to pick (default first).
func Languages() []i18n.Lang { return bundle.Languages() }

// ActiveLanguage returns the currently selected language.
func ActiveLanguage() i18n.Lang { return activeLang }

// SetActiveLanguage persists the chosen language and reloads so every rendered
// string re-resolves in it (T is non-reactive by design, so a reload is the
// clean, reliable way to switch the whole UI at once).
func SetActiveLanguage(l i18n.Lang) {
	activeLang = l
	js.Global().Get("localStorage").Call("setItem", activeLangStoreID, string(l))
	js.Global().Get("location").Call("reload")
}

// ExportLanguages serializes the whole language bundle (every language) to JSON —
// the file translators edit and re-import.
func ExportLanguages() ([]byte, error) {
	return bundle.ExportJSON()
}

// ImportLanguages merges a JSON language bundle into the app and persists the
// merged set to localStorage so it survives reloads.
func ImportLanguages(data []byte) error {
	if err := bundle.ImportJSON(data); err != nil {
		return err
	}
	if out, err := bundle.ExportJSON(); err == nil {
		js.Global().Get("localStorage").Call("setItem", langStoreID, string(out))
	}
	return nil
}
