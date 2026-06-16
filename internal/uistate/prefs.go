//go:build js && wasm

package uistate

import (
	"encoding/json"
	"strconv"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	prefsAtomID  = "app:prefs"
	prefsStoreID = "cashflux:prefs"
)

// UsePrefs returns the shared display-preferences atom, seeded from localStorage
// so week-start and date-format choices survive reloads (the dataset is re-seeded
// each boot, so preferences persist here, not in the store). Screens read it to
// format dates; the settings form writes it back via PersistPrefs.
func UsePrefs() state.Atom[prefs.Prefs] {
	return state.UseAtom(prefsAtomID, loadPrefs())
}

// PersistPrefs saves preferences to localStorage. Call it after writing the atom
// so the choice is remembered across reloads.
func PersistPrefs(p prefs.Prefs) {
	data, err := json.Marshal(p.Normalize())
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", prefsStoreID, string(data))
}

// LoadPrefs returns the persisted preferences directly (without the atom), for
// one-shot application at boot before any component mounts.
func LoadPrefs() prefs.Prefs { return loadPrefs() }

// ApplyPrefs reflects the appearance preferences onto the document root so CSS
// can react: a data-theme attribute (resolving "system" to the OS setting), a
// data-density attribute, and the --accent custom property. Call it on boot and
// whenever the preferences change.
func ApplyPrefs(p prefs.Prefs) {
	p = p.Normalize()
	root := js.Global().Get("document").Get("documentElement")
	if root.IsNull() || root.IsUndefined() {
		return
	}
	root.Call("setAttribute", "data-theme", resolveTheme(p.Theme))
	density := "comfortable"
	if p.Compact {
		density = "compact"
	}
	root.Call("setAttribute", "data-density", density)
	root.Get("style").Call("setProperty", "--accent", p.Accent)
	root.Get("style").Call("setProperty", "--ui-scale", strconv.FormatFloat(p.ScaleFraction(), 'f', -1, 64))
}

// resolveTheme turns the theme preference into a concrete "dark"/"light" value,
// consulting the OS color-scheme for "system".
func resolveTheme(t prefs.Theme) string {
	if t == prefs.ThemeSystem {
		m := js.Global().Call("matchMedia", "(prefers-color-scheme: light)")
		if !m.IsNull() && !m.IsUndefined() && m.Get("matches").Bool() {
			return "light"
		}
		return "dark"
	}
	return string(t)
}

// loadPrefs reads saved preferences from localStorage, falling back to defaults
// when absent or invalid. The result is always normalized.
func loadPrefs() prefs.Prefs {
	v := js.Global().Get("localStorage").Call("getItem", prefsStoreID)
	if v.IsNull() || v.IsUndefined() {
		return prefs.Default()
	}
	var p prefs.Prefs
	if err := json.Unmarshal([]byte(v.String()), &p); err != nil {
		return prefs.Default()
	}
	return p.Normalize()
}
