// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestFlipPanelTV8FTitleUsesUnbalancedLeftTitleSlot(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("tv8f-flip-body"), uic.CreateElement(flipPanel, FlipPanelProps{
		Title:    "Review transactions",
		Back:     Button(Type("button"), "Done"),
		NoFooter: true,
	})))

	if f.ByRole("dialog", "Review transactions") == nil {
		t.Fatal("flip panel should retain its visible dialog title")
	}
	found := false
	for _, node := range f.AllByTag("span") {
		if strings.Contains(node.Attr("class"), "flip-title-balance") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("flip title balance slot is missing for the TV8F left-aligned title style")
	}
}
