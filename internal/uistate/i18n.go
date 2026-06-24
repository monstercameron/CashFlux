// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"sync"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/i18n"
)

const (
	langStoreID       = "cashflux:languages"
	activeLangStoreID = "cashflux:active-lang"
)

// Language state is LAZILY initialized (not at package-init) so it reads from the
// SQLite-backed browser store AFTER boot has opened it — IndexedDB can't be read
// synchronously at package-init. ensureI18n runs once, on the first T()/Languages
// call (which happens during the first render, well after browserstore.Init).
var (
	i18nOnce    sync.Once
	bundle      *i18n.Bundle
	activeLang  i18n.Lang
)

func ensureI18n() {
	i18nOnce.Do(func() {
		bundle = loadBundle()
		activeLang = loadActiveLang()
	})
}

// loadBundle seeds the English source catalog and merges any persisted imported
// languages on top.
func loadBundle() *i18n.Bundle {
	b := i18n.DefaultBundle()
	if raw := browserstore.GetString(langStoreID); raw != "" {
		_ = b.ImportJSON([]byte(raw))
	}
	return b
}

// loadActiveLang reads the chosen language, defaulting to English when absent.
func loadActiveLang() i18n.Lang {
	if raw := browserstore.GetString(activeLangStoreID); raw != "" {
		return i18n.Lang(raw)
	}
	return i18n.English
}

// T translates a dot-namespaced key in the active language for display,
// formatting with fmt.Sprintf when args are given. It does not call a hook (so
// it is safe inside loops and row components); the bundle falls back to English
// (then the key) for anything untranslated.
func T(key string, args ...any) string {
	ensureI18n()
	return bundle.T(activeLang, key, args...)
}

// Languages lists the languages available to pick (default first).
func Languages() []i18n.Lang { ensureI18n(); return bundle.Languages() }

// ActiveLanguage returns the currently selected language.
func ActiveLanguage() i18n.Lang { ensureI18n(); return activeLang }

// SetActiveLanguage persists the chosen language and reloads so every rendered
// string re-resolves in it (T is non-reactive by design, so a reload is the
// clean, reliable way to switch the whole UI at once).
func SetActiveLanguage(l i18n.Lang) {
	ensureI18n()
	activeLang = l
	browserstore.Set(activeLangStoreID, string(l))
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
		browserstore.Set(langStoreID, string(out))
	}
	return nil
}
