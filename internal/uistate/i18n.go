// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"sync"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/i18n"
)

const (
	langStoreID       = "cashflux:languages"
	activeLangStoreID = "cashflux:active-lang"
)

// Language state is LAZILY initialized (not at package-init) so it reads from the
// SQLite dataset AFTER boot has loaded it. ensureI18n runs once, on the first
// T()/Languages call (which happens during the first render, well after the dataset
// is hydrated). The chosen language + any imported bundles live in the dataset's
// PRESERVED settings KV (the single source of truth, so they travel with an exported
// or synced dataset); a legacy browser-store copy is read non-destructively as a
// first-paint fallback and folded in by MigrateLegacyLanguage once the dataset loads.
var (
	i18nOnce   sync.Once
	bundle     *i18n.Bundle
	activeLang i18n.Lang
)

func ensureI18n() {
	i18nOnce.Do(func() {
		bundle = loadBundle()
		activeLang = loadActiveLang()
	})
}

// settingPeek reads a settings value non-destructively: the dataset settings KV
// (source of truth) first, then a legacy browser-store value as a READ-ONLY
// first-paint fallback (used before the dataset is loaded, or while an encrypted
// dataset is still locked). Unlike SettingKVGet it never migrates/removes the legacy
// copy — the destructive fold happens in MigrateLegacyLanguage once the dataset is
// confirmed loaded, so a pre-unlock read can't lose the value to a later ImportJSON.
func settingPeek(key string) string {
	if app := appstate.Default; app != nil {
		if v, ok := app.GetSettingKV(key); ok && v != "" {
			return v
		}
	}
	return browserstore.GetString(key)
}

// loadBundle seeds the English source catalog and merges any persisted imported
// languages on top.
func loadBundle() *i18n.Bundle {
	b := i18n.DefaultBundle()
	if raw := settingPeek(langStoreID); raw != "" {
		_ = b.ImportJSON([]byte(raw))
	}
	return b
}

// loadActiveLang reads the chosen language, defaulting to English when absent.
func loadActiveLang() i18n.Lang {
	if raw := settingPeek(activeLangStoreID); raw != "" {
		return i18n.Lang(raw)
	}
	return i18n.English
}

// MigrateLegacyLanguage folds a legacy browser-store language selection + imported
// bundle into the dataset settings KV (so they travel with an exported/synced
// dataset) and drops the legacy copies. It MUST be called only once the dataset is
// actually loaded — never while an encrypted dataset is still locked, or a later
// ImportJSON would clobber the just-migrated value. No-op once migrated.
func MigrateLegacyLanguage() {
	app := appstate.Default
	if app == nil {
		return
	}
	for _, key := range []string{activeLangStoreID, langStoreID} {
		if _, ok := app.GetSettingKV(key); ok {
			continue // already in the dataset
		}
		if raw := browserstore.GetString(key); raw != "" {
			SettingKVSet(key, raw)
			browserstore.Remove(key)
		}
	}
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
	SettingKVSet(activeLangStoreID, string(l)) // dataset settings KV (single source of truth)
	js.Global().Get("location").Call("reload")
}

// ExportLanguages serializes the whole language bundle (every language) to JSON —
// the file translators edit and re-import.
func ExportLanguages() ([]byte, error) {
	ensureI18n()
	return bundle.ExportJSON()
}

// ImportLanguages merges a JSON language bundle into the app and persists the
// merged set to localStorage so it survives reloads.
func ImportLanguages(data []byte) error {
	ensureI18n()
	if err := bundle.ImportJSON(data); err != nil {
		return err
	}
	if out, err := bundle.ExportJSON(); err == nil {
		SettingKVSet(langStoreID, string(out)) // dataset settings KV (single source of truth)
	}
	return nil
}
