//go:build js && wasm

package uistate

import (
	"encoding/json"
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
	a := state.UseAtom(prefsAtomID, loadPrefs())
	capturedPrefs = a
	prefsCaptured = true
	return a
}

var (
	capturedPrefs state.Atom[prefs.Prefs]
	prefsCaptured bool
)

// CurrentPrefs returns the live preferences from the captured atom, for global
// callbacks (keyboard shortcuts / command palette) that can't call the UsePrefs
// hook. Falls back to the persisted prefs before the first render.
func CurrentPrefs() prefs.Prefs {
	if prefsCaptured {
		return capturedPrefs.Get()
	}
	return loadPrefs()
}

// SetPrefs writes preferences from outside a component render and persists +
// applies them. Routes through the captured atom so the change re-renders
// subscribers; a hook call from such a callback would panic.
func SetPrefs(p prefs.Prefs) {
	if prefsCaptured {
		capturedPrefs.Set(p)
	}
	PersistPrefs(p)
	ApplyPrefs(p)
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
// can react: a data-theme attribute (resolving "system" to the OS setting) and
// the --accent custom property. Density and display scale are owned by the theme
// engine (see ApplyTheme), which sets data-density and --ui-scale; ApplyPrefs no
// longer touches them, so the two systems can't fight. Call it on boot and
// whenever the preferences change.
func ApplyPrefs(p prefs.Prefs) {
	p = p.Normalize()
	root := js.Global().Get("document").Get("documentElement")
	if root.IsNull() || root.IsUndefined() {
		return
	}
	root.Call("setAttribute", "data-theme", resolveTheme(p.Theme))
	root.Get("style").Call("setProperty", "--accent", p.Accent)
	// WONDER: motion level drives data-wonder on <html>, which keys all
	// CSS WONDER tokens (--wonder-on, durations, etc.). "full" keeps the
	// default :root values, so we omit the attribute when full to let the
	// stylesheet's defaults win — identical visual result, cleaner DOM.
	switch p.Motion {
	case prefs.MotionOff, prefs.MotionSubtle:
		root.Call("setAttribute", "data-wonder", string(p.Motion))
	default:
		root.Call("removeAttribute", "data-wonder")
	}
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
