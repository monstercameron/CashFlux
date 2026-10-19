// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBudgetTrackAllNameStartsWithVisibleText(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(trackAllButton, trackAllButtonProps{Total: 2}))
	button := byTestID(f, "button", "budgets-track-all")
	if button == nil {
		t.Fatal("Review all untracked button did not render")
	}
	if name, visible := button.Attr("aria-label"), button.Text(); !strings.HasPrefix(name, visible) {
		t.Fatalf("accessible name = %q, visible text = %q", name, button.Text())
	}
}
