// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// pickerFixtureTarget portals the category picker's panel into the test fixture for the rest of
// the test. The renderer cannot see the page body, so without it the picker is open but unseen.
func pickerFixtureTarget(t *testing.T, f *render.Fixture) {
	t.Helper()
	previous := categoryPickerPortal
	categoryPickerPortal = ui.PortalTarget{Node: f.Target()}
	t.Cleanup(func() { categoryPickerPortal = previous })
}
