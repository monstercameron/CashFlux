//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/i18n"
	"github.com/monstercameron/GoWebComponents/state"
)

const langStoreID = "cashflux:languages"

// bundle is the app's shared language bundle: the English source catalog with any
// imported languages merged in from localStorage. English is the source/fallback.
var bundle = loadBundle()

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

const langAtomID = "app:lang"

// UseLang returns the active-language atom (default English). A future language
// selector writes it; T resolves against it once more languages exist.
func UseLang() state.Atom[i18n.Lang] {
	return state.UseAtom(langAtomID, i18n.English)
}

// T translates a dot-namespaced key for display, formatting with fmt.Sprintf when
// args are given. It does not call a hook (so it is safe inside loops and row
// components); it resolves against the bundle's default language. When language
// switching lands, the active language will be threaded in at render edges.
func T(key string, args ...any) string {
	return bundle.T(bundle.Default, key, args...)
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
