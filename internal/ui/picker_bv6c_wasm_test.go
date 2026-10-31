// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	sh "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

type pickerDialogTestProps struct{ Child uic.Node }

func pickerDialogTest(props pickerDialogTestProps) uic.Node {
	return uic.AccessibleOverlay(uic.AccessibleOverlayProps{
		Open: true, Target: uic.PortalTarget{Selector: "#picker-test-portal"},
		SurfaceID: "budget-add-dialog", Kind: uic.OverlayKindDialog, Role: "dialog",
		Modal: true, TrapFocus: true, CloseOnEscape: true, CloseOnOutsideClick: true,
		Child: props.Child,
	})
}

func TestPickerInsideDialogOwnsTopLayerAndKeepsDialogOpenOnChoose(t *testing.T) {
	var committed string
	f := render.New(t)
	picker := uic.CreateElement(pickerComponent, pickerComponentProps{
		Props: PickerProps{
			ID: "budget-category", Label: "Category", Placeholder: "Choose a category", Searchable: true,
			Options:  []PickerOption{{Value: "food", Label: "Food"}, {Value: "rent", Label: "Rent"}},
			OnCommit: func(value string) { committed = value },
		},
		PortalTarget: "#picker-test-portal",
	})
	f.Render(sh.Fragment(sh.Div(sh.Attr("id", "picker-test-portal")),
		uic.CreateElement(pickerDialogTest, pickerDialogTestProps{Child: picker})))

	f.ByID("budget-category").Click()
	search := f.ByID("budget-category-search")
	if !search.Exists() {
		t.Fatal("opening the category picker did not expose its search field")
	}
	if got := f.BuildOverlayOutsideSurfaceID(); got != "budget-category-surface" {
		t.Fatalf("top outside-click owner = %q, want picker surface", got)
	}
	surfaces := f.BuildOverlaySurfaces()
	var pickerSurface, dialogFound bool
	for _, surface := range surfaces {
		if surface.SurfaceID == "budget-category-surface" {
			pickerSurface = surface.Kind == "popover" && surface.Depth > 0 && surface.HandlesOutsideClick && !surface.IsModal
		}
		if surface.SurfaceID == "budget-add-dialog" && surface.IsModal {
			dialogFound = true
		}
	}
	if !pickerSurface || !dialogFound {
		t.Fatalf("picker/dialog overlay stack is incorrect: %+v", surfaces)
	}

	f.ByID("budget-category-option-0").Click()
	if committed != "food" {
		t.Fatalf("chosen category = %q, want food", committed)
	}
	if f.ByID("budget-category-search").Exists() {
		t.Fatal("picker remained open after choosing a category")
	}
	if f.ByID("budget-add-dialog").Exists() == false {
		t.Fatal("choosing a category dismissed the parent dialog")
	}
}
