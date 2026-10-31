// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestFlipPanelCloseOnlyFooterOffersOnlyClose(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	defer unmountFlipPanel(f)
	f.Render(uic.CreateElement(flipPanel, FlipPanelProps{
		Title:     "Import",
		Back:      Button("Choose a source"),
		FormID:    "import-modal-form",
		CloseOnly: true,
	}))

	if f.ByRole("button", "Close") == nil {
		t.Fatal("Close-only panel should render a Close button")
	}
	if f.ByRole("button", "Cancel") != nil || f.ByRole("button", "Save") != nil {
		t.Fatal("Close-only panel should not imply a pending Save or Cancel action")
	}
}
