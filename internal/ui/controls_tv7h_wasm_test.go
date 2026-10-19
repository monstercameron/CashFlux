// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTV7HSegmentPressedStateMatchesARIA(t *testing.T) {
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	f.Render(uic.CreateElement(segmented, SegmentedProps{
		Options:  []SegOption{{Value: "ledger", Label: "Ledger"}, {Value: "calendar", Label: "Calendar"}},
		Selected: "calendar",
	}))
	f.Stabilize()

	for _, button := range f.AllByTag("button") {
		pressed := button.Attr("aria-checked")
		if got := button.Attr("data-pressed"); got != pressed {
			t.Errorf("%q pressed marker = %q, aria-checked = %q", button.Text(), got, pressed)
		}
	}
}
