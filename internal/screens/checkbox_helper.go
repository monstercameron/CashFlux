//go:build js && wasm

package screens

import (
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
)

// CheckedIf renders the `checked` attribute on a checkbox when on is true, and a
// harmless no-op attribute otherwise — the checkbox analogue of SelectedIf for
// <option>s. (The framework has SelectedIf but no CheckedIf; this fills the gap so
// controlled checkboxes can be written inline.)
func CheckedIf(on bool) any {
	if on {
		return Attr("checked", "")
	}
	return Attr("aria-checked", "false")
}
