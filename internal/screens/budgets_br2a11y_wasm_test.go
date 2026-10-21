// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBudgetTrackAllNameStartsWithVisibleText(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(trackAllButton, trackAllButtonProps{Total: 2}))
	button := byTestID(f, "button", "budgets-track-all")
	if button == nil {
		t.Fatal("Review all untracked button did not render")
	}
	if name, visible := button.Attr("aria-label"), button.Text(); name != visible || visible != "Review untracked categories: 2" {
		t.Fatalf("accessible name = %q, visible text = %q; want both to include the count", name, visible)
	}
}
