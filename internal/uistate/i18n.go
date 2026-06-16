//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/CashFlux/internal/i18n"
	"github.com/monstercameron/GoWebComponents/state"
)

// bundle is the app's shared language bundle, seeded with the English source
// catalog. Imported languages will merge in here later (Settings → import); for
// now English is the only language.
var bundle = i18n.DefaultBundle()

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
