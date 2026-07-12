// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
)

// CheckedIf sets a checkbox's checked state — the checkbox analogue of SelectedIf for
// <option>s. It uses the framework's Checked() boolean prop, which sets the DOM
// `checked` PROPERTY (not the `checked` content attribute): an attribute only seeds
// defaultChecked and does NOT update the live property when a keyed input re-renders,
// so a selected row's checkbox never appeared ticked. Checked(false) also clears it,
// so unchecking works too.
func CheckedIf(on bool) any {
	return Checked(on)
}
