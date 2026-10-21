// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"syscall/js"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestFlipPanelEnterLeavesNestedFormSubmitToEditor(t *testing.T) {
	object := js.Global().Get("Object")
	active, nestedForm := object.New(), object.New()
	nestedForm.Set("id", "draft-row-form")
	active.Set("form", nestedForm)

	if flipPanelHandlesEnterInForm(active, "import-modal-form") {
		t.Fatal("FlipPanel intercepted Enter from a nested row form")
	}
	if flipPanelHandlesEnterInForm(active, "") {
		t.Fatal("FlipPanel intercepted Enter when no footer form is configured")
	}
}

func TestFlipPanelEnterStillTargetsConfiguredFooterForm(t *testing.T) {
	object := js.Global().Get("Object")
	active, form := object.New(), object.New()
	form.Set("id", "import-modal-form")
	active.Set("form", form)

	if !flipPanelHandlesEnterInForm(active, "import-modal-form") {
		t.Fatal("FlipPanel should handle Enter from its configured footer form")
	}
}

func TestFlipPanelInitialFocusSkipsHeaderCloseForPickerChoice(t *testing.T) {
	object := js.Global().Get("Object")
	close, firstTile := object.New(), object.New()
	close.Set("tagName", "BUTTON")
	close.Set("className", "set-close")
	firstTile.Set("tagName", "BUTTON")
	firstTile.Set("className", "import-tile")
	close.Set("hasAttribute", testJSFunc(t, func(js.Value, []js.Value) any { return false }))
	firstTile.Set("hasAttribute", testJSFunc(t, func(js.Value, []js.Value) any { return false }))

	if got := flipPanelInitialFocusTarget([]js.Value{close, firstTile}); !got.Equal(firstTile) {
		t.Fatal("picker should open on its first choice instead of the Close button")
	}
}

func TestFlipPanelCloseOnlyFooterOffersOnlyClose(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
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
